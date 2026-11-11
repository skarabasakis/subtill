package database_test

import (
	"regexp"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenOutdated(t *testing.T) {
	t.Run("returns ErrSchemaOutdated without backup when migrate is false", func(t *testing.T) {
		path := seededV1(t)
		before := digest(t, path)

		db, closeDB, err := newService(t, migrationBase, migrationAddColumn).Open(path, false)

		require.ErrorIs(t, err, database.ErrSchemaOutdated)
		se := schemaError(t, err)
		assert.Equal(t, uint(1), se.Found)
		assert.Equal(t, uint(2), se.Expected)
		assert.Empty(t, se.Backup)
		assertRefused(t, path, before, db, closeDB)
		assert.Empty(t, backups(t, path))
	})

	t.Run("migrates schema, keeps rows, and writes backup when migrate is true", func(t *testing.T) {
		path := seededV1(t)

		db, closeDB, err := newService(t, migrationBase, migrationAddColumn).Open(path, true)

		require.NoError(t, err)
		var version uint
		require.NoError(t, db.Get(&version, `SELECT version FROM schema_migrations`))
		assert.Equal(t, uint(2), version)
		var parents int
		require.NoError(t, db.Get(&parents, `SELECT COUNT(*) FROM parents WHERE note IS NULL`))
		assert.Equal(t, 2, parents)
		require.NoError(t, closeDB())
		assertNoSidecars(t, path)

		found := backups(t, path)
		require.Len(t, found, 1)
		backup := found[0]
		pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(path) + `\.\d{8}T\d{6}\.v1\.bak$`)
		assert.Regexp(t, pattern, backup)
		backupVersion, dirty := rawSchemaState(t, backup)
		assert.Equal(t, uint(1), backupVersion)
		assert.False(t, dirty)
		assert.EqualValues(t, 2, rawQueryInt(t, backup, `SELECT COUNT(*) FROM children`))
	})

	failures := []struct {
		name string
		bad  string
	}{
		{"restores backup and returns ErrMigrationFailed when a later migration fails", migrationFailing},
		{"restores backup and returns ErrMigrationFailed when foreign keys fail", migrationFKViolates},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			path := seededV1(t)
			cfg := newService(t, migrationBase, migrationAddColumn, tc.bad)

			db, closeDB, err := cfg.Open(path, true)

			require.ErrorIs(t, err, database.ErrMigrationFailed)
			se := schemaError(t, err)
			assert.Equal(t, uint(1), se.Found)
			assert.Equal(t, uint(3), se.Expected)
			require.NotEmpty(t, se.Backup)
			assert.Nil(t, db)
			assert.Nil(t, closeDB)
			assert.Equal(t, digest(t, se.Backup), digest(t, path), "file was not restored from the backup")
			assertNoSidecars(t, path)
			assertNoOpenConnections(t, path)
			version, dirty := rawSchemaState(t, path)
			assert.Equal(t, uint(1), version)
			assert.False(t, dirty)

			_, _, err = cfg.Open(path, false)

			assert.ErrorIs(t, err, database.ErrSchemaOutdated)
		})
	}

	t.Run("keeps child rows when migration rebuilds parent table", func(t *testing.T) {
		path := seededV1(t)

		db, closeDB, err := newService(t, migrationBase, migrationRebuild).Open(path, true)

		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()
		var children int
		require.NoError(t, db.Get(&children, `SELECT COUNT(*) FROM children`))
		assert.Equal(t, 2, children)
	})

	t.Run("returns ErrDatabaseInUse without backup when another connection holds the file", func(t *testing.T) {
		path := seededV1(t)
		other := rawDB(t, path)
		before := digest(t, path)

		db, closeDB, err := newService(t, migrationBase, migrationAddColumn).Open(path, true)

		require.ErrorIs(t, err, database.ErrDatabaseInUse)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		assert.Equal(t, before, digest(t, path))
		assert.Empty(t, backups(t, path))
		require.NoError(t, other.Close())
		assertNoSidecars(t, path)
		assertNoOpenConnections(t, path)
	})
}
