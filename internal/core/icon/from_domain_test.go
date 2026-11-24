package icon_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/icon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromDomain(t *testing.T) {
	ctx := context.Background()

	requestedURLs := []struct {
		name   string
		domain string
		want   string
	}{
		{"requests FaviconDL URL when domain is bare", "example.com",
			"https://favicondl.com/api/extract?size=64&url=example.com"},
		{"escapes space when domain contains space", "a b.com",
			"https://favicondl.com/api/extract?size=64&url=a+b.com"},
		{"escapes query when domain contains query string", "x.com?y=1",
			"https://favicondl.com/api/extract?size=64&url=x.com%3Fy%3D1"},
		{"escapes full URL when domain is full URL", "https://www.example.com/path?q=1#frag",
			"https://favicondl.com/api/extract?size=64&url=https%3A%2F%2Fwww.example.com%2Fpath%3Fq%3D1%23frag"},
	}
	for _, tc := range requestedURLs {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, transport := fetcherServing(http.StatusOK, fixture(t, "opaque64.png"))

			_, err := fetcher.FromDomain(ctx, tc.domain)

			require.NoError(t, err)
			assert.Equal(t, []string{tc.want}, transport.requestedURLs())
		})
	}

	statuses := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, icon.ErrNotFound},
		{http.StatusNotFound, icon.ErrNotFound},
		{http.StatusTooManyRequests, icon.ErrFetchFailed},
		{http.StatusBadGateway, icon.ErrFetchFailed},
	}
	for _, tc := range statuses {
		t.Run("returns "+tc.want.Error()+" when FaviconDL answers "+http.StatusText(tc.status), func(t *testing.T) {
			fetcher, _ := fetcherServing(tc.status, []byte("Favicon extraction failed"))

			got, err := fetcher.FromDomain(ctx, "example.com")

			assert.ErrorIs(t, err, tc.want)
			assert.Contains(t, err.Error(), "example.com")
			assert.Empty(t, got)
		})
	}

	t.Run("returns 64x64 valid icon when FaviconDL serves non-64 PNG", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, fixture(t, "normal.png"))

		got, err := fetcher.FromDomain(ctx, "example.com")

		require.NoError(t, err)
		decodeIcon(t, got)
	})

	t.Run("returns ErrFetchFailed without network when package-level ctx is cancelled", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		got, err := icon.FromDomain(cancelled, "example.com")

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, got)
	})
}
