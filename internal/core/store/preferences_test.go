package store_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreferences(t *testing.T) {
	preferences := store.Preferences{}

	t.Run("returns stored preferences when database is created", func(t *testing.T) {
		db := newDB(t)

		got, err := preferences.Get(db)

		require.NoError(t, err)
		assert.Equal(t, &domain.PreferencesRecord{
			Currency: "EUR", CurrencyConversionMode: domain.CurrencyConversionModeApproximate,
		}, got)
	})

	t.Run("replaces values when preferences row exists", func(t *testing.T) {
		db := newDB(t)
		updated := domain.PreferencesRecord{Currency: "GBP", CurrencyConversionMode: domain.CurrencyConversionModeLive}

		err := preferences.Upsert(db, updated)

		require.NoError(t, err)
		got, err := preferences.Get(db)
		require.NoError(t, err)
		assert.Equal(t, &updated, got)
	})

	t.Run("returns ErrNotFound when preferences row is missing", func(t *testing.T) {
		db := newDB(t)
		_, err := db.Exec(`DELETE FROM preferences`)
		require.NoError(t, err)

		_, err = preferences.Get(db)

		assert.ErrorIs(t, err, store.ErrNotFound)
	})
}
