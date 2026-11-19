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
	"base_currency",
	"quote_currency",
	"rate",
	"exchange_on",
}

type exchangeRateRow struct {
	BaseCurrency  string `db:"base_currency"  json:"base_currency"`
	QuoteCurrency string `db:"quote_currency" json:"quote_currency"`
	Rate          string `db:"rate"           json:"rate"`
	ExchangeOn    string `db:"exchange_on"    json:"exchange_on"`
}

func exchangeRateRowFrom(r domain.ExchangeRateRecord) exchangeRateRow {
	return exchangeRateRow{
		BaseCurrency:  r.BaseCurrency,
		QuoteCurrency: r.QuoteCurrency,
		Rate:          r.Rate,
		ExchangeOn:    r.ExchangeOn.String(),
	}
}

// exchangeRateRowsFrom never returns nil, so an empty input marshals to a JSON
// [] rather than null.
func exchangeRateRowsFrom(records []domain.ExchangeRateRecord) []exchangeRateRow {
	rows := make([]exchangeRateRow, 0, len(records))
	for _, r := range records {
		rows = append(rows, exchangeRateRowFrom(r))
	}
	return rows
}

func (r exchangeRateRow) record() (domain.ExchangeRateRecord, error) {
	exchangeOn, err := civil.ParseDate(r.ExchangeOn)
	if err != nil {
		return domain.ExchangeRateRecord{}, fmt.Errorf("exchange rate %s/%s exchange_on: %w", r.BaseCurrency, r.QuoteCurrency, err)
	}
	return domain.ExchangeRateRecord{
		BaseCurrency:  r.BaseCurrency,
		QuoteCurrency: r.QuoteCurrency,
		Rate:          r.Rate,
		ExchangeOn:    exchangeOn,
	}, nil
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

// Get returns the saved rate of baseCurrency in quoteCurrency on exactly
// exchangeOn.
func (ExchangeRates) Get(q sqlx.Ext, baseCurrency, quoteCurrency string, exchangeOn civil.Date) (*domain.ExchangeRateRecord, error) {
	var row exchangeRateRow
	err := getOne(q, &row, "SELECT "+columnList(exchangeRateColumns)+" "+hereduck.D(`
		 FROM exchange_rates
		 WHERE base_currency = :base AND quote_currency = :quote AND exchange_on = :exchange_on
	`), map[string]any{"base": baseCurrency, "quote": quoteCurrency, "exchange_on": exchangeOn.String()})
	if err != nil {
		return nil, err
	}
	record, err := row.record()
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// List returns the saved rates of baseCurrency in quoteCurrency on any of
// dates, ordered by exchange_on. Dates without a saved rate are absent from
// the result.
func (ExchangeRates) List(q sqlx.Ext, baseCurrency, quoteCurrency string, dates []civil.Date) ([]domain.ExchangeRateRecord, error) {
	datesJSON, err := json.Marshal(dateStrings(dates))
	if err != nil {
		return nil, err
	}
	var rows []exchangeRateRow
	query := "SELECT " + columnList(exchangeRateColumns) + " " + hereduck.D(`
		 FROM exchange_rates
		 WHERE base_currency = :base
		   AND quote_currency = :quote
		   AND exchange_on IN (SELECT value FROM json_each(:dates))
		 ORDER BY exchange_on
	`)
	args := map[string]any{"base": baseCurrency, "quote": quoteCurrency, "dates": string(datesJSON)}
	if err := selectAll(q, &rows, query, args); err != nil {
		return nil, err
	}
	return exchangeRateRecords(rows)
}

// Upsert saves rates, replacing the rate of the same currency pair and date.
// All rates are written in one statement, so either all of them are written
// or none. It returns the number of rows written.
func (ExchangeRates) Upsert(q sqlx.Ext, rates []domain.ExchangeRateRecord) (int64, error) {
	ratesJSON, err := json.Marshal(exchangeRateRowsFrom(rates))
	if err != nil {
		return 0, err
	}
	// SQLite needs a WHERE clause between INSERT ... SELECT and ON CONFLICT to
	// parse the upsert unambiguously.
	return execRowsAffected(q, hereduck.D(`
		INSERT INTO exchange_rates (base_currency, quote_currency, rate, exchange_on)
		SELECT j.value ->> 'base_currency', j.value ->> 'quote_currency', j.value ->> 'rate', j.value ->> 'exchange_on'
		  FROM json_each(:rates) AS j
		 WHERE true
		ON CONFLICT (base_currency, quote_currency, exchange_on) DO UPDATE SET rate = excluded.rate
	`), map[string]any{"rates": string(ratesJSON)})
}
