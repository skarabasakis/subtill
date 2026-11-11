package store

import (
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
)

type SubscriptionRepository struct{}

// Count returns the number of subscriptions.
func (SubscriptionRepository) Count(q sqlx.Ext) (int, error) {
	var count int
	err := sqlx.Get(q, &count, hereduck.D(`
		SELECT COUNT(*) FROM subscriptions s WHERE 1=1
	`))
	return count, err
}
