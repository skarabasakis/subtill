package domain

import "github.com/google/uuid"

// NewID returns a new UUIDv7. IDs sort by creation time, which
// store.Subscriptions.ListAll relies on for newest-first order.
func NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}
