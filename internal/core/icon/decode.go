package icon

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
)

const maxSourceSide = 512

var (
	pngSignature = []byte("\x89PNG\r\n\x1a\n")
	icoSignature = []byte{0, 0, 1, 0}
)

type imageFormat struct {
	name         string
	decodeConfig func(body []byte) (image.Config, error)
	decode       func(body []byte) (image.Image, error)
}

var (
	pngFormat = imageFormat{name: "png", decodeConfig: pngDecodeConfig, decode: pngDecode}
	icoFormat = imageFormat{name: "ico", decodeConfig: icoDecodeConfig, decode: icoDecode}
)

func decode(body []byte) (image.Image, error) {
	format, err := detectedFormat(body)
	if err != nil {
		return nil, err
	}
	cfg, err := format.decodeConfig(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s header: %w", ErrInvalid, format.name, err)
	}
	if err := checkSourceDimensions(cfg); err != nil {
		return nil, err
	}
	img, err := format.decode(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalid, format.name, err)
	}
	return img, nil
}

func detectedFormat(body []byte) (imageFormat, error) {
	switch {
	case bytes.HasPrefix(body, pngSignature):
		return pngFormat, nil
	case bytes.HasPrefix(body, icoSignature):
		return icoFormat, nil
	}
	return imageFormat{}, fmt.Errorf("%w: unsupported image format", ErrInvalid)
}

func checkSourceDimensions(cfg image.Config) error {
	if cfg.Width > maxSourceSide || cfg.Height > maxSourceSide {
		return fmt.Errorf("%w: %d×%d exceeds %d pixels", ErrInvalid, cfg.Width, cfg.Height, maxSourceSide)
	}
	return nil
}

func pngDecodeConfig(body []byte) (image.Config, error) {
	return png.DecodeConfig(bytes.NewReader(body))
}

func pngDecode(body []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(body))
}
