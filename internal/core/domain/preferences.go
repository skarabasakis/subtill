package domain

type PreferencesRecord struct {
	Currency string `json:"currency" db:"currency"`
}

// Preferences is a partial set of preferences. Nil fields are left unchanged.
type Preferences struct {
	Currency *string `json:"currency,omitempty"`
}

// AppliedTo returns base with the set fields of p.
func (p Preferences) AppliedTo(base PreferencesRecord) PreferencesRecord {
	if p.Currency != nil {
		base.Currency = *p.Currency
	}
	return base
}
