package store_test

import (
	"testing"

	"cloud.google.com/go/civil"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// saveRates saves records quoted in EUR, the preferred currency of newDB.
func saveRates(t *testing.T, q sqlx.Ext, records ...domain.ExchangeRateRecord) {
	t.Helper()
	affected, err := store.ExchangeRates{}.Upsert(q, "EUR", records)
	require.NoError(t, err)
	require.EqualValues(t, len(records), affected)
}

func TestExchangeRates(t *testing.T) {
	exchangeRates := store.ExchangeRates{}

	t.Run("gets rate when currency and date match exactly", func(t *testing.T) {
		db := newDB(t)
		record := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		saveRates(t, db, record)

		got, err := exchangeRates.Get(db, "USD", date("2026-02-01"))

		require.NoError(t, err)
		assert.Equal(t, &record, got)
	})

	t.Run("returns ErrNotFound when date differs", func(t *testing.T) {
		db := newDB(t)
		saveRates(t, db, domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")})

		_, err := exchangeRates.Get(db, "USD", date("2026-02-02"))

		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("saves every rate when quote matches preferred currency", func(t *testing.T) {
		db := newDB(t)
		usd := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		gbp := domain.ExchangeRateRecord{Currency: "GBP", Rate: "1.15", ExchangeOn: date("2026-02-01")}

		affected, err := exchangeRates.Upsert(db, "EUR", []domain.ExchangeRateRecord{usd, gbp})

		require.NoError(t, err)
		assert.EqualValues(t, 2, affected)
		gotUSD, err := exchangeRates.List(db, "USD", []civil.Date{date("2026-02-01")})
		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{usd}, gotUSD)
		gotGBP, err := exchangeRates.List(db, "GBP", []civil.Date{date("2026-02-01")})
		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{gbp}, gotGBP)
	})

	t.Run("saves nothing when quote differs from preferred currency", func(t *testing.T) {
		db := newDB(t)
		rates := []domain.ExchangeRateRecord{
			{Currency: "GBP", Rate: "1.15", ExchangeOn: date("2026-02-01")},
			{Currency: "GBP", Rate: "1.16", ExchangeOn: date("2026-02-02")},
		}

		affected, err := exchangeRates.Upsert(db, "USD", rates)

		require.NoError(t, err)
		assert.EqualValues(t, 0, affected)
		got, err := exchangeRates.List(db, "GBP", []civil.Date{date("2026-02-01"), date("2026-02-02")})
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("replaces rate when currency and date already saved", func(t *testing.T) {
		db := newDB(t)
		saveRates(t, db, domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")})

		affected, err := exchangeRates.Upsert(db, "EUR", []domain.ExchangeRateRecord{
			{Currency: "USD", Rate: "0.95", ExchangeOn: date("2026-02-01")},
		})

		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
		got, err := exchangeRates.Get(db, "USD", date("2026-02-01"))
		require.NoError(t, err)
		assert.Equal(t, "0.95", got.Rate)
	})

	t.Run("saves nothing when no rates given", func(t *testing.T) {
		db := newDB(t)

		affected, err := exchangeRates.Upsert(db, "EUR", nil)

		require.NoError(t, err)
		assert.EqualValues(t, 0, affected)
	})

	t.Run("saves rates when preferred currency changed earlier in same transaction", func(t *testing.T) {
		db := newDB(t)
		tx, err := db.Beginx()
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback() })
		require.NoError(t, store.Preferences{}.Upsert(tx, domain.PreferencesRecord{
			Currency: "USD", CurrencyConversionMode: domain.CurrencyConversionModeLive,
		}))

		affected, err := exchangeRates.Upsert(tx, "USD", []domain.ExchangeRateRecord{
			{Currency: "GBP", Rate: "1.25", ExchangeOn: date("2026-02-01")},
		})

		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
	})

	t.Run("lists saved rates when every requested date is saved", func(t *testing.T) {
		db := newDB(t)
		feb1 := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		feb2 := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.93", ExchangeOn: date("2026-02-02")}
		saveRates(t, db, feb2, feb1)

		got, err := exchangeRates.List(db, "USD", []civil.Date{date("2026-02-02"), date("2026-02-01")})

		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{feb1, feb2}, got)
	})

	t.Run("omits dates without saved rate when listing", func(t *testing.T) {
		db := newDB(t)
		feb1 := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		saveRates(t, db, feb1)

		got, err := exchangeRates.List(db, "USD", []civil.Date{date("2026-02-01"), date("2026-02-02")})

		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{feb1}, got)
	})

	t.Run("lists only rates of requested currency", func(t *testing.T) {
		db := newDB(t)
		usd := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		saveRates(t, db, usd, domain.ExchangeRateRecord{Currency: "GBP", Rate: "1.15", ExchangeOn: date("2026-02-01")})

		got, err := exchangeRates.List(db, "USD", []civil.Date{date("2026-02-01")})

		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{usd}, got)
	})

	t.Run("lists nothing when no dates requested", func(t *testing.T) {
		db := newDB(t)
		saveRates(t, db, domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")})

		got, err := exchangeRates.List(db, "USD", nil)

		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("lists each rate once when dates repeat", func(t *testing.T) {
		db := newDB(t)
		feb1 := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		saveRates(t, db, feb1)

		got, err := exchangeRates.List(db, "USD", []civil.Date{date("2026-02-01"), date("2026-02-01")})

		require.NoError(t, err)
		assert.Equal(t, []domain.ExchangeRateRecord{feb1}, got)
	})

	t.Run("removes every rate when deleting all", func(t *testing.T) {
		db := newDB(t)
		saveRates(t, db,
			domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")},
			domain.ExchangeRateRecord{Currency: "GBP", Rate: "1.15", ExchangeOn: date("2026-02-01")},
		)

		affected, err := exchangeRates.DeleteAll(db)

		require.NoError(t, err)
		assert.EqualValues(t, 2, affected)
		_, err = exchangeRates.Get(db, "USD", date("2026-02-01"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})
}
