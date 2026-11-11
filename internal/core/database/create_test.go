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
	t.Run("creates a current subtill database", func(t *testing.T) {
		path := newDB(t)

		assert.EqualValues(t, database.SubtillAppID, database.RawQueryInt(t, path, "PRAGMA application_id"))
		version, dirty := database.RawSchemaState(t, path)
		assert.Equal(t, database.SchemaVersion, version)
		assert.False(t, dirty)

		db := database.RawDB(t, path)
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
		database.AssertNoSidecars(t, path)
		database.AssertNoOpenConnections(t, path)

		_, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())
	})

	t.Run("takes omitted preferences from the locale", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MONETARY", "")
		t.Setenv("LANG", "ja_JP.UTF-8")
		path := filepath.Join(t.TempDir(), "subtill.db")
		require.NoError(t, database.Create(path, domain.Preferences{
			CurrencyConversionMode: ptr(domain.CurrencyConversionModeLive),
		}))

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

	t.Run("path with URI special characters", func(t *testing.T) {
		name := "my subs #1 100%.db"
		if runtime.GOOS != "windows" {
			name = "what?" + name
		}
		path := filepath.Join(t.TempDir(), name)
		require.NoError(t, database.Create(path, testPreferences))

		_, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())
		entries, err := os.ReadDir(filepath.Dir(path))
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, name, entries[0].Name())
	})

	t.Run("existing file", func(t *testing.T) {
		path := newDB(t)
		before := database.Digest(t, path)

		err := database.Create(path, testPreferences)
		assert.ErrorIs(t, err, database.ErrAlreadyExists)
		assert.Equal(t, before, database.Digest(t, path))
		database.AssertNoSidecars(t, path)
	})

	t.Run("missing parent directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		err := database.Create(filepath.Join(dir, "subtill.db"), testPreferences)
		require.Error(t, err)
		assert.NotErrorIs(t, err, database.ErrAlreadyExists)
		_, statErr := os.Stat(dir)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
	})

	t.Run("concurrent calls", func(t *testing.T) {
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
}

func TestCreateMigrationFailure(t *testing.T) {
	for name, bad := range map[string]string{
		"failing migration":     migrationFailing,
		"foreign key violation": migrationFKViolates,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "subtill.db")

			err := newService(t, migrationBase, bad).Create(path, testPreferences)
			require.ErrorIs(t, err, database.ErrMigrationFailed)
			_, statErr := os.Stat(path)
			assert.ErrorIs(t, statErr, os.ErrNotExist)
			database.AssertNoSidecars(t, path)

			require.NoError(t, newService(t, migrationBase).Create(path, testPreferences))
		})
	}
}
