package database_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfo(t *testing.T) {
	t.Run("returns version, preferences, and zero subscriptions when database is new", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()

		info, err := database.Info(db)

		require.NoError(t, err)
		assert.Equal(t, &database.DBInfo{
			SchemaVersion:     database.SchemaVersion,
			SubscriptionCount: 0,
			Preferences:       domain.PreferencesRecord{Currency: "EUR"},
		}, info)
	})

	t.Run("returns subscription count when subscriptions exist", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		defer func() { require.NoError(t, closeDB()) }()
		_, err = db.Exec(`INSERT INTO subscriptions (id, name) VALUES ('01', 'Netflix'), ('02', 'Spotify')`)
		require.NoError(t, err)

		info, err := database.Info(db)

		require.NoError(t, err)
		assert.Equal(t, 2, info.SubscriptionCount)
	})

	t.Run("returns an error when connection is closed", func(t *testing.T) {
		path := newDB(t)
		db, closeDB, err := database.Open(path, false)
		require.NoError(t, err)
		require.NoError(t, closeDB())

		_, err = database.Info(db)

		assert.Error(t, err)
	})
}
