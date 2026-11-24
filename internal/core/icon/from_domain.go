package icon

import (
	"context"
	"fmt"
	"net/url"
)

const faviconDLExtractURL = "https://favicondl.com/api/extract"

// FromDomain calls IconFetcher.FromDomain on the default IconFetcher.
func FromDomain(ctx context.Context, domain string) (icon string, err error) {
	return defaultService.FromDomain(ctx, domain)
}

// FromDomain resolves the icon of domain through FaviconDL and returns it as
// a 64×64 PNG data URI.
func (s *IconFetcher) FromDomain(ctx context.Context, domain string) (icon string, err error) {
	icon, err = s.FromImageURL(ctx, faviconURL(domain))
	if err != nil {
		return "", fmt.Errorf("domain %q: %w", domain, err)
	}
	return icon, nil
}

func faviconURL(domain string) string {
	query := url.Values{}
	query.Set("url", domain)
	query.Set("size", "64")
	return faviconDLExtractURL + "?" + query.Encode()
}
