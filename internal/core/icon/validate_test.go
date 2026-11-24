package icon_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/skarabasakis/subtill/internal/core/icon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	ctx := context.Background()

	t.Run("accepts icon when it comes from FromImageURL", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, fixture(t, "extrachunks.png"))
		got, err := fetcher.FromImageURL(ctx, imageURL)
		require.NoError(t, err)

		err = icon.Validate(got)

		assert.NoError(t, err)
	})

	t.Run("accepts icon when it comes from FromDomain", func(t *testing.T) {
		fetcher, _ := fetcherServing(http.StatusOK, fixture(t, "normal.png"))
		got, err := fetcher.FromDomain(ctx, "example.com")
		require.NoError(t, err)

		err = icon.Validate(got)

		assert.NoError(t, err)
	})

	rejectedFixtures := []struct {
		name    string
		fixture string
	}{
		{"rejects PNG when it isn't 64x64", "normal.png"},
		{"rejects PNG when it carries tEXt chunk", "extrachunk-tEXt.png"},
		{"rejects PNG when it carries iTXt chunk", "extrachunk-iTXt.png"},
		{"rejects PNG when it carries zTXt chunk", "extrachunk-zTXt.png"},
		{"rejects PNG when it carries eXIf chunk", "extrachunk-eXIf.png"},
		{"rejects PNG when it carries iCCP chunk", "extrachunk-iCCP.png"},
		{"rejects PNG when it carries private ancillary chunk", "extrachunk-prVt.png"},
		{"rejects PNG when it is truncated mid-IDAT", "truncated.png"},
		{"rejects PNG when IDAT CRC is wrong", "corrupt-crc.png"},
		{"rejects PNG when IDAT data is garbled", "corrupt-data.png"},
		{"rejects PNG when bytes follow IEND", "trailing.png"},
		{"rejects PNG when header claims huge dimensions", "hugeheader.png"},
		{"rejects data URI when payload is GIF", "image.gif"},
	}
	for _, tc := range rejectedFixtures {
		t.Run(tc.name, func(t *testing.T) {
			input := dataURI(fixture(t, tc.fixture))

			err := icon.Validate(input)

			assert.ErrorIs(t, err, icon.ErrInvalid)
		})
	}

	t.Run("rejects PNG when it exceeds 36 KiB", func(t *testing.T) {
		input := dataURI(append(fixture(t, "opaque64.png"), make([]byte, 36<<10)...))

		err := icon.Validate(input)

		assert.ErrorIs(t, err, icon.ErrInvalid)
	})

	t.Run("rejects data URI when it exceeds length cap", func(t *testing.T) {
		input := iconPrefix + strings.Repeat("A", base64.StdEncoding.EncodedLen(36<<10)+4)

		err := icon.Validate(input)

		assert.ErrorIs(t, err, icon.ErrInvalid)
	})

	rejectedStrings := []struct {
		name  string
		input string
	}{
		{"rejects input when it is empty", ""},
		{"rejects input when it is URL", "https://example.com/favicon.png"},
		{"rejects data URI when media type is JPEG", "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("jpeg"))},
		{"rejects data URI when it is prefix only", iconPrefix},
		{"rejects data URI when base64 is invalid", iconPrefix + "not*base64!"},
	}
	for _, tc := range rejectedStrings {
		t.Run(tc.name, func(t *testing.T) {
			err := icon.Validate(tc.input)

			assert.ErrorIs(t, err, icon.ErrInvalid)
		})
	}
}
