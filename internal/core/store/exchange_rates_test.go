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

func saveRates(t *testing.T, q sqlx.Ext, records ...domain.ExchangeRateRecord) {
	t.Helper()
	affected, err := store.ExchangeRates{}.Upsert(q, records)
	require.NoError(t, err)
	require.EqualValues(t, len(records), affected)
}

func rate(base, quote, value, exchangeOn string) domain.ExchangeRateRecord {
	return domain.ExchangeRateRecord{BaseCurrency: base, QuoteCurrency: quote, Rate: value, ExchangeOn: date(exchangeOn)}
}

func TestExchangeRates(t *testing.T) {
	exchangeRates := store.ExchangeRates{}

	t.Run("Get", func(t *testing.T) {
		t.Run("gets rate when currencies and date match exactly", func(t *testing.T) {
			db := newDB(t)
			record := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, record)

			got, err := exchangeRates.Get(db, "USD", "EUR", date("2026-02-01"))

			require.NoError(t, err)
			assert.Equal(t, &record, got)
		})

		t.Run("returns ErrNotFound when date differs", func(t *testing.T) {
			db := newDB(t)
			saveRates(t, db, rate("USD", "EUR", "0.92", "2026-02-01"))

			_, err := exchangeRates.Get(db, "USD", "EUR", date("2026-02-02"))

			assert.ErrorIs(t, err, store.ErrNotFound)
		})

		t.Run("returns ErrNotFound when quote currency differs", func(t *testing.T) {
			db := newDB(t)
			saveRates(t, db, rate("USD", "EUR", "0.92", "2026-02-01"))

			_, err := exchangeRates.Get(db, "USD", "GBP", date("2026-02-01"))

			assert.ErrorIs(t, err, store.ErrNotFound)
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("lists saved rates when every requested date is saved", func(t *testing.T) {
			db := newDB(t)
			feb1 := rate("USD", "EUR", "0.92", "2026-02-01")
			feb2 := rate("USD", "EUR", "0.93", "2026-02-02")
			saveRates(t, db, feb2, feb1)

			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-02"), date("2026-02-01")})

			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{feb1, feb2}, got)
		})

		t.Run("omits dates without saved rate when listing", func(t *testing.T) {
			db := newDB(t)
			feb1 := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, feb1)

			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01"), date("2026-02-02")})

			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{feb1}, got)
		})

		t.Run("lists only rates of requested base currency", func(t *testing.T) {
			db := newDB(t)
			usd := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, usd, rate("GBP", "EUR", "1.15", "2026-02-01"))

			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01")})

			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{usd}, got)
		})

		t.Run("lists only rates of requested quote currency", func(t *testing.T) {
			db := newDB(t)
			eur := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, eur, rate("USD", "GBP", "0.79", "2026-02-01"))

			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01")})

			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{eur}, got)
		})

		t.Run("lists nothing when no dates requested", func(t *testing.T) {
			db := newDB(t)
			saveRates(t, db, rate("USD", "EUR", "0.92", "2026-02-01"))

			got, err := exchangeRates.List(db, "USD", "EUR", nil)

			require.NoError(t, err)
			assert.Empty(t, got)
		})

		t.Run("lists each rate once when dates repeat", func(t *testing.T) {
			db := newDB(t)
			feb1 := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, feb1)

			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01"), date("2026-02-01")})

			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{feb1}, got)
		})
	})

	t.Run("Upsert", func(t *testing.T) {
		t.Run("saves every rate when rates given", func(t *testing.T) {
			db := newDB(t)
			usd := rate("USD", "EUR", "0.92", "2026-02-01")
			gbp := rate("GBP", "EUR", "1.15", "2026-02-01")

			affected, err := exchangeRates.Upsert(db, []domain.ExchangeRateRecord{usd, gbp})

			require.NoError(t, err)
			assert.EqualValues(t, 2, affected)
			gotUSD, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01")})
			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{usd}, gotUSD)
			gotGBP, err := exchangeRates.List(db, "GBP", "EUR", []civil.Date{date("2026-02-01")})
			require.NoError(t, err)
			assert.Equal(t, []domain.ExchangeRateRecord{gbp}, gotGBP)
		})

		t.Run("replaces rate when currency pair and date already saved", func(t *testing.T) {
			db := newDB(t)
			saveRates(t, db, rate("USD", "EUR", "0.92", "2026-02-01"))

			affected, err := exchangeRates.Upsert(db, []domain.ExchangeRateRecord{rate("USD", "EUR", "0.95", "2026-02-01")})

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			got, err := exchangeRates.Get(db, "USD", "EUR", date("2026-02-01"))
			require.NoError(t, err)
			assert.Equal(t, "0.95", got.Rate)
		})

		t.Run("keeps rates of other quote currencies when upserting same base and date", func(t *testing.T) {
			db := newDB(t)
			eur := rate("USD", "EUR", "0.92", "2026-02-01")
			saveRates(t, db, eur)

			affected, err := exchangeRates.Upsert(db, []domain.ExchangeRateRecord{rate("USD", "GBP", "0.79", "2026-02-01")})

			require.NoError(t, err)
			assert.EqualValues(t, 1, affected)
			got, err := exchangeRates.Get(db, "USD", "EUR", date("2026-02-01"))
			require.NoError(t, err)
			assert.Equal(t, &eur, got)
		})

		t.Run("saves nothing when no rates given", func(t *testing.T) {
			db := newDB(t)

			affected, err := exchangeRates.Upsert(db, nil)

			require.NoError(t, err)
			assert.EqualValues(t, 0, affected)
		})

		t.Run("fails and saves nothing when any rate has identical currencies", func(t *testing.T) {
			db := newDB(t)
			valid := rate("USD", "EUR", "0.92", "2026-02-01")
			identical := rate("EUR", "EUR", "1", "2026-02-01")

			_, err := exchangeRates.Upsert(db, []domain.ExchangeRateRecord{valid, identical})

			require.Error(t, err)
			got, err := exchangeRates.List(db, "USD", "EUR", []civil.Date{date("2026-02-01")})
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	})
}
