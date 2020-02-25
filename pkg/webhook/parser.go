package webhook

// nolint: gosec
import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Event represent supported webhook events
type Event string

var (
	// PingEvent stores the action reported by GitHub on an ping event
	PingEvent Event = "ping"
	// PullRequestEvent stores the action reported by GitHub on a PR event
	PullRequestEvent Event = "pull_request"
	// IssueCommentEvent stores the action reported by GitHub on a PR/Issue comment event
	IssueCommentEvent Event = "issue_comment"
)

var (
	// ErrInvalidHTTPMethod Error
	ErrInvalidHTTPMethod = errors.New("invalid HTTP Method")
	// ErrMissingGithubEventHeader Error
	ErrMissingGithubEventHeader = errors.New("missing X-GitHub-Event Header")
	// ErrMissingHubSignatureHeader Error
	ErrMissingHubSignatureHeader = errors.New("missing X-Hub-Signature Header")
	// ErrParsingPayload Error
	ErrParsingPayload = errors.New("error parsing payload")
	// ErrHMACVerificationFailed Error
	ErrHMACVerificationFailed = errors.New("HMAC verification failed")
)

// Parse parses GitHub webhooks
// nolint: gocyclo
func (w *Webhook) Parse(r *http.Request) (interface{}, error) {
	if r.Method != http.MethodPost {
		return nil, ErrInvalidHTTPMethod
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		return nil, ErrMissingGithubEventHeader
	}
	gitHubEvent := Event(event)

	payload, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close() // nolint: errcheck
	if err != nil || len(payload) == 0 {
		return nil, ErrParsingPayload
	}

	signature := r.Header.Get("X-Hub-Signature")
	if len(signature) == 0 {
		return nil, ErrMissingHubSignatureHeader
	}
	mac := hmac.New(sha1.New, []byte(w.secret))
	_, _ = mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature[5:]), []byte(expectedMAC)) {
		return nil, ErrHMACVerificationFailed
	}

	switch gitHubEvent {
	case PingEvent:
		var pl PingPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err
	case PullRequestEvent:
		var pl PullRequestPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err
	case IssueCommentEvent:
		var pl IssueCommentPayload
		err = json.Unmarshal(payload, &pl)
		return pl, err
	default:
		return nil, fmt.Errorf("unknown event %s", gitHubEvent)
	}
}
