//nolint:gosec // fixture generator: sizes are small constants
package main

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

type chunk struct {
	typ  string
	data []byte
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func chunks(data []byte) []chunk {
	var out []chunk
	rest := data[len(pngSignature):]
	for len(rest) >= 12 {
		n := binary.BigEndian.Uint32(rest[:4])
		out = append(out, chunk{typ: string(rest[4:8]), data: rest[8 : 8+n]})
		rest = rest[12+n:]
	}
	return out
}

func assemble(cs []chunk) []byte {
	buf := bytes.NewBuffer(append([]byte{}, pngSignature...))
	for _, c := range cs {
		buf.Write(chunkBytes(c))
	}
	return buf.Bytes()
}

func chunkBytes(c chunk) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(c.data)))
	out = append(out, c.typ...)
	out = append(out, c.data...)
	crc := crc32.ChecksumIEEE(append([]byte(c.typ), c.data...))
	return binary.BigEndian.AppendUint32(out, crc)
}

// withChunksAfterIHDR inserts extra after the IHDR chunk of data.
func withChunksAfterIHDR(data []byte, extra ...chunk) []byte {
	cs := chunks(data)
	out := append([]chunk{cs[0]}, extra...)
	return assemble(append(out, cs[1:]...))
}

func chunkOffset(data []byte, typ string) int {
	off := len(pngSignature)
	for off+8 <= len(data) {
		n := int(binary.BigEndian.Uint32(data[off : off+4]))
		if string(data[off+4:off+8]) == typ {
			return off
		}
		off += 12 + n
	}
	panic("chunk not found: " + typ)
}

func uniform(w, h int, c color.Color) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func gradient(w, h int, alpha func(x, y int) uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: alpha(x, y)})
		}
	}
	return img
}

func opaque(int, int) uint8 { return 255 }

func deep(w, h int) image.Image {
	img := image.NewRGBA64(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA64(x, y, color.RGBA64{R: uint16(x * 65535 / w), G: uint16(y * 65535 / h), B: 0x1234, A: 0xffff})
		}
	}
	return img
}
