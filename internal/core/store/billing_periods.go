package store

import (
	"encoding/json"
	"fmt"

	"cloud.google.com/go/civil"
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type BillingPeriods struct{}

var billingPeriodColumns = []string{
	"id",
	"subscription_id",
	"billing_currency",
	"billing_cost",
	"cost",
	"frequency",
	"start_on",
	"end_on",
}

const billingPeriodsOrder = `ORDER BY start_on`

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

// ListBySubscriptionID returns the billing periods of a subscription, oldest
// first.
func (BillingPeriods) ListBySubscriptionID(q sqlx.Ext, subscriptionID string) ([]domain.BillingPeriodRecord, error) {
	var rows []billingPeriodRow
	query := "SELECT " + columnList(billingPeriodColumns) + " FROM billing_periods WHERE subscription_id = :subscription_id " + billingPeriodsOrder
	if err := selectAll(q, &rows, query, map[string]any{"subscription_id": subscriptionID}); err != nil {
		return nil, err
	}
	return billingPeriodRecords(rows)
}

// billingPeriodsAggregate returns a scalar subquery with the billing periods
// of the subscription identified by subscriptionIDExpr, as a JSON array of
// objects keyed by column name.
func billingPeriodsAggregate(subscriptionIDExpr string) string {
	return "(SELECT json_group_array(" + jsonObject("b", billingPeriodColumns) + ")" +
		" FROM (SELECT " + columnList(billingPeriodColumns) + " FROM billing_periods" +
		" WHERE subscription_id = " + subscriptionIDExpr + " " + billingPeriodsOrder + ") b)"
}

func decodeBillingPeriods(data string) ([]domain.BillingPeriodRecord, error) {
	var rows []billingPeriodRow
	if err := json.Unmarshal([]byte(data), &rows); err != nil {
		return nil, err
	}
	return billingPeriodRecords(rows)
}

func (BillingPeriods) GetById(q sqlx.Ext, id string) (*domain.BillingPeriodRecord, error) {
	var row billingPeriodRow
	query := "SELECT " + columnList(billingPeriodColumns) + " FROM billing_periods WHERE id = :id"
	if err := getOne(q, &row, query, map[string]any{"id": id}); err != nil {
		return nil, err
	}
	record, err := row.record()
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (BillingPeriods) Insert(q sqlx.Ext, billingPeriod domain.BillingPeriodRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO billing_periods (id, subscription_id, billing_currency, billing_cost, cost, frequency, start_on, end_on)
		VALUES (:id, :subscription_id, :billing_currency, :billing_cost, :cost, :frequency, :start_on, NULLIF(:end_on, ''))
	`), billingPeriodRowFrom(billingPeriod))
}

// Update replaces every column of the billing period except its id and
// subscription. A nil EndOn clears the end date.
func (BillingPeriods) Update(q sqlx.Ext, billingPeriod domain.BillingPeriodRecord) (int64, error) {
	return execRowsAffected(q, hereduck.D(`
		UPDATE billing_periods
		   SET billing_currency = :billing_currency, billing_cost = :billing_cost, cost = :cost,
		       frequency = :frequency, start_on = :start_on, end_on = NULLIF(:end_on, '')
		 WHERE id = :id
	`), billingPeriodRowFrom(billingPeriod))
}

// UpdateCostById updates only the cost in the preferred currency.
func (BillingPeriods) UpdateCostById(q sqlx.Ext, id, cost string) (int64, error) {
	return execRowsAffected(q, `UPDATE billing_periods SET cost = :cost WHERE id = :id`,
		map[string]any{"id": id, "cost": cost})
}

func (BillingPeriods) DeleteById(q sqlx.Ext, id string) (int64, error) {
	return execRowsAffected(q, `DELETE FROM billing_periods WHERE id = :id`, map[string]any{"id": id})
}
