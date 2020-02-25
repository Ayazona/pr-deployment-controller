package builder

import "errors"

const (
	// WorkerPoolSize defines the number of concurrent build workers
	WorkerPoolSize = 4
)

var (
	// ErrWorkerClosed Error
	ErrWorkerClosed = errors.New("worker closed, cannot accept new jobs")
	// ErrNoDockerfileFound Error
	ErrNoDockerfileFound = errors.New("no dockerfile found in repository")

	// ErrJobOutdated Error
	ErrJobOutdated = errors.New("job ID outdated")
	// ErrJobIgnored Error
	ErrJobIgnored = errors.New("job ignored")

	// CommentTemplate contains the template used to render the environment information
	// nolint: lll
	CommentTemplate = `☁️ Find your changes in the cloud! ☁️

{{if .OnDemand}}<b>We don't deploy this build automatically, comment ` + "`/rebuild`" + ` to deploy this branch to the test environment.</b>{{end}}

- Environment URL: https://{{.BuildURL}}
- Logs: [https://{{.LoggingURLReadable}}](https://{{.LoggingURL}})
{{.Extra}}

---

<details>
<summary>test-environment commands</summary>
<br />

You can trigger test-environment actions by commenting on this PR:
- ` + "`/rebuild`" + ` will issue a new deployment to the test-environment based on the latest commit.
- ` + "`/clean`" + ` will remove the current test-environment build if exists and issue a new deployment based on the latest commit.
</details>
`
)
