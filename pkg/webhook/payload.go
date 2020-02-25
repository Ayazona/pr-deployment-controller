package webhook

// PingPayload contains the information for GitHub's ping hook event
type PingPayload struct {
	HookID int `json:"hook_id"`
}

// PullRequestPayload contains the information for GitHub's pull_request hook event
type PullRequestPayload struct {
	Action      string `json:"action"`
	Number      int64  `json:"number"`
	PullRequest struct {
		Head struct {
			Sha  string `json:"sha"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"head"`
		Base struct {
			Repo struct {
				ID       int64  `json:"id"`
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				Owner    struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repo"`
		} `json:"base"`
	} `json:"pull_request"`
	Sender struct {
		Login string `json:"login"`
	}
}

// IssueCommentIssuePullRequestPayload contains the information about the pull_request (used by IssueCommentPayload)
type IssueCommentIssuePullRequestPayload struct {
	URL string `json:"url"`
}

// IssueCommentPayload contains the information for GitHub's issue_comment hook event
type IssueCommentPayload struct {
	Action string `json:"action"`
	Issue  struct {
		PullRequest *IssueCommentIssuePullRequestPayload `json:"pull_request,omitempty"`
	} `json:"issue"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
	Sender struct {
		Login string `json:"login"`
	}
}

// PullRequestResponse defines the response from GET pull_request Github api
type PullRequestResponse struct {
	Number int64 `json:"number"`
	Head   struct {
		Sha  string `json:"sha"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"head"`
	Base struct {
		Repo struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repo"`
	} `json:"base"`
}
