package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Github defines the interface used to talk with Github.
type Github interface {
	// CloneBuild downloads the build from GitHub
	CloneBuild(
		ctx context.Context,
		owner,
		repository,
		ref string,
	) (io.ReadCloser, error)
	// PostBuildStatus updates the build status
	PostBuildStatus(
		ctx context.Context,
		owner,
		repository,
		ref string,
		state State,
		description,
		url string,
	) error
	// PRComment comments in a PR
	PRComment(
		ctx context.Context,
		owner,
		repository string,
		pullRequestNumber int64,
		comment string,
	) error

	//
	// Low-level access apis
	//

	// Get exposes a generic http get function
	Get(ctx context.Context, url string, body interface{}, v interface{}) (*http.Response, error)
}

type baseGithub struct {
	logger         *logrus.Entry
	http           *http.Client
	c              *github.Client
	downloadClient *http.Client
}

// New creates a new Github controller
func New(logger *logrus.Entry, accessToken string) (Github, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accessToken},
	)

	// Base http client with authentication
	httpClient := oauth2.NewClient(context.Background(), ts)
	httpClient.Timeout = Timeout

	c := github.NewClient(httpClient)

	return &baseGithub{
		logger: logger,
		http:   httpClient,
		c:      c,
		downloadClient: &http.Client{
			Timeout: Timeout,
		},
	}, nil
}

// CloneBuild fetches the archive url from the Github API and downloads the archive
func (g *baseGithub) CloneBuild(
	ctx context.Context,
	owner string,
	repository string,
	ref string,
) (io.ReadCloser, error) {
	url, _, err := g.c.Repositories.GetArchiveLink(
		ctx,
		owner,
		repository,
		github.Tarball,
		&github.RepositoryContentGetOptions{
			Ref: ref,
		},
	)
	if err != nil {
		return nil, err
	}

	resp, err := g.downloadClient.Do(&http.Request{ // nolint: bodyclose
		Method: "GET",
		URL:    url,
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// PostBuildStatus updates the status on a given commit
func (g *baseGithub) PostBuildStatus(
	ctx context.Context,
	owner string,
	repository string,
	ref string,
	state State,
	description string,
	url string,
) error {
	_, _, err := g.c.Repositories.CreateStatus(
		ctx,
		owner,
		repository,
		ref,
		&github.RepoStatus{
			State:       github.String(state.String()),
			Description: github.String(description),
			TargetURL:   github.String(url),
			Context:     github.String("test-environment"),
		},
	)
	return err
}

// PRComment creates a new comment on a PR
func (g *baseGithub) PRComment(
	ctx context.Context,
	owner,
	repository string,
	pullRequestNumber int64,
	comment string,
) error {
	_, _, err := g.c.Issues.CreateComment(ctx, owner, repository, int(pullRequestNumber), &github.IssueComment{
		Body: github.String(comment),
	})
	return err
}

func (g *baseGithub) Get(ctx context.Context, url string, body interface{}, v interface{}) (*http.Response, error) {
	req, err := g.newRequest("GET", url, body)
	if err != nil {
		return nil, err
	}

	return g.do(ctx, req, v)
}

//
// Internal methods
//

// newRequest creates a new http request
func (g *baseGithub) newRequest(method, url string, body interface{}) (*http.Request, error) {
	var err error

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err = enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// do executes an http request and returns a response
// nolint: gocyclo
func (g *baseGithub) do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req = withContext(ctx, req)

	resp, err := g.http.Do(req)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		return nil, err
	}
	defer resp.Body.Close() // nolint: errcheck

	err = checkResponse(resp)
	if err != nil {
		return resp, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
			if err != nil {
				return nil, err
			}
		} else {
			decErr := json.NewDecoder(resp.Body).Decode(v)
			if decErr == io.EOF {
				decErr = nil // ignore EOF errors caused by empty response body
			}
			if decErr != nil {
				err = decErr
			}
		}
	}

	return resp, err
}

func withContext(ctx context.Context, req *http.Request) *http.Request {
	return req.WithContext(ctx)
}

// checkResponse validates the response and raises an error if necceceary
func checkResponse(resp *http.Response) error {
	// Make sure we receive application/json
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		return fmt.Errorf(
			"content type not application/json; charset=utf-8, actual value %s",
			resp.Header.Get("Content-Type"),
		)
	}

	// Check status code
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return fmt.Errorf("response status not in range [200, 300], actual code %d", resp.StatusCode)
	}

	return nil
}
