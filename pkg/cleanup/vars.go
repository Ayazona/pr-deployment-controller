package cleanup

import "time"

var (
	// IterationDelay defines the delay between each cleanup run
	IterationDelay = 10 * time.Minute
	// EnvironmentLifetime defines the liftetime of a build without new changes
	EnvironmentLifetime = 48 * time.Hour
)
