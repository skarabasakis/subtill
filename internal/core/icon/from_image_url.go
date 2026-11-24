package icon

import "context"

// FromImageURL calls IconFetcher.FromImageURL on the default IconFetcher.
func FromImageURL(ctx context.Context, rawURL string) (icon string, err error) {
	return defaultService.FromImageURL(ctx, rawURL)
}

// FromImageURL downloads the PNG or ICO image at rawURL and returns it as a
// 64×64 PNG data URI. The caller's ctx is the only time limit.
func (s *IconFetcher) FromImageURL(ctx context.Context, rawURL string) (icon string, err error) {
	body, err := s.fetch(ctx, rawURL)
	if err != nil {
		return "", err
	}
	img, err := decode(body)
	if err != nil {
		return "", err
	}
	png, err := process(img)
	if err != nil {
		return "", err
	}
	return toDataURI(png), nil
}
