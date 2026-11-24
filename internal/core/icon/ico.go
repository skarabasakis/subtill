package icon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"sort"
)

const (
	icoHeaderSize = 6
	icoEntrySize  = 16
	preferredSide = 64
)

var errNoUsableICOEntry = errors.New("no PNG entry of 512×512 or smaller")

type icoEntry struct {
	png []byte
	cfg image.Config
}

func (e icoEntry) longerSide() int {
	return max(e.cfg.Width, e.cfg.Height)
}

func icoDecodeConfig(body []byte) (image.Config, error) {
	entries, err := icoCandidates(body)
	if err != nil {
		return image.Config{}, err
	}
	return entries[0].cfg, nil
}

// icoDecode decodes the preferred candidate, falling back to the next ones
// when an entry turns out to be broken past its header.
func icoDecode(body []byte) (image.Image, error) {
	entries, err := icoCandidates(body)
	if err != nil {
		return nil, err
	}
	var errs []error
	for _, entry := range entries {
		img, err := png.Decode(bytes.NewReader(entry.png))
		if err == nil {
			return img, nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

// icoCandidates returns the usable PNG entries of an ICO, most preferred
// first.
func icoCandidates(body []byte) ([]icoEntry, error) {
	payloads, err := icoPayloads(body)
	if err != nil {
		return nil, err
	}
	entries := usablePNGEntries(payloads)
	if len(entries) == 0 {
		return nil, errNoUsableICOEntry
	}
	sortByPreference(entries)
	return entries, nil
}

func icoPayloads(body []byte) ([][]byte, error) {
	if len(body) < icoHeaderSize {
		return nil, errors.New("truncated ICO header")
	}
	count := int(binary.LittleEndian.Uint16(body[4:6]))
	payloads := make([][]byte, 0, count)
	for i := range count {
		start := icoHeaderSize + i*icoEntrySize
		if start+icoEntrySize > len(body) {
			return nil, errors.New("truncated ICO directory")
		}
		if payload, ok := icoPayload(body, body[start:start+icoEntrySize]); ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads, nil
}

func icoPayload(body, dirEntry []byte) ([]byte, bool) {
	size := uint64(binary.LittleEndian.Uint32(dirEntry[8:12]))
	offset := uint64(binary.LittleEndian.Uint32(dirEntry[12:16]))
	if size == 0 || offset+size > uint64(len(body)) {
		return nil, false
	}
	return body[offset : offset+size], true
}

func usablePNGEntries(payloads [][]byte) []icoEntry {
	var entries []icoEntry
	for _, payload := range payloads {
		if !bytes.HasPrefix(payload, pngSignature) {
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(payload))
		if err != nil || checkSourceDimensions(cfg) != nil {
			continue
		}
		entries = append(entries, icoEntry{png: payload, cfg: cfg})
	}
	return entries
}

// sortByPreference puts entries of at least 64 pixels first, smallest
// first, then the smaller ones, largest first.
func sortByPreference(entries []icoEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].longerSide(), entries[j].longerSide()
		aBig, bBig := a >= preferredSide, b >= preferredSide
		if aBig != bBig {
			return aBig
		}
		if aBig {
			return a < b
		}
		return a > b
	})
}
