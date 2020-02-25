package database

import "errors"

var (
	// ErrOptionsNotConfigured Error
	ErrOptionsNotConfigured = errors.New("call the SetOptions function before initializing the controller")
)
