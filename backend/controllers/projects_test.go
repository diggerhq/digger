package controllers

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestAutomergeWhenBatchIsSuccessfulStatus(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)
	isMergeCalled := false
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposPullsByOwnerByRepoByPullNumber,
			github.PullRequest{
				Number: github.Int(2),
				Head:   &github.PullRequestBranch{Ref: github.String("main")},
			}),
		mock.WithRequestMatch(
			mock.GetReposByOwnerByRepo,
			github.Repository{
				Name:          github.String("testRepo"),
				DefaultBranch: github.String("main"),
			}),
		mock.WithRequestMatch(
			mock.GetReposGitRefByOwnerByRepoByRef,
			github.Reference{Object: &github.GitObject{SHA: github.String("test")}, Ref: github.String("test_ref")},
		),
		mock.WithRequestMatch(
			mock.PostReposGitRefsByOwnerByRepo,
			github.Reference{Object: &github.GitObject{SHA: github.String("test")}},
		),
		mock.WithRequestMatch(
			mock.GetReposContentsByOwnerByRepoByPath,
			github.RepositoryContent{},
		),
		mock.WithRequestMatchHandler(
			mock.PutReposPullsMergeByOwnerByRepoByPullNumber,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				isMergeCalled = true
			}),
		),
	)
	gh := &utils.DiggerGithubClientMockProvider{}
	gh.MockedHTTPClient = mockedHTTPClient

	batch := models.DiggerBatch{
		ID:         uuid.UUID{},
		PrNumber:   2,
		Status:     orchestrator_scheduler.BatchJobSucceeded,
		BranchName: "main",
		DiggerConfig: "" +
			"projects:\n" +
			"  - name: dev\n" +
			"    dir: dev\n" +
			"auto_merge: false",
		GithubInstallationId: int64(41584295),
		RepoFullName:         "diggerhq/github-job-scheduler",
		RepoOwner:            "diggerhq",
		RepoName:             "github-job-scheduler",
		BatchType:            orchestrator.DiggerCommandApply,
	}
	err := AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.False(t, isMergeCalled)

	batch.DiggerConfig = "" +
		"projects:\n" +
		"  - name: dev\n" +
		"    dir: dev\n" +
		"auto_merge: true"
	batch.BatchType = orchestrator.DiggerCommandPlan
	err = AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.False(t, isMergeCalled)

	batch.DiggerConfig = "" +
		"projects:\n" +
		"  - name: dev\n" +
		"    dir: dev\n" +
		"auto_merge: true"
	batch.BatchType = orchestrator.DiggerCommandApply
	err = AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.True(t, isMergeCalled)

}
