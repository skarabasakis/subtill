package icon

import "errors"

var (
	ErrNotFound         = errors.New("icon not found")
	ErrFetchFailed      = errors.New("icon fetch failed")
	ErrInvalid          = errors.New("invalid icon image")
	ErrProcessingFailed = errors.New("icon processing failed")
)
