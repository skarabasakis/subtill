package icon_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/icon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const iconPrefix = "data:image/png;base64,"

var errMidRead = errors.New("connection reset mid-body")

// fakeTransport records every request and answers with a canned response.
type fakeTransport struct {
	status  int
	body    []byte
	readErr bool

	mu       sync.Mutex
	requests []*http.Request
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	var body io.Reader = bytes.NewReader(f.body)
	if f.readErr {
		body = io.MultiReader(body, errReader{})
	}
	return &http.Response{
		StatusCode: f.status,
		Status:     http.StatusText(f.status),
		Body:       io.NopCloser(body),
		Header:     http.Header{},
		Request:    req,
	}, nil
}

func (f *fakeTransport) requestedURLs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	urls := make([]string, len(f.requests))
	for i, req := range f.requests {
		urls[i] = req.URL.String()
	}
	return urls
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errMidRead }

func fetcherServing(status int, body []byte) (*icon.IconFetcher, *fakeTransport) {
	transport := &fakeTransport{status: status, body: body}
	return icon.NewIconFetcher(icon.WithHTTPClient(&http.Client{Transport: transport})), transport
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)
	return data
}

func fixturePNG(t *testing.T, name string) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(fixture(t, name)))
	require.NoError(t, err)
	return img
}

func dataURI(data []byte) string {
	return iconPrefix + base64.StdEncoding.EncodeToString(data)
}

// requireValidIcon checks the shape every returned icon must have and
// returns its PNG bytes.
func requireValidIcon(t *testing.T, got string) []byte {
	t.Helper()
	require.True(t, strings.HasPrefix(got, iconPrefix), "icon %.40q lacks data URI prefix", got)
	require.NoError(t, icon.Validate(got))
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, iconPrefix))
	require.NoError(t, err)
	return data
}

func decodeIcon(t *testing.T, got string) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(requireValidIcon(t, got)))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 64, 64), img.Bounds())
	return img
}

func chunkTypes(t *testing.T, data []byte) []string {
	t.Helper()
	require.True(t, bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")))
	var types []string
	rest := data[8:]
	for len(rest) >= 12 {
		n := binary.BigEndian.Uint32(rest[:4])
		types = append(types, string(rest[4:8]))
		rest = rest[12+n:]
	}
	return types
}

func nrgbaAt(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func assertSamePixels(t *testing.T, want, got image.Image) {
	t.Helper()
	require.Equal(t, want.Bounds(), got.Bounds())
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			if !assert.Equal(t, nrgbaAt(want, x, y), nrgbaAt(got, x, y), "pixel (%d,%d)", x, y) {
				return
			}
		}
	}
}

func assertTransparentRows(t *testing.T, img image.Image, fromY, toY int) {
	t.Helper()
	for y := fromY; y < toY; y++ {
		for x := range 64 {
			if !assert.Zero(t, nrgbaAt(img, x, y).A, "pixel (%d,%d) alpha", x, y) {
				return
			}
		}
	}
}
