package controllers

import (
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
)

func TestAutomergeWhenBatchIsSuccessfulStatus(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)
	isMergeCalled := false
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposIssuesByOwnerByRepoByIssueNumber,
			github.Issue{
				Number:           github.Int(2),
				PullRequestLinks: &github.PullRequestLinks{},
			}),
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
		VCS:        "github",
		ID:         uuid.UUID{},
		PrNumber:   2,
		Status:     orchestrator_scheduler.BatchJobSucceeded,
		BranchName: "main",
		DiggerConfig: "" +
			"projects:\n" +
			"  - name: dev\n" +
			"    dir: dev\n" +
			"auto_merge: false",
		GithubInstallationId:     int64(41584295),
		RepoFullName:             "diggerhq/github-job-scheduler",
		RepoOwner:                "diggerhq",
		RepoName:                 "github-job-scheduler",
		BatchType:                orchestrator_scheduler.DiggerCommandApply,
		CoverAllImpactedProjects: true,
	}
	err := AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.False(t, isMergeCalled)

	batch.DiggerConfig = "" +
		"projects:\n" +
		"  - name: dev\n" +
		"    dir: dev\n" +
		"auto_merge: true"
	batch.BatchType = orchestrator_scheduler.DiggerCommandPlan
	err = AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.False(t, isMergeCalled)

	batch.DiggerConfig = "" +
		"projects:\n" +
		"  - name: dev\n" +
		"    dir: dev\n" +
		"auto_merge: true"
	batch.BatchType = orchestrator_scheduler.DiggerCommandApply
	batch.CoverAllImpactedProjects = false
	err = AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.False(t, isMergeCalled)

	batch.DiggerConfig = "" +
		"projects:\n" +
		"  - name: dev\n" +
		"    dir: dev\n" +
		"auto_merge: true"
	batch.BatchType = orchestrator_scheduler.DiggerCommandApply
	batch.CoverAllImpactedProjects = true
	err = AutomergePRforBatchIfEnabled(gh, &batch)
	assert.NoError(t, err)
	assert.True(t, isMergeCalled)
}

func TestCharacterLimit(t *testing.T) {
	tests := []struct {
		name           string
		inputLength    int
		expectTruncate bool
	}{
		{
			name:           "under limit - no truncation",
			inputLength:    1000,
			expectTruncate: false,
		},
		{
			name:           "at limit - no truncation",
			inputLength:    65535,
			expectTruncate: false,
		},
		{
			name:           "over limit - truncation applied",
			inputLength:    70000,
			expectTruncate: true,
		},
	}

	const maxCheckRunTextLength = 65535
	cutOffMsg := "\n[Character limit exceeded, output truncated]"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Repeat("a", tt.inputLength)

			result := input
			if utf8.RuneCountInString(result) > maxCheckRunTextLength {
				runes := []rune(result)
				truncateAt := maxCheckRunTextLength - utf8.RuneCountInString(cutOffMsg)
				result = string(runes[:truncateAt]) + cutOffMsg
			}

			if tt.expectTruncate {
				assert.Equal(t, maxCheckRunTextLength, utf8.RuneCountInString(result),
					"truncated output should be exactly 65535 characters")
				assert.True(t, strings.HasSuffix(result, cutOffMsg),
					"truncated output should end with cutoff message")
			} else {
				assert.Equal(t, tt.inputLength, utf8.RuneCountInString(result),
					"non-truncated output should maintain original length")
				assert.False(t, strings.HasSuffix(result, cutOffMsg),
					"non-truncated output should not have cutoff message")
			}
		})
	}
}
