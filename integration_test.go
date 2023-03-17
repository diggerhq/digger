package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func skipCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}
}

func NewPullRequestTestEvent(parsedGhContext *Github, ghEvent map[string]interface{}, diggerConfig *DiggerConfig, prManager PullRequestManager, eventName string, dynamoDbLock *DynamoDbLock, tf TerraformExecutor) error {
	err := processGitHubContext(parsedGhContext, ghEvent, diggerConfig, prManager, eventName, dynamoDbLock, tf)
	if err != nil {
		print(err)
		os.Exit(1)
	}
	return nil
}

var githubContextDiggerPlanCommentMinJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "diggerhq/tfrun_demo_multienv",
  "repository_owner": "diggerhq",
  "repository_owner_id": "71334590",
  "workflow": "CI",
  "head_ref": "",
  "base_ref": "",
  "event_name": "issue_comment",
  "event": {
    "action": "created",
    "comment": {
      "author_association": "CONTRIBUTOR",
      "body": "digger plan",
      "created_at": "2023-03-13T15:14:08Z",
      "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11#issuecomment-1466341992",
      "id": 1466341992,
      "issue_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
      "node_id": "IC_kwDOJG5hVM5XZppo"
    }
  }
}`

var githubContextDiggerApplyCommentMinJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "diggerhq/tfrun_demo_multienv",
  "repository_owner": "diggerhq",
  "repository_owner_id": "71334590",
  "workflow": "CI",
  "head_ref": "",
  "base_ref": "",
  "event_name": "issue_comment",
  "event": {
    "action": "created",
    "comment": {
      "author_association": "CONTRIBUTOR",
      "body": "digger apply",
      "created_at": "2023-03-13T15:14:08Z",
      "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11#issuecomment-1466341992",
      "id": 1466341992,
      "issue_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
      "node_id": "IC_kwDOJG5hVM5XZppo"
    }
  }
}`

var githubContextNewPullRequestMinJson = `{
    "job": "build",
    "ref": "refs/pull/11/merge",
    "sha": "b8d885f7be8c742eccf037029b580dba7ab3d239",
    "repository": "diggerhq/tfrun_demo_multienv",
    "repository_owner": "diggerhq",
    "repository_owner_id": "71334590",
    "repositoryUrl": "git://github.com/diggerhq/tfrun_demo_multienv.git",
    "run_id": "4385306738",
    "run_number": "63",
    "retention_days": "90",
    "run_attempt": "1",
    "artifact_cache_size_limit": "10",
    "repository_visibility": "public",
    "repository_id": "611213652",
    "actor_id": "2407061",
    "actor": "veziak",
    "triggering_actor": "veziak",
    "workflow": "CI",
    "head_ref": "test-prod",
    "base_ref": "main",
    "event_name": "pull_request",
    "event": {
      "action": "opened",
      "number": 11,
      "pull_request": {
        "active_lock_reason": null,
        "additions": 0,
        "assignee": null,
        "assignees": [],
        "author_association": "CONTRIBUTOR",
        "auto_merge": null,
        "body": null,
        "changed_files": 1,
        "closed_at": null,
        "comments": 0,
        "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/comments",
        "commits": 1,
        "commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11/commits",
        "created_at": "2023-03-10T14:09:35Z",
        "deletions": 3,
        "diff_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11.diff",
        "draft": false,
        "head": {
          "label": "diggerhq:test-prod",
          "ref": "test-prod",
          "repo": {
            "allow_auto_merge": false,
            "allow_forking": true,
            "allow_merge_commit": true
          },
          "sha": "9d10ac8489bf70e466061f1042cde50db6027ffd",
          "user": {
            "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
            "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
            "followers_url": "https://api.github.com/users/diggerhq/followers",
            "following_url": "https://api.github.com/users/diggerhq/following{/other_user}"
          }
        },
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11",
        "id": 1271219596,
        "issue_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
        "labels": [],
        "locked": false,
        "maintainer_can_modify": false,
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11"
      }
    }
  }`

func TestHappyPath(t *testing.T) {
	skipCI(t)

	dir := createTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	createValidTerraformTestFile(dir)

	tf := Terraform{workingDir: dir}

	diggerConfig, err := NewDiggerConfig()
	assert.NoError(t, err)

	sess := session.Must(session.NewSession())
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")
	assert.NotEmpty(t, ghToken)

	newPullRequestContext := githubContextNewPullRequestMinJson
	parsedNewPullRequestContext, err := getGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := getGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := getGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	eventName := parsedNewPullRequestContext.EventName
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// new pr should lock the project
	err = processGitHubContext(&parsedNewPullRequestContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	assert.NoError(t, err)

	ghEvent = parsedDiggerPlanCommentContext.Event
	eventName = parsedDiggerPlanCommentContext.EventName
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	err = processGitHubContext(&parsedDiggerPlanCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	assert.NoError(t, err)

	ghEvent = parsedDiggerApplyCommentContext.Event
	eventName = parsedDiggerApplyCommentContext.EventName
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution
	err = processGitHubContext(&parsedDiggerApplyCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, &tf)
	assert.NoError(t, err)
}
