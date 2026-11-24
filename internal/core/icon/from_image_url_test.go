package icon_test

import (
	"bytes"
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/skarabasakis/subtill/internal/core/icon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const imageURL = "https://icons.example/favicon"

func TestFromImageURL(t *testing.T) {
	ctx := context.Background()

	statuses := []struct {
		status int
		want   error
	}{
		{http.StatusBadRequest, icon.ErrNotFound},
		{http.StatusNotFound, icon.ErrNotFound},
		{http.StatusGone, icon.ErrNotFound},
		{http.StatusNoContent, icon.ErrFetchFailed},
		{http.StatusFound, icon.ErrFetchFailed},
		{http.StatusUnauthorized, icon.ErrFetchFailed},
		{http.StatusForbidden, icon.ErrFetchFailed},
		{http.StatusTooManyRequests, icon.ErrFetchFailed},
		{http.StatusInternalServerError, icon.ErrFetchFailed},
		{http.StatusBadGateway, icon.ErrFetchFailed},
	}
	for _, tc := range statuses {
		t.Run("returns "+tc.want.Error()+" when server answers "+http.StatusText(tc.status), func(t *testing.T) {
			fetcher, _ := fetcherServing(tc.status, fixture(t, "opaque64.png"))

			got, err := fetcher.FromImageURL(ctx, imageURL)

			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, got)
		})
	}

	t.Run("requests given URL when server answers OK", func(t *testing.T) {
		fetcher, transport := fetcherServing(http.StatusOK, fixture(t, "opaque64.png"))

		got, err := fetcher.FromImageURL(ctx, imageURL)

		require.NoError(t, err)
		requireValidIcon(t, got)
		assert.Equal(t, []string{imageURL}, transport.requestedURLs())
	})

	t.Run("returns ErrInvalid when body is empty", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, nil)

		_, err := fetcher.FromImageURL(ctx, imageURL)

		assert.ErrorIs(t, err, icon.ErrInvalid)
	})

	t.Run("returns ErrInvalid when body exceeds 1 MiB", func(t *testing.T) {
		body := append(fixture(t, "opaque64.png"), make([]byte, 1<<20)...)
		fetcher, _ := fetcherServing(http.StatusOK, body)

		_, err := fetcher.FromImageURL(ctx, imageURL)

		assert.ErrorIs(t, err, icon.ErrInvalid)
	})

	t.Run("returns ErrFetchFailed when body read fails midway", func(t *testing.T) {
		fetcher, transport := fetcherServing(http.StatusOK, fixture(t, "opaque64.png")[:40])
		transport.readErr = true

		_, err := fetcher.FromImageURL(ctx, imageURL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
		assert.ErrorIs(t, err, errMidRead)
	})

	accepted := []string{"normal.png", "nonsquare.png", "deep16.png", "opaque64.png",
		"icon.ico", "icon-small.ico", "icon-broken-entry.ico"}
	for _, name := range accepted {
		t.Run("returns valid 64x64 icon when body is "+name, func(t *testing.T) {
			fetcher, _ := fetcherServing(http.StatusOK, fixture(t, name))

			got, err := fetcher.FromImageURL(ctx, imageURL)

			require.NoError(t, err)
			decodeIcon(t, got)
		})
	}

	rejected := []string{"image.gif", "wide513.png", "tall513.png", "truncated.png",
		"corrupt-crc.png", "corrupt-data.png", "hugeheader.png",
		"icon-too-large.ico", "icon-bmp-only.ico", "icon-no-usable.ico"}
	for _, name := range rejected {
		t.Run("returns ErrInvalid when body is "+name, func(t *testing.T) {
			fetcher, _ := fetcherServing(http.StatusOK, fixture(t, name))

			got, err := fetcher.FromImageURL(ctx, imageURL)

			assert.ErrorIs(t, err, icon.ErrInvalid)
			assert.Empty(t, got)
		})
	}

	t.Run("returns ErrInvalid when body is HTML page", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, []byte("<!doctype html><title>Down</title>"))

		_, err := fetcher.FromImageURL(ctx, imageURL)

		assert.ErrorIs(t, err, icon.ErrInvalid)
	})

	icoSelections := []struct {
		name    string
		fixture string
		want    color.NRGBA
	}{
		{"picks smallest entry of at least 64 when ICO has several", "icon.ico",
			color.NRGBA{B: 255, A: 255}},
		{"picks largest entry below 64 when ICO has only small ones", "icon-small.ico",
			color.NRGBA{G: 255, A: 255}},
		{"skips broken entry when ICO has another usable one", "icon-broken-entry.ico",
			color.NRGBA{G: 255, A: 255}},
	}
	for _, tc := range icoSelections {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, _ := fetcherServing(http.StatusOK, fixture(t, tc.fixture))

			got, err := fetcher.FromImageURL(ctx, imageURL)

			require.NoError(t, err)
			assert.Equal(t, tc.want, nrgbaAt(decodeIcon(t, got), 32, 32))
		})
	}

	t.Run("pads with transparency without cropping when image is non-square", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, fixture(t, "nonsquare.png"))

		got, err := fetcher.FromImageURL(ctx, imageURL)

		require.NoError(t, err)
		img := decodeIcon(t, got)
		assertTransparentRows(t, img, 0, 24)
		assertTransparentRows(t, img, 40, 64)
		red := color.NRGBA{R: 255, A: 255}
		for _, x := range []int{0, 32, 63} {
			assert.Equal(t, red, nrgbaAt(img, x, 32), "pixel (%d,32)", x)
		}
	})

	t.Run("returns 8-bit icon passing Validate when image is 16-bit", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, fixture(t, "deep16.png"))

		got, err := fetcher.FromImageURL(ctx, imageURL)

		require.NoError(t, err)
		data := requireValidIcon(t, got)
		const bitDepthOffset = 8 + 8 + 8
		assert.Equal(t, byte(8), data[bitDepthOffset])
	})

	t.Run("strips extra chunks keeping pixels when PNG carries metadata", func(t *testing.T) {
		input := fixture(t, "extrachunks.png")
		fetcher, _ := fetcherServing(http.StatusOK, input)

		got, err := fetcher.FromImageURL(ctx, imageURL)

		require.NoError(t, err)
		data := requireValidIcon(t, got)
		assert.Equal(t, []string{"IHDR", "IDAT", "IEND"}, chunkTypes(t, data))
		assert.Less(t, len(data), len(input))
		assertSamePixels(t, fixturePNG(t, "extrachunks.png"), decodeIcon(t, got))
		assert.False(t, bytes.Contains(data, []byte("Someone")))
	})

	t.Run("returns ErrFetchFailed when server drops connection mid-body", func(t *testing.T) {
		server := serve(t, func(w http.ResponseWriter, _ *http.Request) {
			conn, buf, err := http.NewResponseController(w).Hijack()
			if err != nil {
				return
			}
			_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n\x89PNG\r\n\x1a\n")
			_ = buf.Flush()
			_ = conn.Close()
		})

		_, err := icon.NewIconFetcher().FromImageURL(ctx, server.URL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
	})

	t.Run("returns ErrFetchFailed when server redirects more than 10 times", func(t *testing.T) {
		server := serve(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/again", http.StatusFound)
		})

		_, err := icon.NewIconFetcher().FromImageURL(ctx, server.URL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
	})

	t.Run("returns ErrFetchFailed when server redirects to non-http scheme", func(t *testing.T) {
		server := serve(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "ftp://icons.example/favicon.ico", http.StatusFound)
		})

		_, err := icon.NewIconFetcher().FromImageURL(ctx, server.URL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
	})

	t.Run("returns ErrFetchFailed with DeadlineExceeded when server never answers", func(t *testing.T) {
		server := serveNever(t)
		short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		_, err := icon.NewIconFetcher().FromImageURL(short, server.URL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("returns ErrFetchFailed with Canceled when ctx is cancelled", func(t *testing.T) {
		server := serveNever(t)
		cancelled, cancel := context.WithCancel(ctx)
		time.AfterFunc(20*time.Millisecond, cancel)

		_, err := icon.NewIconFetcher().FromImageURL(cancelled, server.URL)

		assert.ErrorIs(t, err, icon.ErrFetchFailed)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("returns valid icon when package-level function downloads from server", func(t *testing.T) {
		body := fixture(t, "normal.png")
		server := serve(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(body)
		})

		got, err := icon.FromImageURL(ctx, server.URL+"/favicon.png")

		require.NoError(t, err)
		decodeIcon(t, got)
	})
}

func serve(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

// serveNever starts a server whose handler never answers. The handler is
// released before the server closes, so the test doesn't wait on it.
func serveNever(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	server := serve(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })
	return server
}
