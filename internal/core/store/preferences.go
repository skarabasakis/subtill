package store

import (
	"crdx.org/hereduck"
	"github.com/jmoiron/sqlx"
	"github.com/skarabasakis/subtill/internal/core/domain"
)

type Preferences struct{}

var preferencesColumns = []string{
	"currency",
	"currency_conversion_mode",
}

// Get returns the singleton preferences row.
func (Preferences) Get(q sqlx.Ext) (*domain.PreferencesRecord, error) {
	var p domain.PreferencesRecord
	query := "SELECT " + columnList(preferencesColumns) + " FROM preferences WHERE id = 1"
	if err := getOne(q, &p, query, map[string]any{}); err != nil {
		return nil, err
	}
	return &p, nil
}

// Upsert creates the singleton preferences row or replaces its values.
func (Preferences) Upsert(q sqlx.Ext, preferences domain.PreferencesRecord) error {
	return exec(q, hereduck.D(`
		INSERT INTO preferences (id, currency, currency_conversion_mode)
		VALUES (1, :currency, :currency_conversion_mode)
		ON CONFLICT (id) DO UPDATE
		   SET currency = excluded.currency,
		       currency_conversion_mode = excluded.currency_conversion_mode
	`), preferences)
}
