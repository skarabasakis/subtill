package domain

type TagRecord struct {
	ID             string `json:"id"             db:"id"`
	SubscriptionID string `json:"subscriptionId" db:"subscription_id"`
	Tag            string `json:"tag"            db:"tag"`
}
