package models

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/diggerhq/lib-orchestrator/github/models"
	"github.com/google/go-github/v54/github"
)

type GithubAction struct {
	Action           string      `json:"action"`
	ActionPath       string      `json:"action_path"`
	ActionRef        string      `json:"action_ref"`
	ActionRepository string      `json:"action_repository"`
	ActionStatus     string      `json:"action_status"`
	Actor            string      `json:"actor"`
	BaseRef          string      `json:"base_ref"`
	Env              string      `json:"env"`
	Event            interface{} `json:"event"`
	EventName        string      `json:"event_name"`
	EventPath        string      `json:"event_path"`
	Path             string      `json:"path"`
	RefType          string      `json:"ref_type"`
	Repository       string      `json:"repository"`
	RepositoryOwner  string      `json:"repository_owner"`
}

func (g *GithubAction) ToEventPackage() models.EventPackage {
	return models.EventPackage{
		Event:      g.Event,
		EventName:  g.EventName,
		Actor:      g.Actor,
		Repository: g.Repository,
	}
}
func (g *GithubAction) UnmarshalJSON(data []byte) error {
	type Alias GithubAction
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
		var event github.PullRequestEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	case "issue_comment":
		var event github.IssueCommentEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	case "push":
		var event github.PushEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	case "workflow_dispatch":
		var event github.WorkflowDispatchEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		g.Event = event
	default:
		return errors.New("unknown GitHub event: " + g.EventName)
	}

	return nil
}

func GetGitHubContext(ghContext string) (*GithubAction, error) {
	parsedGhContext := new(GithubAction)
	err := json.Unmarshal([]byte(ghContext), &parsedGhContext)
	if err != nil {
		return &GithubAction{}, fmt.Errorf("error parsing GitHub context JSON: %v", err)
	}
	return parsedGhContext, nil
}
