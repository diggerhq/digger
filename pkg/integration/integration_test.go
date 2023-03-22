package integration

import (
	"digger/pkg/aws"
	"digger/pkg/digger"
	"digger/pkg/github"
	"digger/pkg/terraform"
	"digger/pkg/utils"
	awssdk "github.com/aws/aws-sdk-go/aws"
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

func getProjetLockForTests() (error, *utils.ProjectLockImpl) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "digger-test",
		Config: awssdk.Config{
			Region: awssdk.String("us-east-1"),
		},
	})
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}

	repoOwner := "diggerhq"
	repositoryName := "test_dynamodb_lock"
	ghToken := "token"
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	projectLock := &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "test_dynamodb_lock",
		RepoName:     repositoryName,
	}
	return err, projectLock
}

var githubContextDiggerPlanCommentMinJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "digger_demo",
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
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11#issuecomment-1466341992",
      "id": 1466341992,
      "issue_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11",
      "node_id": "IC_kwDOJG5hVM5XZppo"
    },
    "issue": {
      "assignees": [],
      "author_association": "CONTRIBUTOR",
      "comments": 5,
      "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/comments",
      "created_at": "2023-03-10T14:09:35Z",
      "draft": false,
      "events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/events",
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
      "id": 1619042081,
      "labels": [],
      "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/labels{/name}",
      "locked": false,
      "node_id": "PR_kwDOJG5hVM5LxUWM",
      "number": 11,
      "pull_request": {
        "diff_url": "https://github.com/diggerhq/digger_demo/pull/11.diff",
        "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
        "patch_url": "https://github.com/diggerhq/digger_demo/pull/11.patch",
        "url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/11"
      }
    }
  }
}`

var githubContextDiggerApplyCommentMinJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "digger_demo",
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
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11#issuecomment-1466341992",
      "id": 1466341992,
      "issue_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11",
      "node_id": "IC_kwDOJG5hVM5XZppo"
    },
    "issue": {
      "assignees": [],
      "author_association": "CONTRIBUTOR",
      "comments": 5,
      "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/comments",
      "created_at": "2023-03-10T14:09:35Z",
      "draft": false,
      "events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/events",
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
      "id": 1619042081,
      "labels": [],
      "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/labels{/name}",
      "locked": false,
      "node_id": "PR_kwDOJG5hVM5LxUWM",
      "number": 11,
      "pull_request": {
        "diff_url": "https://github.com/diggerhq/digger_demo/pull/11.diff",
        "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
        "patch_url": "https://github.com/diggerhq/digger_demo/pull/11.patch",
        "url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/11"
      }
    }
  }
}`

var githubContextDiggerUnlockCommentMinJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "digger_demo",
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
      "body": "digger unlock",
      "created_at": "2023-03-13T15:14:08Z",
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11#issuecomment-1466341992",
      "id": 1466341992,
      "issue_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11",
      "node_id": "IC_kwDOJG5hVM5XZppo"
    },
    "issue": {
      "assignees": [],
      "author_association": "CONTRIBUTOR",
      "comments": 5,
      "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/comments",
      "created_at": "2023-03-10T14:09:35Z",
      "draft": false,
      "events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/events",
      "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
      "id": 1619042081,
      "labels": [],
      "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/labels{/name}",
      "locked": false,
      "node_id": "PR_kwDOJG5hVM5LxUWM",
      "number": 11,
      "pull_request": {
        "diff_url": "https://github.com/diggerhq/digger_demo/pull/11.diff",
        "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
        "patch_url": "https://github.com/diggerhq/digger_demo/pull/11.patch",
        "url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/11"
      }
    }
  }
}`

var githubContextNewPullRequestMinJson = `{
    "job": "build",
    "ref": "refs/pull/11/merge",
    "sha": "b8d885f7be8c742eccf037029b580dba7ab3d239",
    "repository": "digger_demo",
    "repository_owner": "diggerhq",
    "repository_owner_id": "71334590",
    "repositoryUrl": "git://github.com/diggerhq/digger_demo.git",
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
        "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11/comments",
        "commits": 1,
        "commits_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/11/commits",
        "created_at": "2023-03-10T14:09:35Z",
        "deletions": 3,
        "diff_url": "https://github.com/diggerhq/digger_demo/pull/11.diff",
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
        "html_url": "https://github.com/diggerhq/digger_demo/pull/11",
        "id": 1271219596,
        "issue_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/11",
        "labels": [],
        "locked": false,
        "maintainer_can_modify": false,
        "url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/11"
      }
    }
  }`

func TestHappyPath(t *testing.T) {
	skipCI(t)

	dir := terraform.CreateTestTerraformProject()
	cwd, _ := os.Getwd()

	defer func(name string, cwd string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
		//os.Chdir(cwd)
	}(dir, cwd)

	//os.Chdir(dir)
	terraform.CreateValidTerraformTestFile(dir)
	terraform.CreateSingleEnvDiggerYmlFile(dir)

	diggerConfig, err := digger.NewDiggerConfig(dir)
	assert.NoError(t, err)

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "digger-test",
		Config: awssdk.Config{
			Region: awssdk.String("us-east-1"),
		},
	})

	assert.NoError(t, err)
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")
	assert.NotEmpty(t, ghToken)

	println("--- new pull request ---")
	newPullRequestContext := githubContextNewPullRequestMinJson
	parsedNewPullRequestContext, err := digger.GetGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := digger.GetGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := digger.GetGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	diggerUnlockCommentContext := githubContextDiggerUnlockCommentMinJson
	parsedDiggerUnlockCommentContext, err := digger.GetGitHubContext(diggerUnlockCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	eventName := parsedNewPullRequestContext.EventName
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// new pr should lock the project
	err = digger.ProcessGitHubContext(&parsedNewPullRequestContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock := &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "dev",
		RepoName:     repositoryName,
	}
	resource := repositoryName + "#dev"
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Equal(t, 11, *transactionId, "TransactionId")

	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	eventName = parsedDiggerPlanCommentContext.EventName
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	err = digger.ProcessGitHubContext(&parsedDiggerPlanCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	eventName = parsedDiggerApplyCommentContext.EventName
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	err = digger.ProcessGitHubContext(&parsedDiggerApplyCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock = &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "dev",
		RepoName:     repositoryName,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)

	println("--- digger unlock comment ---")
	ghEvent = parsedDiggerUnlockCommentContext.Event
	eventName = parsedDiggerUnlockCommentContext.EventName
	repoOwner = parsedDiggerUnlockCommentContext.RepositoryOwner
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	err = digger.ProcessGitHubContext(&parsedDiggerUnlockCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock = &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "dev",
		RepoName:     repositoryName,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestMultiEnvHappyPath(t *testing.T) {
	skipCI(t)
	t.Skip()

	dir := terraform.CreateTestTerraformProject()
	cwd, _ := os.Getwd()

	defer func(name string, cwd string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
		//os.Chdir(cwd)
	}(dir, cwd)

	//os.Chdir(dir)
	terraform.CreateValidTerraformTestFile(dir)
	terraform.CreateMultiEnvDiggerYmlFile(dir)

	diggerConfig, err := digger.NewDiggerConfig(dir)
	assert.NoError(t, err)

	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "digger-test",
		Config: awssdk.Config{
			Region: awssdk.String("us-east-1"),
		},
	})

	assert.NoError(t, err)
	dynamoDb := dynamodb.New(sess)
	dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}

	ghToken := os.Getenv("GITHUB_TOKEN")
	assert.NotEmpty(t, ghToken)

	println("--- new pull request ---")
	newPullRequestContext := githubContextNewPullRequestMinJson
	parsedNewPullRequestContext, err := digger.GetGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := digger.GetGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := digger.GetGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	diggerUnlockCommentContext := githubContextDiggerUnlockCommentMinJson
	parsedDiggerUnlockCommentContext, err := digger.GetGitHubContext(diggerUnlockCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	eventName := parsedNewPullRequestContext.EventName
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// no files changed, no locks
	err = digger.ProcessGitHubContext(&parsedNewPullRequestContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock := &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "digger_demo",
		RepoName:     repositoryName,
	}
	resource := "digger_demo#default"
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Equal(t, 11, *transactionId, "TransactionId")

	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	eventName = parsedDiggerPlanCommentContext.EventName
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	err = digger.ProcessGitHubContext(&parsedDiggerPlanCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	eventName = parsedDiggerApplyCommentContext.EventName
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	err = digger.ProcessGitHubContext(&parsedDiggerApplyCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock = &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "digger_demo",
		RepoName:     repositoryName,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)

	println("--- digger unlock comment ---")
	ghEvent = parsedDiggerUnlockCommentContext.Event
	eventName = parsedDiggerUnlockCommentContext.EventName
	repoOwner = parsedDiggerUnlockCommentContext.RepositoryOwner
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	err = digger.ProcessGitHubContext(&parsedDiggerUnlockCommentContext, ghEvent, diggerConfig, githubPrService, eventName, &dynamoDbLock, dir)
	assert.NoError(t, err)

	projectLock = &utils.ProjectLockImpl{
		InternalLock: &dynamoDbLock,
		PrManager:    githubPrService,
		ProjectName:  "digger_demo",
		RepoName:     repositoryName,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestGetNonExistingLock(t *testing.T) {
	skipCI(t)

	err, projectLock := getProjetLockForTests()
	resource := "test_dynamodb_non_existing_lock#default"
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestGetExistingLock(t *testing.T) {
	skipCI(t)

	err, projectLock := getProjetLockForTests()
	resource := "test_dynamodb_existing_lock#default"
	locked, err := projectLock.InternalLock.Lock(2, 100, resource)
	assert.True(t, locked)

	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.NotNil(t, transactionId)
	assert.Equal(t, 100, *transactionId, "TransactionId")
}

func TestUnLock(t *testing.T) {
	skipCI(t)

	err, projectLock := getProjetLockForTests()
	resource := "test_dynamodb_unlock#default"
	locked, err := projectLock.InternalLock.Lock(2, 100, resource)
	assert.True(t, locked)

	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.NotNil(t, transactionId)
	assert.Equal(t, 100, *transactionId, "TransactionId")

	unlocked, err := projectLock.InternalLock.Unlock(resource)
	assert.True(t, unlocked)

	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}
