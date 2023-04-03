package github

import (
	"context"
	"log"

	"github.com/google/go-github/v50/github"
)

func NewGithubPullRequestService(ghToken string, repoName string, owner string) PullRequestManager {
	client := github.NewTokenClient(context.Background(), ghToken)
	return &GithubPullRequestService{
		Client:   client,
		RepoName: repoName,
		Owner:    owner,
	}
}

type GithubPullRequestService struct {
	Client   *github.Client
	RepoName string
	Owner    string
}

type PullRequestManager interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string)
	ReplyComment(prNumber int, reply string)
}

func (svc *GithubPullRequestService) GetChangedFiles(prNumber int) ([]string, error) {
	files, _, err := svc.Client.PullRequests.ListFiles(context.Background(), svc.Owner, svc.RepoName, prNumber, nil)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	fileNames := make([]string, len(files))

	for i, file := range files {
		fileNames[i] = *file.Filename
	}
	return fileNames, nil
}

func (svc *GithubPullRequestService) PublishComment(prNumber int, comment string) {
	_, _, err := svc.Client.Issues.CreateComment(context.Background(), svc.Owner, svc.RepoName, prNumber, &github.IssueComment{Body: &comment})
	if err != nil {
		log.Fatalf("error publishing comment: %v", err)
	}
}

func (svc *GithubPullRequestService) ReplyComment(prNumber int, reply string) {
	comments, _, err := svc.Client.PullRequests.ListComments(context.Background(), svc.Owner, svc.RepoName, prNumber, nil)
	if err != nil {
		log.Fatalf("error getting pull request comments: %v", err)
	}

	for _, comment := range comments {
		if comment.Body != nil && *comment.Body == "digger help" {
			svc.Client.PullRequests.CreateCommentInReplyTo(context.Background(), svc.Owner, svc.RepoName, prNumber, reply, *comment.ID)
		}
	}
}
