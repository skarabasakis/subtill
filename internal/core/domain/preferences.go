package domain

type CurrencyConversionMode string

const (
	CurrencyConversionModeApproximate CurrencyConversionMode = "approximate"
	CurrencyConversionModeLive        CurrencyConversionMode = "live"
)

type PreferencesRecord struct {
	Currency               string                 `json:"currency"               db:"currency"`
	CurrencyConversionMode CurrencyConversionMode `json:"currencyConversionMode" db:"currency_conversion_mode"`
}

// Preferences is a partial set of preferences. Nil fields are left unchanged.
type Preferences struct {
	Currency               *string                 `json:"currency,omitempty"`
	CurrencyConversionMode *CurrencyConversionMode `json:"currencyConversionMode,omitempty"`
}

// AppliedTo returns base with the set fields of p.
func (p Preferences) AppliedTo(base PreferencesRecord) PreferencesRecord {
	if p.Currency != nil {
		base.Currency = *p.Currency
	}
	if p.CurrencyConversionMode != nil {
		base.CurrencyConversionMode = *p.CurrencyConversionMode
	}
	return base
}
