//nolint:gosec // fixture generator: sizes are small constants
package main

import (
	"encoding/binary"
	"image"
	"image/color"
)

type icoImage struct {
	width, height int
	payload       []byte
}

func pngEntry(img image.Image) icoImage {
	b := img.Bounds()
	return icoImage{width: b.Dx(), height: b.Dy(), payload: encodePNG(img)}
}

// bmpEntry returns a 32-bit BMP icon entry: a BITMAPINFOHEADER with doubled
// height, the pixels and the AND mask.
func bmpEntry(side int) icoImage {
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], uint32(side))
	binary.LittleEndian.PutUint32(header[8:], uint32(2*side))
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], 32)
	pixels := make([]byte, side*side*4+side*side/8)
	for i := 0; i < side*side*4; i += 4 {
		copy(pixels[i:], []byte{0x20, 0x40, 0xc0, 0xff})
	}
	return icoImage{width: side, height: side, payload: append(header, pixels...)}
}

func brokenPNGEntry(side int) icoImage {
	data := encodePNG(uniform(side, side, color.NRGBA{A: 255}))
	idat := chunkOffset(data, "IDAT")
	for i := idat + 8; i < len(data)-12; i++ {
		data[i] ^= 0x5a
	}
	return icoImage{width: side, height: side, payload: data}
}

func encodeICO(images ...icoImage) []byte {
	out := []byte{0, 0, 1, 0}
	out = binary.LittleEndian.AppendUint16(out, uint16(len(images)))
	offset := 6 + 16*len(images)
	for _, img := range images {
		out = append(out, directoryByte(img.width), directoryByte(img.height), 0, 0)
		out = binary.LittleEndian.AppendUint16(out, 1)
		out = binary.LittleEndian.AppendUint16(out, 32)
		out = binary.LittleEndian.AppendUint32(out, uint32(len(img.payload)))
		out = binary.LittleEndian.AppendUint32(out, uint32(offset))
		offset += len(img.payload)
	}
	for _, img := range images {
		out = append(out, img.payload...)
	}
	return out
}

func directoryByte(side int) byte {
	if side >= 256 {
		return 0
	}
	return byte(side)
}
