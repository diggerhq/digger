package bitbucket

import (
	"encoding/json"
	"fmt"
)

type Context struct {
	EventName string `json:"event_name"`
	RepoSlug  string `json:"repo_slug"`
	Author    string `json:"author"`
}

type EventType string

const (
	PullRequestOpened   = "pull request opened"
	PullRequestClosed   = "pull request closed"
	PullRequestUpdated  = "pull request updated"
	PullRequestApproved = "pull request approved"
	CommentCreated      = "comment created"
)

type Event struct {
	Type     EventType
	RepoName string
}

func ParseBitbucketContext(contextString string) (*Context, error) {
	var parsedBitbucketContext Context
	err := json.Unmarshal([]byte(contextString), &parsedBitbucketContext)
	if err != nil {
		return nil, fmt.Errorf("error parsing Bitbucket context JSON: %v", err)
	}
	return &parsedBitbucketContext, nil
}
