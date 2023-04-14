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
	SetStatus(prNumber int, status string, statusContext string) error
	GetCombinedPullRequestStatus(prNumber int) (string, error)
	MergePullRequest(prNumber int) error
	IsMergeable(prNumber int) (bool, string, error)
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

func (svc *GithubPullRequestService) SetStatus(prNumber int, status string, statusContext string) error {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	_, _, err = svc.Client.Repositories.CreateStatus(context.Background(), svc.Owner, svc.RepoName, *pr.Head.SHA, &github.RepoStatus{
		State:       &status,
		Context:     &statusContext,
		Description: &statusContext,
	})
	return err
}

func (svc *GithubPullRequestService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	statuses, _, err := svc.Client.Repositories.GetCombinedStatus(context.Background(), svc.Owner, svc.RepoName, pr.Head.GetSHA(), nil)
	if err != nil {
		log.Fatalf("error getting combined status: %v", err)
	}

	return *statuses.State, nil
}

func (svc *GithubPullRequestService) MergePullRequest(prNumber int) error {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	_, _, err = svc.Client.PullRequests.Merge(context.Background(), svc.Owner, svc.RepoName, prNumber, "auto-merge", &github.PullRequestOptions{
		MergeMethod: "squash",
		SHA:         pr.Head.GetSHA(),
	})
	return err
}

func (svc *GithubPullRequestService) IsMergeable(prNumber int) (bool, string, error) {
	pr, _, err := svc.Client.PullRequests.Get(context.Background(), svc.Owner, svc.RepoName, prNumber)
	if err != nil {
		log.Fatalf("error getting pull request: %v", err)
	}

	return pr.GetMergeable(), pr.GetMergeableState(), nil
}
