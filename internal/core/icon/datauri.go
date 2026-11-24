package icon

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const dataURIPrefix = "data:image/png;base64,"

func toDataURI(png []byte) string {
	return dataURIPrefix + base64.StdEncoding.EncodeToString(png)
}

func iconPNG(dataURI string) ([]byte, error) {
	payload, ok := strings.CutPrefix(dataURI, dataURIPrefix)
	if !ok {
		return nil, fmt.Errorf("%w: not a %s data URI", ErrInvalid, strings.TrimSuffix(dataURIPrefix, ","))
	}
	png, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return png, nil
}
