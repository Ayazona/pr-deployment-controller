package builder

import "time"

// nolint: maligned
type job struct {
	id                int64
	owner             string
	repository        string
	deleteEnvironment bool

	pullRequestNumber int64
	ref               string
	user              string
	firstRun          bool
	clean             bool
	force             bool

	createTime *time.Time
	startTime  *time.Time
}

type jobResult struct {
	job *job
	err error
}
