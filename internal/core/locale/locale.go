// Package locale derives defaults for new databases from the OS locale.
package locale

import (
	"os"
	"strings"

	"github.com/skarabasakis/subtill/internal/core/domain"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

// FallbackCurrency is used when the locale names no region with a currency.
const FallbackCurrency = "USD"

// DefaultPreferences returns the preferences of a new database.
func DefaultPreferences() domain.PreferencesRecord {
	return domain.PreferencesRecord{
		Currency:               Currency(),
		CurrencyConversionMode: domain.CurrencyConversionModeApproximate,
	}
}

// Currency returns the currency of the region named by the POSIX locale
// environment (LC_ALL, then LC_MONETARY, then LANG), or FallbackCurrency.
func Currency() string {
	return currencyFromEnv(os.Getenv)
}

func currencyFromEnv(getenv func(string) string) string {
	for _, key := range []string{"LC_ALL", "LC_MONETARY", "LANG"} {
		if value := getenv(key); value != "" {
			if c, ok := currencyForLocale(value); ok {
				return c
			}
			return FallbackCurrency
		}
	}
	return FallbackCurrency
}

// currencyForLocale maps a POSIX locale name such as "de_DE.UTF-8@euro" to
// the currency of its region. Locales without an explicit region don't match.
func currencyForLocale(name string) (string, bool) {
	if i := strings.IndexAny(name, ".@"); i >= 0 {
		name = name[:i]
	}
	tag, err := language.Parse(strings.ReplaceAll(name, "_", "-"))
	if err != nil {
		return "", false
	}
	region, confidence := tag.Region()
	if confidence != language.Exact {
		return "", false
	}
	unit, ok := currency.FromRegion(region)
	if !ok {
		return "", false
	}
	return unit.String(), true
}
