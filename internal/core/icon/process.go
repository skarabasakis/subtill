package icon

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

const iconSide = 64

func process(img image.Image) ([]byte, error) {
	canvas := image.NewNRGBA(image.Rect(0, 0, iconSide, iconSide))
	draw.CatmullRom.Scale(canvas, fittedRect(img.Bounds().Size(), iconSide), img, img.Bounds(), draw.Src, nil)
	return encodePNG(canvas)
}

// fittedRect returns the largest rectangle with the aspect ratio of src that
// fits in a side×side square, centred in it.
func fittedRect(src image.Point, side int) image.Rectangle {
	w, h := side, side
	switch {
	case src.X > src.Y:
		h = max(1, (src.Y*side+src.X/2)/src.X)
	case src.Y > src.X:
		w = max(1, (src.X*side+src.Y/2)/src.Y)
	}
	x, y := (side-w)/2, (side-h)/2
	return image.Rect(x, y, x+w, y+h)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrProcessingFailed, err)
	}
	return buf.Bytes(), nil
}
