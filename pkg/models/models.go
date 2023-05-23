package models

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Github struct {
	Action           string `json:"action"`
	ActionPath       string `json:"action_path"`
	ActionRef        string `json:"action_ref"`
	ActionRepository string `json:"action_repository"`
	ActionStatus     string `json:"action_status"`
	Actor            string `json:"actor"`
	BaseRef          string `json:"base_ref"`
	Env              string `json:"env"`
	Event            Event  `json:"event"`
	EventName        string `json:"event_name"`
	EventPath        string `json:"event_path"`
	Path             string `json:"path"`
	RefType          string `json:"ref_type"`
	Repository       string `json:"repository"`
	RepositoryOwner  string `json:"repository_owner"`
}

type Event interface{}

func (g *Github) UnmarshalJSON(data []byte) error {
	type Alias Github
	aux := struct {
		*Alias
	}{
		Alias: (*Alias)(g),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var rawEvent json.RawMessage
	auxEvent := struct {
		Event *json.RawMessage `json:"event"`
	}{
		Event: &rawEvent,
	}
	if err := json.Unmarshal(data, &auxEvent); err != nil {
		return err
	}

	switch g.EventName {
	case "pull_request":
		var event PullRequestEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	case "issue_comment":
		var event IssueCommentEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	default:
		return errors.New("unknown GitHub event: " + g.EventName)
	}

	return nil
}

type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type PullRequest struct {
	Number int  `json:"number"`
	Merged bool `json:"merged"`
	Base   Base `json:"base"`
}

type IssueCommentEvent struct {
	Action  string  `json:"action"`
	Comment Comment `json:"comment"`
	Issue   Issue   `json:"issue"`
}

type Base struct {
	Ref string `json:"ref"`
}

type Comment struct {
	Body string `json:"body"`
}

type Issue struct {
	Number int `json:"number"`
}

type Repository struct {
	DefaultBranch string `json:"default_branch"`
}

func GetGitHubContext(ghContext string) (Github, error) {
	println(ghContext)
	var parsedGhContext Github
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return Github{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}
