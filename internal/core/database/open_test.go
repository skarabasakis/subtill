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

func TestOpen(t *testing.T) {
	t.Run("returns ErrNotFound and creates nothing when file is missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")

		db, closeDB, err := database.Open(path, true)

		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		_, statErr := os.Stat(path)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
		assertNoSidecars(t, path)
	})

	t.Run("returns ErrNotFound and creates nothing when parent directory is missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")

		db, closeDB, err := database.Open(filepath.Join(dir, "subtill.db"), true)

		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		_, statErr := os.Stat(dir)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
	})

	t.Run("returns ErrNotFound and creates nothing when path is a directory", func(t *testing.T) {
		dir := t.TempDir()

		db, closeDB, err := database.Open(dir, true)

		assert.ErrorIs(t, err, database.ErrNotFound)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		entries, readErr := os.ReadDir(dir)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
		assertNoSidecars(t, dir)
	})

	unopenable := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  error
	}{
		{"returns ErrNotSubtillDatabase unchanged when file is text", func(t *testing.T, path string) {
			require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("not a database\n", 100)), 0o600))
		}, database.ErrNotSubtillDatabase},
		{"returns ErrNotSubtillDatabase unchanged when file is random", func(t *testing.T, path string) {
			data := make([]byte, 4096)
			_, _ = rand.Read(data)
			require.NoError(t, os.WriteFile(path, data, 0o600))
		}, database.ErrNotSubtillDatabase},
		{"returns ErrNotSubtillDatabase unchanged when file is foreign sqlite", func(t *testing.T, path string) {
			rawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"PRAGMA application_id = 42",
				"CREATE TABLE notes (id INTEGER PRIMARY KEY, body TEXT)",
				"INSERT INTO notes (body) VALUES ('hello')")
		}, database.ErrNotSubtillDatabase},
		{"returns ErrNotSubtillDatabase unchanged when foreign sqlite has schema_migrations", func(t *testing.T, path string) {
			rawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"CREATE TABLE schema_migrations (version uint64, dirty bool)",
				"INSERT INTO schema_migrations VALUES ("+strconv.FormatUint(uint64(database.SchemaVersion), 10)+", 0)")
		}, database.ErrNotSubtillDatabase},
		{"returns ErrNoSchemaMigrationsTable unchanged when application_id set without schema_migrations", func(t *testing.T, path string) {
			rawExec(t, path,
				"PRAGMA journal_mode = WAL",
				"PRAGMA application_id = 1398096468",
				"CREATE TABLE notes (id INTEGER PRIMARY KEY)")
		}, database.ErrNoSchemaMigrationsTable},
	}
	for _, tc := range unopenable {
		for _, migrate := range []bool{false, true} {
			t.Run(withMigrate(tc.name, migrate), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "subtill.db")
				tc.setup(t, path)
				before := digest(t, path)

				db, closeDB, err := database.Open(path, migrate)

				assert.ErrorIs(t, err, tc.want)
				assertRefused(t, path, before, db, closeDB)
			})
		}
	}

	t.Run("returns ErrNotSubtillDatabase and adds no schema_migrations when file is foreign sqlite", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "foreign.db")
		rawExec(t, path, "PRAGMA application_id = 42", "CREATE TABLE notes (id INTEGER PRIMARY KEY)")

		_, _, err := database.Open(path, true)

		require.ErrorIs(t, err, database.ErrNotSubtillDatabase)
		assert.Zero(t, rawQueryInt(t, path,
			`SELECT COUNT(*) FROM sqlite_master WHERE name = 'schema_migrations'`))
	})

	for _, migrate := range []bool{false, true} {
		t.Run(withMigrate("returns ErrSchemaTooNew unchanged when schema is newer", migrate), func(t *testing.T) {
			path := newDB(t)
			rawExec(t, path, "UPDATE schema_migrations SET version = version + 1")
			before := digest(t, path)

			db, closeDB, err := database.Open(path, migrate)

			require.ErrorIs(t, err, database.ErrSchemaTooNew)
			se := schemaError(t, err)
			assert.Equal(t, database.SchemaVersion+1, se.Found)
			assert.Equal(t, database.SchemaVersion, se.Expected)
			assert.Empty(t, se.Backup)
			assertRefused(t, path, before, db, closeDB)
		})
	}

	t.Run("returns ErrSchemaDirty with empty backup when dirty, no backups", func(t *testing.T) {
		path := newDB(t)
		rawExec(t, path, "UPDATE schema_migrations SET dirty = 1")
		before := digest(t, path)

		db, closeDB, err := database.Open(path, true)

		require.ErrorIs(t, err, database.ErrSchemaDirty)
		se := schemaError(t, err)
		assert.Equal(t, database.SchemaVersion, se.Found)
		assert.Equal(t, database.SchemaVersion, se.Expected)
		assert.Empty(t, se.Backup)
		assertRefused(t, path, before, db, closeDB)
	})

	t.Run("returns ErrSchemaDirty with newest backup when dirty, backups exist", func(t *testing.T) {
		path := newDB(t)
		rawExec(t, path, "UPDATE schema_migrations SET dirty = 1")
		older := backupName(path, 0, time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local))
		newer := backupName(path, 0, time.Date(2026, 6, 2, 3, 4, 5, 0, time.Local))
		otherDB := filepath.Join(filepath.Dir(path), "other.db.20270101T000000.v0.bak")
		for _, p := range []string{newer, older, otherDB} {
			require.NoError(t, os.WriteFile(p, nil, 0o600))
		}
		before := digest(t, path)

		db, closeDB, err := database.Open(path, false)

		require.ErrorIs(t, err, database.ErrSchemaDirty)
		assert.Equal(t, newer, schemaError(t, err).Backup)
		assertRefused(t, path, before, db, closeDB)
	})

	t.Run("sets foreign keys, busy timeout, and WAL on each connection when opened", func(t *testing.T) {
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
	t.Run("checkpoints WAL into main file when another connection stays open", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		other := rawDB(t, path)
		_, err = db.Exec(`INSERT INTO subscriptions (id, name) VALUES ('01', 'Netflix')`)
		require.NoError(t, err)

		require.NoError(t, closeDB())

		data, err := os.ReadFile(filepath.Clean(path))
		require.NoError(t, err)
		copied := filepath.Join(t.TempDir(), "copy.db")
		require.NoError(t, os.WriteFile(copied, data, 0o600)) // #nosec G703 -- test temp directory
		assert.EqualValues(t, 1, rawQueryInt(t, copied, "SELECT COUNT(*) FROM subscriptions"))
		require.NoError(t, other.Close())
	})

	t.Run("returns nil when called a second time", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())

		err = closeDB()

		assert.NoError(t, err)
		assert.Error(t, db.Ping())
		assertNoSidecars(t, path)
		assertNoOpenConnections(t, path)
	})
}

func withMigrate(name string, migrate bool) string {
	if migrate {
		return name + ", migrate true"
	}
	return name + ", migrate false"
}
