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
	Currency   string `db:"currency"    json:"currency"`
	Rate       string `db:"rate"        json:"rate"`
	ExchangeOn string `db:"exchange_on" json:"exchange_on"`
}

func exchangeRateRowFrom(r domain.ExchangeRateRecord) exchangeRateRow {
	return exchangeRateRow{Currency: r.Currency, Rate: r.Rate, ExchangeOn: r.ExchangeOn.String()}
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

// Get returns the saved rate of currency on exactly exchangeOn.
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

// List returns the saved rates of currency on any of dates, ordered by
// exchange_on. Dates without a saved rate are absent from the result.
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

// Upsert saves rates, replacing the rate of the same currency and date, only
// while the preferred currency is quoteCurrency. All rates are written in one
// statement, so either all of them are written or none. It returns the number
// of rows written, which is 0 when the preferred currency differs.
func (ExchangeRates) Upsert(q sqlx.Ext, quoteCurrency string, rates []domain.ExchangeRateRecord) (int64, error) {
	ratesJSON, err := json.Marshal(exchangeRateRowsFrom(rates))
	if err != nil {
		return 0, err
	}
	return execRowsAffected(q, hereduck.D(`
		INSERT INTO exchange_rates (currency, rate, exchange_on)
		SELECT j.value ->> 'currency', j.value ->> 'rate', j.value ->> 'exchange_on'
		  FROM json_each(:rates) AS j
		 WHERE (SELECT currency FROM preferences WHERE id = 1) = :quote
		ON CONFLICT (currency, exchange_on) DO UPDATE SET rate = excluded.rate
	`), map[string]any{"rates": string(ratesJSON), "quote": quoteCurrency})
}

func (ExchangeRates) DeleteAll(q sqlx.Ext) (int64, error) {
	return execRowsAffected(q, `DELETE FROM exchange_rates`, map[string]any{})
}
