package database_test

import (
	"regexp"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenOutdated(t *testing.T) {
	t.Run("migrate false", func(t *testing.T) {
		path := seededV1(t)
		before := database.Digest(t, path)

		db, closeDB, err := newService(t, migrationBase, migrationAddColumn).Open(path, false)
		require.ErrorIs(t, err, database.ErrSchemaOutdated)
		se := database.SchemaErrorOf(t, err)
		assert.Equal(t, uint(1), se.Found)
		assert.Equal(t, uint(2), se.Expected)
		assert.Empty(t, se.Backup)
		database.AssertRefused(t, path, before, db, closeDB)
		backup, err := database.NewestBackup(path)
		require.NoError(t, err)
		assert.Empty(t, backup)
	})

	t.Run("migrate true", func(t *testing.T) {
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
		database.AssertNoSidecars(t, path)

		backup, err := database.NewestBackup(path)
		require.NoError(t, err)
		pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(path) + `\.\d{8}T\d{6}\.v1\.bak$`)
		assert.Regexp(t, pattern, backup)
		backupVersion, dirty := database.RawSchemaState(t, backup)
		assert.Equal(t, uint(1), backupVersion)
		assert.False(t, dirty)
		assert.EqualValues(t, 2, database.RawQueryInt(t, backup, `SELECT COUNT(*) FROM children`))
	})

	for name, bad := range map[string]string{
		"second pending migration fails": migrationFailing,
		"foreign key violation":          migrationFKViolates,
	} {
		t.Run(name, func(t *testing.T) {
			path := seededV1(t)
			cfg := newService(t, migrationBase, migrationAddColumn, bad)

			db, closeDB, err := cfg.Open(path, true)
			require.ErrorIs(t, err, database.ErrMigrationFailed)
			se := database.SchemaErrorOf(t, err)
			assert.Equal(t, uint(1), se.Found)
			assert.Equal(t, uint(3), se.Expected)
			require.NotEmpty(t, se.Backup)
			assert.Nil(t, db)
			assert.Nil(t, closeDB)

			assert.Equal(t, database.Digest(t, se.Backup), database.Digest(t, path), "file was not restored from the backup")
			database.AssertNoSidecars(t, path)
			database.AssertNoOpenConnections(t, path)
			version, dirty := database.RawSchemaState(t, path)
			assert.Equal(t, uint(1), version)
			assert.False(t, dirty)

			_, _, err = cfg.Open(path, false)
			assert.ErrorIs(t, err, database.ErrSchemaOutdated)
		})
	}

	t.Run("table rebuild keeps child rows", func(t *testing.T) {
		path := seededV1(t)

		db, closeDB, err := newService(t, migrationBase, migrationRebuild).Open(path, true)
		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()
		var children int
		require.NoError(t, db.Get(&children, `SELECT COUNT(*) FROM children`))
		assert.Equal(t, 2, children)
	})

	t.Run("database in use", func(t *testing.T) {
		path := seededV1(t)
		other := database.RawDB(t, path)
		before := database.Digest(t, path)

		db, closeDB, err := newService(t, migrationBase, migrationAddColumn).Open(path, true)
		require.ErrorIs(t, err, database.ErrDatabaseInUse)
		assert.Nil(t, db)
		assert.Nil(t, closeDB)
		assert.Equal(t, before, database.Digest(t, path))
		backup, err := database.NewestBackup(path)
		require.NoError(t, err)
		assert.Empty(t, backup)

		require.NoError(t, other.Close())
		database.AssertNoSidecars(t, path)
		database.AssertNoOpenConnections(t, path)
	})
}
