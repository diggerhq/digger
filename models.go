package main

type Github struct {
	Action           string `json:"action"`
	ActionPath       string `json:"action_path"`
	ActionRef        string `json:"action_ref"`
	ActionRepository string `json:"action_repository"`
	ActionStatus     string `json:"action_status"`
	Actor            string `json:"actor"`
	BaseRef          string `json:"base_ref"`
	Env              string `json:"env"`
	Event            []byte `json:"event"`
	EventName        string `json:"event_name"`
	EventPath        string `json:"event_path"`
	Path             string `json:"path"`
	RefType          string `json:"ref_type"`
	Repository       string `json:"repository"`
	RepositoryOwner  string `json:"repository_owner"`
}

type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
}

type PullRequest struct {
	Number int  `json:"number"`
	Merged bool `json:"merged"`
}

type IssueCommentEvent struct {
	Action  string  `json:"action"`
	Comment Comment `json:"comment"`
}

type Comment struct {
	Body  string `json:"body"`
	Issue Issue  `json:"issue"`
}

type Issue struct {
	Number int `json:"number"`
}
