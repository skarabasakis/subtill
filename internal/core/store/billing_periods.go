package store

import (
	"encoding/json"

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
