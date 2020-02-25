package fetcher

import "errors"

var (
	// ErrUnknownFetcher Error
	ErrUnknownFetcher = errors.New("unknown dump fetcher")
)
