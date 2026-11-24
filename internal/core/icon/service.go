// Package icon resolves subscription icons from a domain or an image URL and
// validates icons supplied by callers. Every icon is a 64×64 PNG encoded as a
// data URI.
//
// See docs/adr/20261021-icon-service.md.
package icon

import "net/http"

//go:generate go run ./internal/genfixtures

var defaultService = NewIconFetcher()

// IconFetcher downloads images and turns them into icons. The package-level
// functions use a default IconFetcher; use NewIconFetcher only when a
// different HTTP client is needed.
type IconFetcher struct {
	httpClient *http.Client
}

type Option func(*IconFetcher)

// WithHTTPClient replaces the default HTTP client, which has no timeout of
// its own: the caller's context is the only time limit.
func WithHTTPClient(c *http.Client) Option {
	return func(s *IconFetcher) { s.httpClient = c }
}

// NewIconFetcher returns an IconFetcher configured with opts.
func NewIconFetcher(opts ...Option) *IconFetcher {
	s := &IconFetcher{httpClient: &http.Client{}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
