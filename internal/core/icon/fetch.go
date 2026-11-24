package icon

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const maxBodySize = 1 << 20

func (s *IconFetcher) fetch(ctx context.Context, rawURL string) (body []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetchFailed, err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetchFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	return readBody(resp.Body)
}

func checkStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest, http.StatusNotFound, http.StatusGone:
		return fmt.Errorf("%w: status %s", ErrNotFound, resp.Status)
	}
	return fmt.Errorf("%w: status %s", ErrFetchFailed, resp.Status)
}

func readBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %w", ErrFetchFailed, err)
	}
	switch {
	case len(body) == 0:
		return nil, fmt.Errorf("%w: empty body", ErrInvalid)
	case len(body) > maxBodySize:
		return nil, fmt.Errorf("%w: body over %d bytes", ErrInvalid, maxBodySize)
	}
	return body, nil
}
