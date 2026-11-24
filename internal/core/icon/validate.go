package icon

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image/png"
)

const maxIconPNGSize = 36 << 10

var maxIconDataURILength = len(dataURIPrefix) + base64.StdEncoding.EncodedLen(maxIconPNGSize)

var allowedChunks = map[string]bool{"IHDR": true, "PLTE": true, "tRNS": true, "IDAT": true, "IEND": true}

// Validate returns ErrInvalid unless dataURI is a 64×64 PNG data URI with
// only the pixel chunks, the shape FromDomain and FromImageURL produce.
func Validate(dataURI string) error {
	if len(dataURI) > maxIconDataURILength {
		return fmt.Errorf("%w: data URI longer than %d bytes", ErrInvalid, maxIconDataURILength)
	}
	data, err := iconPNG(dataURI)
	if err != nil {
		return err
	}
	if len(data) > maxIconPNGSize {
		return fmt.Errorf("%w: PNG larger than %d bytes", ErrInvalid, maxIconPNGSize)
	}
	if err := checkPixelChunksOnly(data); err != nil {
		return err
	}
	if err := checkIconDimensions(data); err != nil {
		return err
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return nil
}

// checkPixelChunksOnly walks the chunks of data and returns ErrInvalid on a
// chunk other than the pixel chunks, or on bytes after IEND.
func checkPixelChunksOnly(data []byte) error {
	if !bytes.HasPrefix(data, pngSignature) {
		return fmt.Errorf("%w: not a PNG", ErrInvalid)
	}
	rest := data[len(pngSignature):]
	for len(rest) > 0 {
		chunkType, chunkLen, err := chunkHeader(rest)
		if err != nil {
			return err
		}
		if !allowedChunks[chunkType] {
			return fmt.Errorf("%w: unexpected %q chunk", ErrInvalid, chunkType)
		}
		rest = rest[chunkLen:]
		if chunkType == "IEND" {
			return checkNoTrailingBytes(rest)
		}
	}
	return fmt.Errorf("%w: missing IEND chunk", ErrInvalid)
}

// chunkHeader returns the type of the chunk at the start of data and its
// total length, including the length, type and CRC fields.
func chunkHeader(data []byte) (chunkType string, total int, err error) {
	if len(data) < 12 {
		return "", 0, fmt.Errorf("%w: truncated chunk", ErrInvalid)
	}
	dataLen := int(binary.BigEndian.Uint32(data[:4]))
	if dataLen > len(data)-12 {
		return "", 0, fmt.Errorf("%w: truncated chunk", ErrInvalid)
	}
	return string(data[4:8]), dataLen + 12, nil
}

func checkNoTrailingBytes(rest []byte) error {
	if len(rest) > 0 {
		return fmt.Errorf("%w: %d bytes after IEND", ErrInvalid, len(rest))
	}
	return nil
}

func checkIconDimensions(data []byte) error {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if cfg.Width != iconSide || cfg.Height != iconSide {
		return fmt.Errorf("%w: %d×%d, want %d×%d", ErrInvalid, cfg.Width, cfg.Height, iconSide, iconSide)
	}
	return nil
}
