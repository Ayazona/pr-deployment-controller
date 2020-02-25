package docker

import "time"

const (
	// Timeout stores the timeout used by the docker client
	Timeout = 30 * time.Minute
)
