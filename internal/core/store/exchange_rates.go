package store

import (
	"encoding/json"
	"fmt"

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

type exchangeRateRow struct {
	Currency   string `db:"currency"`
	Rate       string `db:"rate"`
	ExchangeOn string `db:"exchange_on"`
}

func exchangeRateRowFrom(r domain.ExchangeRateRecord) exchangeRateRow {
	return exchangeRateRow{Currency: r.Currency, Rate: r.Rate, ExchangeOn: r.ExchangeOn.String()}
}

func (r exchangeRateRow) record() (domain.ExchangeRateRecord, error) {
	exchangeOn, err := civil.ParseDate(r.ExchangeOn)
	if err != nil {
		return domain.ExchangeRateRecord{}, fmt.Errorf("exchange rate %s exchange_on: %w", r.Currency, err)
	}
	return domain.ExchangeRateRecord{Currency: r.Currency, Rate: r.Rate, ExchangeOn: exchangeOn}, nil
}

func exchangeRateRecords(rows []exchangeRateRow) ([]domain.ExchangeRateRecord, error) {
	records := make([]domain.ExchangeRateRecord, 0, len(rows))
	for _, row := range rows {
		record, err := row.record()
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
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

// List returns the cached rates of currency on any of dates, ordered by
// exchange_on. Dates without a cached rate are absent from the result.
func (ExchangeRates) List(q sqlx.Ext, currency string, dates []civil.Date) ([]domain.ExchangeRateRecord, error) {
	datesJSON, err := json.Marshal(dateStrings(dates))
	if err != nil {
		return nil, err
	}
	var rows []exchangeRateRow
	query := "SELECT " + columnList(exchangeRateColumns) + " " + hereduck.D(`
		 FROM exchange_rates
		 WHERE currency = :currency
		   AND exchange_on IN (SELECT value FROM json_each(:dates))
		 ORDER BY exchange_on
	`)
	if err := selectAll(q, &rows, query, map[string]any{"currency": currency, "dates": string(datesJSON)}); err != nil {
		return nil, err
	}
	return exchangeRateRecords(rows)
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
