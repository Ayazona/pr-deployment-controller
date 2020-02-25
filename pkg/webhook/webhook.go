package webhook

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kolonialno/test-environment-manager/pkg/builder"
	"github.com/kolonialno/test-environment-manager/pkg/github"
	"github.com/sirupsen/logrus"
)

// Webhook contains the required controllers to handle webhooks
type Webhook struct {
	logger   *logrus.Entry
	b        builder.Builder
	g        github.Github
	secret   string
	username string

	r *mux.Router
}

// New returns a new http handler that is responsible for handling webhooks
func New(
	logger *logrus.Entry,
	b builder.Builder,
	g github.Github,
	githubWebhookSecret string,
	githubUsername string,
) (http.Handler, error) {
	r := mux.NewRouter()

	w := &Webhook{
		logger:   logger,
		b:        b,
		g:        g,
		secret:   githubWebhookSecret,
		username: githubUsername,

		r: r,
	}

	r.HandleFunc("/health", w.healthHandler)
	r.HandleFunc("/webhook", w.webhookHandler)

	return w, nil
}

func (w *Webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.r.ServeHTTP(rw, r)
}

// nolint: gocyclo
func (w *Webhook) webhookHandler(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := w.Parse(r)
	if err != nil {
		w.logger.WithError(err).Warn("could not parese webhook payload")
		errorHandler(rw, err)
		return
	}

	// Generic error handler used if one of the steps returns an error
	handleErr := func(err error) {
		w.logger.WithError(err).Warn("could not handle webhook payload")
		errorHandler(rw, err, http.StatusNotAcceptable)
	}

	switch payload := payload.(type) {
	case PullRequestPayload:

		w.logger.Info("received pull request payload")

		if payload.Action == "opened" ||
			payload.Action == "synchronize" ||
			payload.Action == "reopened" {
			// Create new build on the opened, synchronize (new commit) and reopened action
			if err = w.b.NewBuild(
				ctx,
				payload.PullRequest.Base.Repo.Owner.Login,
				payload.PullRequest.Base.Repo.Name,
				payload.Number,
				payload.PullRequest.Head.Sha,
				payload.Sender.Login,
				payload.Action == "opened",
				false,
				false,
			); err != nil {
				handleErr(err)
				return
			}
		} else if payload.Action == "closed" {
			// Delete build on the closed action (merged included)
			if err = w.b.DeleteBuild(
				ctx,
				payload.PullRequest.Base.Repo.Owner.Login,
				payload.PullRequest.Base.Repo.Name,
				payload.Number,
			); err != nil {
				handleErr(err)
				return
			}
		}

	case IssueCommentPayload:

		w.logger.Info("received issue comment payload")

		if w.username != "" && payload.Sender.Login == w.username {
			w.logger.Info("skipping comment, created by us")
			return
		}

		// Initialize a new build if a user comments "/rebuild" on a PR
		if payload.Action == "created" &&
			payload.Issue.PullRequest != nil &&
			strings.Contains(payload.Comment.Body, "/rebuild") {
			// The issue comment payload doesn't contain the PR head, fetch this from GitHub
			var pullRequest PullRequestResponse
			res, err := w.g.Get(ctx, payload.Issue.PullRequest.URL, nil, &pullRequest)
			if err != nil {
				handleErr(err)
				return
			}
			res.Body.Close() // nolint: errcheck

			if err = w.b.NewBuild(
				ctx,
				pullRequest.Base.Repo.Owner.Login,
				pullRequest.Base.Repo.Name,
				pullRequest.Number,
				pullRequest.Head.Sha,
				payload.Sender.Login,
				false,
				false,
				true,
			); err != nil {
				handleErr(err)
				return
			}
		}

		// Initialize a new build with clean DB if a user comments "/clean" on a PR
		if payload.Action == "created" &&
			payload.Issue.PullRequest != nil &&
			strings.Contains(payload.Comment.Body, "/clean") {
			// The issue comment payload doesn't contain the PR head, fetch this from GitHub
			var pullRequest PullRequestResponse
			res, err := w.g.Get(ctx, payload.Issue.PullRequest.URL, nil, &pullRequest)
			if err != nil {
				handleErr(err)
				return
			}
			res.Body.Close() // nolint: errcheck

			if err = w.b.NewBuild(
				ctx,
				pullRequest.Base.Repo.Owner.Login,
				pullRequest.Base.Repo.Name,
				pullRequest.Number,
				pullRequest.Head.Sha,
				payload.Sender.Login,
				false,
				true,
				true,
			); err != nil {
				handleErr(err)
				return
			}
		}

	case PingPayload:

		w.logger.Info("received ping payload")
	}

	rw.WriteHeader(http.StatusAccepted) // nolint: gosec, gas
}

func (w *Webhook) healthHandler(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK) // nolint: gosec, gas
}
