package github

import (
	"context"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-github/v50/github"
	"log"
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
	CreateCheckStatus(branch string, commitSHA string) error
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

func (svc *GithubPullRequestService) CreateCheckStatus(branch string, commitSHA string) error {
	//status := &github.RepoStatus{
	//	Context:     github.String("my-check"),
	//	State:       github.String("pending"),
	//	Description: github.String("My custom check"),
	//}
	//_, res, err := svc.Client.Repositories.CreateStatus(context.Background(), svc.Owner, svc.RepoName, commitSHA, status)

	options := github.CreateCheckRunOptions{
		Name:    "my digger check",
		HeadSHA: commitSHA,
	}
	_, res, err := svc.Client.Checks.CreateCheckRun(context.Background(), svc.Owner, svc.RepoName, options)
	spew.Dump(res.StatusCode, err)
	return err
}
