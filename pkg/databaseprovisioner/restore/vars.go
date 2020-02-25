package restore

import (
	"errors"
	"time"
)

var (
	// waitdeadline contains the deadline for waiting in a database to become ready
	waitdeadline = 5 * time.Minute

	// ErrDatabaseUnavailable Error returned when the database is unavailable after the wait deadline
	ErrDatabaseUnavailable = errors.New("database unavailable, deadline reached")
)
