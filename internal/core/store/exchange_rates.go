package store

import (
	"cloud.google.com/go/civil"
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type ExchangeRates struct{}

var exchangeRateColumns = []string{
	"currency",
	"rate",
	"exchange_on",
}

// Get returns the cached rate of currency on exactly exchangeOn.
func (ExchangeRates) Get(q sqlx.Ext, currency string, exchangeOn civil.Date) (*domain.ExchangeRateRecord, error) {
	var row exchangeRateRow
	err := getOne(q, &row, "SELECT "+columnList(exchangeRateColumns)+" "+hereduck.D(`
		 FROM exchange_rates
		 WHERE currency = :currency AND exchange_on = :exchange_on
	`), map[string]any{"currency": currency, "exchange_on": exchangeOn.String()})
	if err != nil {
		return nil, err
	}
	record, err := row.record()
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// Upsert caches a rate, replacing the rate of the same currency and date.
func (ExchangeRates) Upsert(q sqlx.Ext, rate domain.ExchangeRateRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO exchange_rates (currency, rate, exchange_on)
		VALUES (:currency, :rate, :exchange_on)
		ON CONFLICT (currency, exchange_on) DO UPDATE SET rate = excluded.rate
	`), exchangeRateRowFrom(rate))
}

func (ExchangeRates) DeleteAll(q sqlx.Ext) (int64, error) {
	return execRowsAffected(q, `DELETE FROM exchange_rates`, map[string]any{})
}
