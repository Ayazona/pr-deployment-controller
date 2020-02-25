package builder

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"
	"time"

	testenvironmentv1alpha1 "github.com/kolonialno/test-environment-manager/pkg/apis/testenvironment/v1alpha1"
	"github.com/kolonialno/test-environment-manager/pkg/github"
	"github.com/kolonialno/test-environment-manager/pkg/internal"
)

//
// Github interaction
//

// commentEnvironmentInformation comments the environment information on the pull request
func (w *worker) commentEnvironmentInformation(
	ctx context.Context, j *job, environment *testenvironmentv1alpha1.Environment,
) error {
	buildURL := internal.GenerateBuildURL(
		j.owner, j.repository, j.pullRequestNumber, w.options.ClusterDomain,
	)

	var extra []string

	// Remote teminal entrypoints
	for _, container := range environment.Spec.Containers {
		for _, terminal := range container.RemoteTerminal {
			extra = append(extra, fmt.Sprintf(
				"- %s %s [Click here](https://%s/term/%s-%s-%d/%s/%s/)",
				container.Name,
				terminal.Name,
				buildURL,
				j.owner,
				j.repository,
				j.pullRequestNumber,
				container.Name,
				terminal.Name,
			))
		}
	}

	// Environment links (Supports template values)
	type props struct {
		BuildURL string
	}
	p := props{
		BuildURL: buildURL,
	}
	for _, link := range environment.Spec.Links {
		tmpl, err := template.New("link").Parse(link.URL)
		if err != nil {
			return err
		}

		buff := bytes.NewBufferString("")

		err = tmpl.Execute(buff, p)
		if err != nil {
			return err
		}

		var value []byte

		value, err = ioutil.ReadAll(buff)
		if err != nil {
			return err
		}

		extra = append(extra, fmt.Sprintf(
			"- %s: %s",
			link.Title,
			string(value),
		))
	}

	tmpl, err := template.New("comment").Parse(CommentTemplate)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"OnDemand":           environment.Spec.OnDemand,
		"BuildURL":           buildURL,
		"LoggingURLReadable": fmt.Sprintf("kibana.%s", w.options.ClusterDomain),
		"LoggingURL": internal.GenerateLogsURL(
			w.options.BuildPrefix,
			j.owner,
			j.repository,
			j.pullRequestNumber,
			fmt.Sprintf("kibana.%s", w.options.ClusterDomain),
		),
		"Extra": strings.Join(extra, "\n"),
	}

	var commentBuffer bytes.Buffer

	if err = tmpl.Execute(&commentBuffer, data); err != nil {
		return err
	}

	return w.options.GitHub.PRComment(
		ctx,
		j.owner,
		j.repository,
		j.pullRequestNumber,
		commentBuffer.String(),
	)
}

// updateBuildStatus updates the build status based on the job value
func (w *worker) updateBuildStatus(
	ctx context.Context, j *job, state github.State, description, url string,
) error {
	return w.options.GitHub.PostBuildStatus(
		ctx,
		j.owner,
		j.repository,
		j.ref,
		state,
		description,
		url,
	)
}

//
// Docker interaction
//

// processBuildContext removes the root folder inside the repository
// archive (place repository files in the archive root)
// nolint: gocyclo
func (w *worker) processBuildContext(
	j *job,
	dockerFile string,
	r io.Reader,
) (io.Reader, error) {
	// Read gzip compressed file
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close() // nolint: gas, errcheck

	// Read tar archive
	tarReader := tar.NewReader(gzipReader)

	// Initialize resulting tar buffer
	resultBuffer := bytes.NewBuffer([]byte{})
	resultWriter := tar.NewWriter(resultBuffer)
	defer resultWriter.Close() // nolint: gas, errcheck

	baseFolder := fmt.Sprintf("%s-%s-%s/", j.owner, j.repository, j.ref)
	dockerFileFound := false

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if header == nil {
			break
		}

		// Skip the first folder (root folder inside the archive)
		if header.Name == baseFolder {
			continue
		}

		// Write to new tar (files at root level)
		header.Name = strings.TrimPrefix(header.Name, baseFolder)
		if header.Name == dockerFile {
			dockerFileFound = true
		}
		if err := resultWriter.WriteHeader(header); err != nil {
			return nil, err
		}

		// Copy content
		if _, err := io.Copy(resultWriter, tarReader); err != nil {
			return nil, err
		}
	}

	if !dockerFileFound {
		return nil, ErrNoDockerfileFound
	}

	return resultBuffer, nil
}

//
// Tracking
//

// trackTask observes a runtime duration with the RuntimeSummary (used to track execution time)
func (w *worker) trackTask(j *job, name string, startTime time.Time) {
	w.options.RuntimeSummary.WithLabelValues(
		j.owner,
		j.repository,
		strconv.FormatInt(j.pullRequestNumber, 10),
		strconv.FormatInt(j.id, 10),
		name,
	).Observe(time.Since(startTime).Seconds())
}
