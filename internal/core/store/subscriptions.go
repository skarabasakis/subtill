package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"cloud.google.com/go/civil"
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type Subscriptions struct{}

var subscriptionColumns = []string{
	"id",
	"name",
	"group",
	"notes",
	"icon",
	"url",
}

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

// Count returns the number of subscriptions matching filter. A nil filter
// matches every subscription. currentDate is only used by status filters.
func (Subscriptions) Count(q sqlx.Ext, filter *domain.SubscriptionFiltersDb, currentDate civil.Date) (int, error) {
	clauses, args, err := subscriptionFilterClauses(filter, currentDate)
	if err != nil {
		return 0, err
	}
	var count int
	err = getOne(q, &count, "SELECT COUNT(*) FROM subscriptions s WHERE 1=1"+clauses, args)
	return count, err
}

// GetByName returns the subscription named name, compared case-insensitively.
func (Subscriptions) GetByName(q sqlx.Ext, name string) (*domain.SubscriptionRecord, error) {
	return getSubscription(q, "s.name = :value", name)
}

func (Subscriptions) GetById(q sqlx.Ext, id string) (*domain.SubscriptionRecord, error) {
	return getSubscription(q, "s.id = :value", id)
}

func getSubscription(q sqlx.Ext, predicate, value string) (*domain.SubscriptionRecord, error) {
	var row subscriptionRow
	query := "SELECT " + columnList(subscriptionColumns) + " FROM subscriptions s WHERE " + predicate
	if err := getOne(q, &row, query, map[string]any{"value": value}); err != nil {
		return nil, err
	}
	record := row.record()
	return &record, nil
}

// ListAll returns the subscriptions matching filter, newest id first. A nil
// filter matches every subscription. currentDate is only used by status
// filters.
func (Subscriptions) ListAll(q sqlx.Ext, filter *domain.SubscriptionFiltersDb, currentDate civil.Date) ([]domain.SubscriptionRecord, error) {
	clauses, args, err := subscriptionFilterClauses(filter, currentDate)
	if err != nil {
		return nil, err
	}
	var rows []subscriptionRow
	query := "SELECT " + columnList(subscriptionColumns) + " FROM subscriptions s WHERE 1=1" + clauses + " ORDER BY s.id DESC"
	if err := selectAll(q, &rows, query, args); err != nil {
		return nil, err
	}
	records := make([]domain.SubscriptionRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, row.record())
	}
	return records, nil
}

// Insert inserts the subscription row. Tags are inserted with Tags.Insert.
func (Subscriptions) Insert(q sqlx.Ext, subscription domain.SubscriptionRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO subscriptions (id, name, "group", notes, icon, url)
		VALUES (:id, :name, NULLIF(:group, ''), NULLIF(:notes, ''), NULLIF(:icon, ''), NULLIF(:url, ''))
	`), subscription)
}

// Update replaces every column of the subscription except its id.
func (Subscriptions) Update(q sqlx.Ext, subscription domain.SubscriptionRecord) (int64, error) {
	return execRowsAffected(q, hereduck.D(`
		UPDATE subscriptions
		   SET name = :name, "group" = NULLIF(:group, ''), notes = NULLIF(:notes, ''),
		       icon = NULLIF(:icon, ''), url = NULLIF(:url, '')
		 WHERE id = :id
	`), subscription)
}

// DeleteById deletes the subscription with its tags, billing periods and
// reminders.
func (Subscriptions) DeleteById(q sqlx.Ext, id string) (int64, error) {
	return execRowsAffected(q, `DELETE FROM subscriptions WHERE id = :id`, map[string]any{"id": id})
}

// associationsSelect selects each subscription's id with its tags and billing
// periods as JSON arrays. Each association is aggregated in its own subquery:
// joining both tables would multiply rows (tags x billing periods) and
// corrupt the arrays.
var associationsSelect = "SELECT s.id, " +
	tagsAggregate("s.id") + " AS tags, " +
	billingPeriodsAggregate("s.id") + " AS billing_periods" +
	" FROM subscriptions s"

type associationsRow struct {
	ID             string `db:"id"`
	Tags           string `db:"tags"`
	BillingPeriods string `db:"billing_periods"`
}

// WithAssociations loads the tags and billing periods of a subscription that
// was already read.
func (Subscriptions) WithAssociations(q sqlx.Ext, subscription domain.SubscriptionRecord) (domain.SubscriptionWithAssociations, error) {
	var row associationsRow
	query := associationsSelect + " WHERE s.id = :id"
	if err := getOne(q, &row, query, map[string]any{"id": subscription.ID}); err != nil {
		return domain.SubscriptionWithAssociations{}, err
	}
	return row.attachTo(subscription)
}

// WithAssociationsA loads the tags and billing periods of many subscriptions
// in one query. The result keeps the order of subscriptions.
func (Subscriptions) WithAssociationsA(q sqlx.Ext, subscriptions []domain.SubscriptionRecord) ([]domain.SubscriptionWithAssociations, error) {
	if len(subscriptions) == 0 {
		return []domain.SubscriptionWithAssociations{}, nil
	}
	rowsByID, err := associationRowsByID(q, subscriptionIDs(subscriptions))
	if err != nil {
		return nil, err
	}
	result := make([]domain.SubscriptionWithAssociations, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		row, ok := rowsByID[subscription.ID]
		if !ok {
			return nil, fmt.Errorf("subscription %s: %w", subscription.ID, ErrNotFound)
		}
		withAssociations, err := row.attachTo(subscription)
		if err != nil {
			return nil, err
		}
		result = append(result, withAssociations)
	}
	return result, nil
}

func subscriptionIDs(subscriptions []domain.SubscriptionRecord) []string {
	ids := make([]string, 0, len(subscriptions))
	for _, s := range subscriptions {
		ids = append(ids, s.ID)
	}
	return ids
}

func associationRowsByID(q sqlx.Ext, ids []string) (map[string]associationsRow, error) {
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	var rows []associationsRow
	query := associationsSelect + " WHERE s.id IN (SELECT value FROM json_each(:ids))"
	if err := selectAll(q, &rows, query, map[string]any{"ids": string(idsJSON)}); err != nil {
		return nil, err
	}
	byID := make(map[string]associationsRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	return byID, nil
}

func (r associationsRow) attachTo(subscription domain.SubscriptionRecord) (domain.SubscriptionWithAssociations, error) {
	tags, err := decodeTags(r.Tags)
	if err != nil {
		return domain.SubscriptionWithAssociations{}, fmt.Errorf("subscription %s tags: %w", r.ID, err)
	}
	billingPeriods, err := decodeBillingPeriods(r.BillingPeriods)
	if err != nil {
		return domain.SubscriptionWithAssociations{}, fmt.Errorf("subscription %s billing periods: %w", r.ID, err)
	}
	subscription.Tags = tags
	return domain.SubscriptionWithAssociations{SubscriptionRecord: subscription, BillingPeriods: billingPeriods}, nil
}

// subscriptionFilterClauses returns the predicates for filter, to append to a
// WHERE clause on subscriptions aliased s, and their named arguments.
func subscriptionFilterClauses(filter *domain.SubscriptionFiltersDb, currentDate civil.Date) (string, map[string]any, error) {
	args := map[string]any{}
	if filter == nil {
		return "", args, nil
	}
	var clauses strings.Builder
	clauses.WriteString(nameClause(filter.Name, args))
	clauses.WriteString(groupClause(filter.Group, args))
	tags, err := tagsClause(filter.Tags, args)
	if err != nil {
		return "", nil, err
	}
	clauses.WriteString(tags)
	status, err := statusClause(filter.Status, currentDate, args)
	if err != nil {
		return "", nil, err
	}
	clauses.WriteString(status)
	return clauses.String(), args, nil
}

func nameClause(filter domain.StringFilter, args map[string]any) string {
	if filter.Value == "" {
		return ""
	}
	args["name"] = filter.Value
	if filter.Operator == domain.StringFilterOperatorContains {
		return "\nAND instr(lower(s.name), lower(:name)) > 0"
	}
	return "\nAND s.name = :name"
}

func groupClause(group string, args map[string]any) string {
	if group == "" {
		return ""
	}
	args["group"] = group
	return "\nAND s.\"group\" = :group"
}

func tagsClause(filter domain.ArrayFilter, args map[string]any) (string, error) {
	if len(filter.Values) == 0 {
		return "", nil
	}
	tags, err := json.Marshal(filter.Values)
	if err != nil {
		return "", err
	}
	args["tags"] = string(tags)
	if filter.Operator == domain.ArrayFilterOperatorAll {
		return "\n" + hereduck.D(`
			AND NOT EXISTS (SELECT 1 FROM json_each(:tags) j
			                 WHERE NOT EXISTS (SELECT 1 FROM tags t
			                                    WHERE t.subscription_id = s.id
			                                      AND t.tag = j.value))
		`), nil
	}
	return "\n" + hereduck.D(`
		AND EXISTS (SELECT 1 FROM tags t
		             WHERE t.subscription_id = s.id
		               AND t.tag IN (SELECT value FROM json_each(:tags)))
	`), nil
}

func statusClause(statuses []domain.SubscriptionStatus, currentDate civil.Date, args map[string]any) (string, error) {
	if len(statuses) == 0 {
		return "", nil
	}
	status, err := json.Marshal(statuses)
	if err != nil {
		return "", err
	}
	args["status"] = string(status)
	args["current_date"] = currentDate.String()
	return "\n" + hereduck.D(`
		AND (SELECT CASE
		              WHEN MAX(b.start_on <= :current_date
		                       AND (b.end_on IS NULL OR b.end_on > :current_date)) THEN 'ongoing'
		              WHEN MAX(b.start_on > :current_date) THEN 'scheduled'
		              ELSE 'ended'
		            END
		       FROM billing_periods b
		      WHERE b.subscription_id = s.id) IN (SELECT value FROM json_each(:status))
	`), nil
}
