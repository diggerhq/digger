package scm

import (
	"context"
	"digger/pkg/domain"
	"fmt"

	"github.com/google/go-github/v50/github"
)

type Github struct {
	client *github.Client
}

func NewGithubClient(token string) *Github {
	client := github.NewTokenClient(context.Background(), token)
	return &Github{
		client,
	}
}

func (gh *Github) PublishComment(comment string, prDetails domain.PRDetails) error {
	ghComment := &github.IssueComment{
		Body: &comment,
	}
	_, _, err := gh.client.Issues.CreateComment(
		context.Background(),
		prDetails.Owner,
		prDetails.RepositoryName,
		prDetails.Number,
		ghComment,
	)
	if err != nil {
		return fmt.Errorf("could not publish comment: %v", err)
	}

	return nil
}
