package domain_test

import (
	"testing"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestPreferencesAppliedTo(t *testing.T) {
	base := domain.PreferencesRecord{
		Currency:               "USD",
		CurrencyConversionMode: domain.CurrencyConversionModeApproximate,
	}
	currency := "EUR"
	live := domain.CurrencyConversionModeLive

	cases := []struct {
		name        string
		preferences domain.Preferences
		want        domain.PreferencesRecord
	}{
		{"no fields set", domain.Preferences{}, base},
		{"currency set", domain.Preferences{Currency: &currency}, domain.PreferencesRecord{
			Currency:               "EUR",
			CurrencyConversionMode: domain.CurrencyConversionModeApproximate,
		}},
		{"both set", domain.Preferences{Currency: &currency, CurrencyConversionMode: &live}, domain.PreferencesRecord{
			Currency:               "EUR",
			CurrencyConversionMode: domain.CurrencyConversionModeLive,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.preferences.AppliedTo(base))
		})
	}
}
