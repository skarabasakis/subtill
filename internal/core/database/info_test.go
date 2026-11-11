package database_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/database"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfo(t *testing.T) {
	path := newDB(t)
	db, closeDB, err := database.Open(path, false)
	require.NoError(t, err)

	info, err := database.Info(db)
	require.NoError(t, err)
	assert.Equal(t, &database.DBInfo{
		SchemaVersion:     database.SchemaVersion,
		SubscriptionCount: 0,
		Preferences: domain.PreferencesRecord{
			Currency:               "EUR",
			CurrencyConversionMode: domain.CurrencyConversionModeLive,
		},
	}, info)

	_, err = db.Exec(`INSERT INTO subscriptions (id, name) VALUES ('01', 'Netflix'), ('02', 'Spotify')`)
	require.NoError(t, err)
	info, err = database.Info(db)
	require.NoError(t, err)
	assert.Equal(t, 2, info.SubscriptionCount)

	require.NoError(t, closeDB())
	_, err = database.Info(db)
	assert.Error(t, err)
}
