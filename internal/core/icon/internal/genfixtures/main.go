// Command genfixtures writes the image fixtures of the icon package tests to
// testdata/. Run it with go generate from internal/core/icon.
//
//nolint:gosec // fixture generator: sizes are small constants
package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"os"
	"path/filepath"
)

var (
	red    = color.NRGBA{R: 255, A: 255}
	green  = color.NRGBA{G: 255, A: 255}
	blue   = color.NRGBA{B: 255, A: 255}
	yellow = color.NRGBA{R: 255, G: 255, A: 255}
)

func main() {
	fixtures := map[string][]byte{
		"normal.png":       encodePNG(gradient(32, 32, func(x, _ int) uint8 { return uint8(128 + x*4) })),
		"nonsquare.png":    encodePNG(uniform(128, 32, red)),
		"deep16.png":       encodePNG(deep(64, 64)),
		"opaque64.png":     encodePNG(gradient(64, 64, opaque)),
		"image.gif":        encodeGIF(),
		"wide513.png":      encodePNG(uniform(513, 1, red)),
		"tall513.png":      encodePNG(uniform(1, 513, red)),
		"truncated.png":    truncated(),
		"corrupt-crc.png":  corruptCRC(),
		"corrupt-data.png": corruptData(),
		"trailing.png":     append(encodePNG(gradient(64, 64, opaque)), "trailing bytes"...),
		"hugeheader.png":   hugeHeader(),
		"icon.ico": encodeICO(
			pngEntry(uniform(16, 16, red)), pngEntry(uniform(256, 256, yellow)),
			bmpEntry(16), pngEntry(uniform(128, 128, blue)), pngEntry(uniform(32, 32, green))),
		"icon-small.ico":        encodeICO(pngEntry(uniform(16, 16, red)), pngEntry(uniform(32, 32, green))),
		"icon-broken-entry.ico": encodeICO(brokenPNGEntry(64), pngEntry(uniform(32, 32, green))),
		"icon-too-large.ico":    encodeICO(pngEntry(uniform(513, 513, blue))),
		"icon-bmp-only.ico":     encodeICO(bmpEntry(16), bmpEntry(32)),
		"icon-no-usable.ico": encodeICO(
			icoImage{width: 64, height: 64, payload: []byte("not an image")}, brokenPNGEntry(64)),
	}
	for name, data := range extraChunkFixtures() {
		fixtures[name] = data
	}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join("testdata", name), data, 0o644); err != nil {
			panic(err)
		}
	}
}

func extraChunkFixtures() map[string][]byte {
	extra := []chunk{
		{"tEXt", []byte("Author\x00Someone")},
		{"iTXt", []byte("Comment\x00\x00\x00en\x00\x00hello")},
		{"zTXt", append([]byte("Software\x00\x00"), deflate([]byte("generator"))...)},
		{"eXIf", []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x00")},
		{"iCCP", append([]byte("profile\x00\x00"), deflate(bytes.Repeat([]byte("icc"), 2000))...)},
		{"prVt", bytes.Repeat([]byte("private"), 200)},
	}
	base := encodePNG(gradient(64, 64, opaque))
	out := map[string][]byte{"extrachunks.png": withChunksAfterIHDR(base, extra...)}
	for _, c := range extra {
		out["extrachunk-"+c.typ+".png"] = withChunksAfterIHDR(base, c)
	}
	return out
}

func encodeGIF() []byte {
	img := image.NewPaletted(image.Rect(0, 0, 16, 16), palette.Plan9)
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func truncated() []byte {
	data := encodePNG(gradient(64, 64, opaque))
	idat := chunkOffset(data, "IDAT")
	n := int(binary.BigEndian.Uint32(data[idat:]))
	return data[:idat+8+n/2]
}

func corruptCRC() []byte {
	data := encodePNG(gradient(64, 64, opaque))
	idat := chunkOffset(data, "IDAT")
	n := int(binary.BigEndian.Uint32(data[idat:]))
	data[idat+8+n] ^= 0xff
	return data
}

// corruptData garbles the compressed IDAT data but keeps a valid CRC, so
// only decompression detects it.
func corruptData() []byte {
	data := encodePNG(gradient(64, 64, opaque))
	cs := chunks(data)
	for i, c := range cs {
		if c.typ == "IDAT" {
			garbled := append([]byte{}, c.data...)
			for j := 2; j < len(garbled); j++ {
				garbled[j] ^= 0xa5
			}
			cs[i].data = garbled
		}
	}
	return assemble(cs)
}

func hugeHeader() []byte {
	cs := chunks(encodePNG(uniform(4, 4, red)))
	ihdr := append([]byte{}, cs[0].data...)
	binary.BigEndian.PutUint32(ihdr[0:], 100000)
	binary.BigEndian.PutUint32(ihdr[4:], 100000)
	cs[0].data = ihdr
	return assemble(cs)
}

func deflate(data []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}
