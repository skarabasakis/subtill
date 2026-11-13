package domain

type SubscriptionStatus string

const (
	SubscriptionStatusEnded     SubscriptionStatus = "ended"
	SubscriptionStatusOngoing   SubscriptionStatus = "ongoing"
	SubscriptionStatusScheduled SubscriptionStatus = "scheduled"
)

// SubscriptionRecord is a row of the subscriptions table. Tags live in their
// own table and are only filled when associations are loaded.
type SubscriptionRecord struct {
	ID    string   `json:"id"    db:"id"`
	Name  string   `json:"name"  db:"name"`
	Group string   `json:"group" db:"group"`
	Tags  []string `json:"tags"  db:"-"`
	Notes string   `json:"notes" db:"notes"`
	Icon  string   `json:"icon"  db:"icon"`
	URL   string   `json:"url"   db:"url"`
}

// SubscriptionWithAssociations is a subscription record with its tags (in
// SubscriptionRecord.Tags) and billing periods loaded.
type SubscriptionWithAssociations struct {
	SubscriptionRecord
	BillingPeriods []BillingPeriodRecord `json:"billingPeriods"`
}
