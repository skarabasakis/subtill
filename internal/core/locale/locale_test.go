package locale

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCurrencyFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"region with encoding", map[string]string{"LANG": "de_DE.UTF-8"}, "EUR"},
		{"region with modifier", map[string]string{"LANG": "fr_FR@euro"}, "EUR"},
		{"hyphenated tag", map[string]string{"LANG": "en-GB"}, "GBP"},
		{"LC_ALL overrides LANG", map[string]string{"LC_ALL": "ja_JP.UTF-8", "LANG": "en_GB.UTF-8"}, "JPY"},
		{"LC_MONETARY overrides LANG", map[string]string{"LC_MONETARY": "el_GR.UTF-8", "LANG": "en_GB.UTF-8"}, "EUR"},
		{"POSIX locale", map[string]string{"LANG": "C.UTF-8"}, FallbackCurrency},
		{"language without region", map[string]string{"LANG": "de"}, FallbackCurrency},
		{"first set variable is authoritative", map[string]string{"LC_ALL": "C", "LANG": "de_DE.UTF-8"}, FallbackCurrency},
		{"unset", map[string]string{}, FallbackCurrency},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := currencyFromEnv(func(key string) string { return tc.env[key] })
			assert.Equal(t, tc.want, got)
		})
	}
}
