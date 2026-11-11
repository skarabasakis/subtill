package database_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate(t *testing.T) {
	t.Run("creates current WAL database with mode 0600 when path is absent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")

		err := database.Create(path, testPreferences)

		require.NoError(t, err)
		assert.EqualValues(t, database.SubtillAppID, rawQueryInt(t, path, "PRAGMA application_id"))
		version, dirty := rawSchemaState(t, path)
		assert.Equal(t, database.SchemaVersion, version)
		assert.False(t, dirty)

		db := rawDB(t, path)
		var currency, mode, journalMode string
		require.NoError(t, db.QueryRow(`SELECT currency, currency_conversion_mode FROM preferences WHERE id = 1`).Scan(&currency, &mode))
		require.NoError(t, db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode))
		require.NoError(t, db.Close())
		assert.Equal(t, "EUR", currency)
		assert.Equal(t, "live", mode)
		assert.Equal(t, "wal", journalMode)

		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
		assertNoSidecars(t, path)
		assertNoOpenConnections(t, path)

		_, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())
	})

	t.Run("fills omitted preferences from locale when currency is unset", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MONETARY", "")
		t.Setenv("LANG", "ja_JP.UTF-8")
		path := filepath.Join(t.TempDir(), "subtill.db")

		err := database.Create(path, domain.Preferences{
			CurrencyConversionMode: ptr(domain.CurrencyConversionModeLive),
		})

		require.NoError(t, err)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()
		info, err := database.Info(db)
		require.NoError(t, err)
		assert.Equal(t, domain.PreferencesRecord{
			Currency:               "JPY",
			CurrencyConversionMode: domain.CurrencyConversionModeLive,
		}, info.Preferences)
	})

	t.Run("creates file at literal path when name has URI characters", func(t *testing.T) {
		name := "my subs #1 100%.db"
		if runtime.GOOS != "windows" {
			name = "what?" + name
		}
		path := filepath.Join(t.TempDir(), name)

		err := database.Create(path, testPreferences)

		require.NoError(t, err)
		_, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())
		entries, err := os.ReadDir(filepath.Dir(path))
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, name, entries[0].Name())
	})

	t.Run("returns ErrAlreadyExists and leaves file unchanged when path exists", func(t *testing.T) {
		path := newDB(t)
		before := digest(t, path)

		err := database.Create(path, testPreferences)

		assert.ErrorIs(t, err, database.ErrAlreadyExists)
		assert.Equal(t, before, digest(t, path))
		assertNoSidecars(t, path)
	})

	t.Run("returns an error and creates nothing when parent directory is missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")

		err := database.Create(filepath.Join(dir, "subtill.db"), testPreferences)

		require.Error(t, err)
		assert.NotErrorIs(t, err, database.ErrAlreadyExists)
		_, statErr := os.Stat(dir)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
	})

	t.Run("lets exactly one caller succeed when creates run concurrently", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")
		const n = 8
		errs := make([]error, n)
		start := make(chan struct{})
		var wg sync.WaitGroup

		for i := range n {
			wg.Go(func() {
				<-start
				errs[i] = database.Create(path, testPreferences)
			})
		}
		close(start)
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
			} else {
				assert.ErrorIs(t, err, database.ErrAlreadyExists)
			}
		}
		assert.Equal(t, 1, succeeded)

		_, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())
	})

	t.Run("returns ErrMigrationFailed and removes files when a statement fails", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")

		err := newService(t, migrationBase, migrationFailing).Create(path, testPreferences)

		require.ErrorIs(t, err, database.ErrMigrationFailed)
		_, statErr := os.Stat(path)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
		assertNoSidecars(t, path)
		require.NoError(t, newService(t, migrationBase).Create(path, testPreferences))
	})

	t.Run("returns ErrMigrationFailed and removes files when foreign keys fail", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subtill.db")

		err := newService(t, migrationBase, migrationFKViolates).Create(path, testPreferences)

		require.ErrorIs(t, err, database.ErrMigrationFailed)
		_, statErr := os.Stat(path)
		assert.ErrorIs(t, statErr, os.ErrNotExist)
		assertNoSidecars(t, path)
		require.NoError(t, newService(t, migrationBase).Create(path, testPreferences))
	})
}
