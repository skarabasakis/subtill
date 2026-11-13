package store

import (
	"encoding/json"

	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type Tags struct{}

var tagColumns = []string{
	"id",
	"subscription_id",
	"tag",
}

const tagsOrder = `ORDER BY tag`

// ListUnique returns the distinct tags in use, case-insensitively.
func (Tags) ListUnique(q sqlx.Ext) ([]string, error) {
	tags := []string{}
	err := selectAll(q, &tags, "SELECT DISTINCT tag FROM tags "+tagsOrder, map[string]any{})
	return tags, err
}

func (Tags) ListBySubscriptionID(q sqlx.Ext, subscriptionID string) ([]domain.TagRecord, error) {
	tags := []domain.TagRecord{}
	query := "SELECT " + columnList(tagColumns) + " FROM tags WHERE subscription_id = :subscription_id " + tagsOrder
	err := selectAll(q, &tags, query, map[string]any{"subscription_id": subscriptionID})
	return tags, err
}

// tagsAggregate returns a scalar subquery with the tags of the subscription
// identified by subscriptionIDExpr, as a JSON array of strings.
func tagsAggregate(subscriptionIDExpr string) string {
	return "(SELECT json_group_array(tag) FROM (SELECT tag FROM tags WHERE subscription_id = " +
		subscriptionIDExpr + " " + tagsOrder + "))"
}

func decodeTags(data string) ([]string, error) {
	tags := []string{}
	if err := json.Unmarshal([]byte(data), &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// Insert adds a tag to a subscription. Adding a tag the subscription already
// has, in any case, is a no-op.
func (Tags) Insert(q sqlx.Ext, tag domain.TagRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO tags (id, subscription_id, tag)
		VALUES (:id, :subscription_id, :tag)
		ON CONFLICT (subscription_id, tag) DO NOTHING
	`), tag)
}

func (Tags) DeleteBySubscriptionID(q sqlx.Ext, subscriptionID string) (int64, error) {
	return execRowsAffected(q, `DELETE FROM tags WHERE subscription_id = :subscription_id`,
		map[string]any{"subscription_id": subscriptionID})
}

// DeleteBySubscriptionIDAndTag removes one tag, compared case-insensitively.
func (Tags) DeleteBySubscriptionIDAndTag(q sqlx.Ext, subscriptionID, tag string) (int64, error) {
	return execRowsAffected(q, `DELETE FROM tags WHERE subscription_id = :subscription_id AND tag = :tag`,
		map[string]any{"subscription_id": subscriptionID, "tag": tag})
}
