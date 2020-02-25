package github

import "time"

const (
	// Timeout stores the timeout used by the github client
	Timeout = 3 * time.Minute
)

// State defines the type that represents different commit status states
type State string

var (
	// PendingState for running jobs
	PendingState State = "pending"
	// SuccessState for success jobs
	SuccessState State = "success"
	// ErrorState for jobs with system failure
	ErrorState State = "error"
	// FailureState for jobs that failed
	FailureState State = "failure"
)

func (s *State) String() string {
	return string(*s)
}
