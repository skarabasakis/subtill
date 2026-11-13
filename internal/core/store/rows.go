package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/civil"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

// Row types mirror table columns with database-friendly field types. Domain
// records use civil.Date; rows store dates as YYYY-MM-DD strings.

type subscriptionRow struct {
	ID    string         `db:"id"`
	Name  string         `db:"name"`
	Group sql.NullString `db:"group"`
	Notes sql.NullString `db:"notes"`
	Icon  sql.NullString `db:"icon"`
	URL   sql.NullString `db:"url"`
}

func (r subscriptionRow) record() domain.SubscriptionRecord {
	return domain.SubscriptionRecord{
		ID:    r.ID,
		Name:  r.Name,
		Group: r.Group.String,
		Notes: r.Notes.String,
		Icon:  r.Icon.String,
		URL:   r.URL.String,
	}
}

// billingPeriodRow is scanned from billing_periods columns and also decoded
// from the JSON objects of billingPeriodsAggregate, so its json tags must
// match its db tags.
type billingPeriodRow struct {
	ID              string  `db:"id"               json:"id"`
	SubscriptionID  string  `db:"subscription_id"  json:"subscription_id"`
	BillingCurrency string  `db:"billing_currency" json:"billing_currency"`
	BillingCost     string  `db:"billing_cost"     json:"billing_cost"`
	Cost            string  `db:"cost"             json:"cost"`
	Frequency       int     `db:"frequency"        json:"frequency"`
	StartOn         string  `db:"start_on"         json:"start_on"`
	EndOn           *string `db:"end_on"           json:"end_on"`
}

func billingPeriodRowFrom(r domain.BillingPeriodRecord) billingPeriodRow {
	return billingPeriodRow{
		ID:              r.ID,
		SubscriptionID:  r.SubscriptionID,
		BillingCurrency: r.BillingCurrency,
		BillingCost:     r.BillingCost,
		Cost:            r.Cost,
		Frequency:       r.Frequency,
		StartOn:         r.StartOn.String(),
		EndOn:           nullableDate(r.EndOn),
	}
}

func (r billingPeriodRow) record() (domain.BillingPeriodRecord, error) {
	startOn, err := civil.ParseDate(r.StartOn)
	if err != nil {
		return domain.BillingPeriodRecord{}, fmt.Errorf("billing period %s start_on: %w", r.ID, err)
	}
	endOn, err := parseNullableDate(r.EndOn)
	if err != nil {
		return domain.BillingPeriodRecord{}, fmt.Errorf("billing period %s end_on: %w", r.ID, err)
	}
	return domain.BillingPeriodRecord{
		ID:              r.ID,
		SubscriptionID:  r.SubscriptionID,
		StartOn:         startOn,
		EndOn:           endOn,
		Frequency:       r.Frequency,
		BillingCurrency: r.BillingCurrency,
		BillingCost:     r.BillingCost,
		Cost:            r.Cost,
	}, nil
}

func billingPeriodRecords(rows []billingPeriodRow) ([]domain.BillingPeriodRecord, error) {
	records := make([]domain.BillingPeriodRecord, 0, len(rows))
	for _, row := range rows {
		record, err := row.record()
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

type reminderRow struct {
	ID             string                `db:"id"`
	SubscriptionID string                `db:"subscription_id"`
	Action         domain.ReminderAction `db:"action"`
	CustomNote     sql.NullString        `db:"custom_note"`
	TriggerOn      string                `db:"trigger_on"`
	IsTriggered    bool                  `db:"is_triggered"`
}

func reminderRowFrom(r domain.ReminderRecord) reminderRow {
	return reminderRow{
		ID:             r.ID,
		SubscriptionID: r.SubscriptionID,
		Action:         r.Action,
		CustomNote:     sql.NullString{String: r.CustomNote, Valid: true},
		TriggerOn:      r.TriggerOn.String(),
		IsTriggered:    r.IsTriggered,
	}
}

func (r reminderRow) record() (domain.ReminderRecord, error) {
	triggerOn, err := civil.ParseDate(r.TriggerOn)
	if err != nil {
		return domain.ReminderRecord{}, fmt.Errorf("reminder %s trigger_on: %w", r.ID, err)
	}
	return domain.ReminderRecord{
		ID:             r.ID,
		SubscriptionID: r.SubscriptionID,
		Action:         r.Action,
		CustomNote:     r.CustomNote.String,
		TriggerOn:      triggerOn,
		IsTriggered:    r.IsTriggered,
	}, nil
}

func reminderRecords(rows []reminderRow) ([]domain.ReminderRecord, error) {
	records := make([]domain.ReminderRecord, 0, len(rows))
	for _, row := range rows {
		record, err := row.record()
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
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

func nullableDate(d *civil.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func parseNullableDate(s *string) (*civil.Date, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	d, err := civil.ParseDate(*s)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// columnList returns columns as a comma-separated list of quoted identifiers,
// so that column names that are keywords, like "group", need no special care.
func columnList(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, `"`+column+`"`)
	}
	return strings.Join(quoted, ", ")
}

// jsonObject returns a json_object expression keyed by column name, with the
// values read from the columns of alias.
func jsonObject(alias string, columns []string) string {
	pairs := make([]string, 0, len(columns))
	for _, column := range columns {
		pairs = append(pairs, fmt.Sprintf(`'%s', %s."%s"`, column, alias, column))
	}
	return "json_object(" + strings.Join(pairs, ", ") + ")"
}

// getOne scans the single row of query into dest, mapping no rows to
// ErrNotFound.
func getOne(q sqlx.Ext, dest any, query string, arg any) error {
	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return err
	}
	err = sqlx.Get(q, dest, q.Rebind(query), args...)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func selectAll(q sqlx.Ext, dest any, query string, arg any) error {
	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return err
	}
	return sqlx.Select(q, dest, q.Rebind(query), args...)
}

func exec(q sqlx.Ext, query string, arg any) error {
	_, err := sqlx.NamedExec(q, query, arg)
	return err
}

func execRowsAffected(q sqlx.Ext, query string, arg any) (int64, error) {
	result, err := sqlx.NamedExec(q, query, arg)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
