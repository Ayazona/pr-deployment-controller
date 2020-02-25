package build

import "errors"

var (
	// LabelClaimedBuild defines the label name used to filter databases based on build that claimed the database
	LabelClaimedBuild = "testenvironment.kolonial.no/build"

	// ErrOptionsNotConfigured Error
	ErrOptionsNotConfigured = errors.New("call the SetOptions function before initializing the controller")

	// ErrNoAvailableDatabases Error
	ErrNoAvailableDatabases = errors.New("no available databases based on requested template")
)
