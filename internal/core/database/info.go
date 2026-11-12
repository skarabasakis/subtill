package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/skarabasakis/subtill/internal/core/store"
)

type DBInfo struct {
	SchemaVersion     uint                     `json:"schemaVersion"`
	SubscriptionCount int                      `json:"subscriptionCount"`
	Preferences       domain.PreferencesRecord `json:"preferences"`
}

// Info calls Service.Info on the Service with the embedded migrations.
func Info(db *sqlx.DB) (*DBInfo, error) {
	return defaultService.Info(db)
}

// Info describes the database behind a pool returned by Open.
func (s *Service) Info(db *sqlx.DB) (*DBInfo, error) {
	count, err := store.Subscriptions{}.Count(db)
	if err != nil {
		return nil, fmt.Errorf("count subscriptions: %w", err)
	}
	preferences, err := store.Preferences{}.Get(db)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	return &DBInfo{
		SchemaVersion:     s.schemaVersion,
		SubscriptionCount: count,
		Preferences:       *preferences,
	}, nil
}
