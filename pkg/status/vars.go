package status

import "errors"

var (
	// ErrMissingVar Error
	ErrMissingVar = errors.New("missing url var")
	// ErrUnknownCommand Error
	ErrUnknownCommand = errors.New("unknown remote command")
	// ErrNoPodFound Error
	ErrNoPodFound = errors.New("no pod found")
)
