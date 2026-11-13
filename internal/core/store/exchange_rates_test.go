package store_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangeRates(t *testing.T) {
	exchangeRates := store.ExchangeRates{}

	t.Run("gets rate when currency and date match exactly", func(t *testing.T) {
		db := newDB(t)
		record := domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}
		require.NoError(t, exchangeRates.Upsert(db, record))

		got, err := exchangeRates.Get(db, "USD", date("2026-02-01"))

		require.NoError(t, err)
		assert.Equal(t, &record, got)
	})

	t.Run("returns ErrNotFound when date differs", func(t *testing.T) {
		db := newDB(t)
		require.NoError(t, exchangeRates.Upsert(db, domain.ExchangeRateRecord{
			Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01"),
		}))

		_, err := exchangeRates.Get(db, "USD", date("2026-02-02"))

		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("replaces rate when currency and date already cached", func(t *testing.T) {
		db := newDB(t)
		require.NoError(t, exchangeRates.Upsert(db, domain.ExchangeRateRecord{
			Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01"),
		}))

		err := exchangeRates.Upsert(db, domain.ExchangeRateRecord{
			Currency: "USD", Rate: "0.95", ExchangeOn: date("2026-02-01"),
		})

		require.NoError(t, err)
		got, err := exchangeRates.Get(db, "USD", date("2026-02-01"))
		require.NoError(t, err)
		assert.Equal(t, "0.95", got.Rate)
	})

	t.Run("removes every rate when deleting all", func(t *testing.T) {
		db := newDB(t)
		require.NoError(t, exchangeRates.Upsert(db, domain.ExchangeRateRecord{Currency: "USD", Rate: "0.92", ExchangeOn: date("2026-02-01")}))
		require.NoError(t, exchangeRates.Upsert(db, domain.ExchangeRateRecord{Currency: "GBP", Rate: "1.15", ExchangeOn: date("2026-02-01")}))

		affected, err := exchangeRates.DeleteAll(db)

		require.NoError(t, err)
		assert.EqualValues(t, 2, affected)
		_, err = exchangeRates.Get(db, "USD", date("2026-02-01"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})
}
