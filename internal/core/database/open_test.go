package database_test

import (
	"context"
	"crypto/rand"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenRefusals(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")
		db, closeDB, err := database.Open(path, true)
		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		_, statErr := os.Stat(path)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
		database.AssertNoSidecars(t, path)
	})

	t.Run("missing parent directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		db, closeDB, err := database.Open(filepath.Join(dir, "subtill.db"), true)
		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		_, statErr := os.Stat(dir)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		db, closeDB, err := database.Open(dir, true)
		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		entries, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
		database.AssertNoSidecars(t, dir)
	})

	unopenable := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  error
	}{
		{"text file", func(t *testing.T, path string) {
			require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("not a database\n", 100)), 0o600))
		}, database.ErrNotSubtillDatabase},
		{"random bytes", func(t *testing.T, path string) {
			data := make([]byte, 4096)
			_, _ = rand.Read(data)
			require.NoError(t, os.WriteFile(path, data, 0o600))
		}, database.ErrNotSubtillDatabase},
		{"foreign SQLite database", func(t *testing.T, path string) {
			database.RawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"PRAGMA application_id = 42",
				"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)",
				"INSERT INTO notes (body) VALUES ('hello')")
		}, database.ErrNotSubtillDatabase},
		{"foreign SQLite database with a current schema_migrations", func(t *testing.T, path string) {
			database.RawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"CREATE TABLE schema_migrations (version uint64, dirty bool)",
				"INSERT INTO schema_migrations VALUES ("+strconv.FormatUint(uint64(database.SchemaVersion), 10)+", 0)")
		}, database.ErrNotSubtillDatabase},
		{"subtill application id without schema_migrations", func(t *testing.T, path string) {
			database.RawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"PRAGMA application_id = 1398096468",
				"CREATE TABLE notes (id INTEGER PRIMARY KEY)")
		}, database.ErrNoSchemaMigrationsTable},
	}
	for _, tc := range unopenable {
		for _, migrate := range []bool{false, true} {
			t.Run(tc.name+" migrate="+strconv.FormatBool(migrate), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "subtill.db")
				tc.setup(t, path)
				before := database.Digest(t, path)

				db, closeDB, err := database.Open(path, migrate)
				assert.ErrorIs(t, err, tc.want)
				database.AssertRefused(t, path, before, db, closeDB)
			})
		}
	}

	t.Run("foreign SQLite database is not given schema_migrations", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "foreign.db")
		database.RawExec(t, path, "PRAGMA application_id = 42", "CREATE TABLE notes (id INTEGER PRIMARY KEY)")

		_, _, err := database.Open(path, true)
		require.ErrorIs(t, err, database.ErrNotSubtillDatabase)
		assert.Zero(t, database.RawQueryInt(t, path,
			`SELECT COUNT(*) FROM sqlite_master WHERE name = 'schema_migrations'`))
	})

	for _, migrate := range []bool{false, true} {
		t.Run("newer schema migrate="+strconv.FormatBool(migrate), func(t *testing.T) {
			path := newDB(t)
			database.RawExec(t, path, "UPDATE schema_migrations SET version = version + 1")
			before := database.Digest(t, path)

			db, closeDB, err := database.Open(path, migrate)
			require.ErrorIs(t, err, database.ErrSchemaTooNew)
			se := database.SchemaErrorOf(t, err)
			assert.Equal(t, database.SchemaVersion+1, se.Found)
			assert.Equal(t, database.SchemaVersion, se.Expected)
			assert.Empty(t, se.Backup)
			database.AssertRefused(t, path, before, db, closeDB)
		})
	}

	t.Run("dirty schema without backup", func(t *testing.T) {
		path := newDB(t)
		database.RawExec(t, path, "UPDATE schema_migrations SET dirty = 1")
		before := database.Digest(t, path)

		db, closeDB, err := database.Open(path, true)
		require.ErrorIs(t, err, database.ErrSchemaDirty)
		se := database.SchemaErrorOf(t, err)
		assert.Equal(t, database.SchemaVersion, se.Found)
		assert.Equal(t, database.SchemaVersion, se.Expected)
		assert.Empty(t, se.Backup)
		database.AssertRefused(t, path, before, db, closeDB)
	})

	t.Run("dirty schema with backups", func(t *testing.T) {
		path := newDB(t)
		database.RawExec(t, path, "UPDATE schema_migrations SET dirty = 1")
		older := database.BackupPath(path, 0, time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local))
		newer := database.BackupPath(path, 0, time.Date(2026, 6, 2, 3, 4, 5, 0, time.Local))
		otherDB := filepath.Join(filepath.Dir(path), "other.db.20270101T000000.v0.bak")
		for _, p := range []string{newer, older, otherDB} {
			require.NoError(t, os.WriteFile(p, nil, 0o600))
		}
		before := database.Digest(t, path)

		db, closeDB, err := database.Open(path, false)
		require.ErrorIs(t, err, database.ErrSchemaDirty)
		assert.Equal(t, newer, database.SchemaErrorOf(t, err).Backup)
		database.AssertRefused(t, path, before, db, closeDB)
	})
}

func TestOpen(t *testing.T) {
	t.Run("sets pragmas on every connection of the pool", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()

		ctx := context.Background()
		for i := range 3 {
			conn, err := db.Conn(ctx)
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()

			var foreignKeys, busyTimeout int
			var journalMode string
			require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys))
			require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout))
			require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode))
			assert.Equal(t, 1, foreignKeys, "connection %d", i)
			assert.Equal(t, 5000, busyTimeout, "connection %d", i)
			assert.Equal(t, "wal", journalMode, "connection %d", i)
		}
	})
}

func TestCloseFn(t *testing.T) {
	t.Run("checkpoints commits into the database file", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)

		other := database.RawDB(t, path)
		_, err = db.Exec(`INSERT INTO subscriptions (id, name) VALUES ('01', 'Netflix')`)
		require.NoError(t, err)
		require.NoError(t, closeDB())

		data, err := os.ReadFile(filepath.Clean(path))
		require.NoError(t, err)
		copied := filepath.Join(t.TempDir(), "copy.db")
		require.NoError(t, os.WriteFile(copied, data, 0o600)) // #nosec G703 -- test temp directory
		assert.EqualValues(t, 1, database.RawQueryInt(t, copied, "SELECT COUNT(*) FROM subscriptions"))
		require.NoError(t, other.Close())
	})

	t.Run("second call", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)

		require.NoError(t, closeDB())
		assert.NoError(t, closeDB())
		assert.Error(t, db.Ping())
		database.AssertNoSidecars(t, path)
		database.AssertNoOpenConnections(t, path)
	})
}
