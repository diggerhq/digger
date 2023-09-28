package integration

import (
	"context"
	"digger/pkg/aws"
	"digger/pkg/core/terraform"
	"digger/pkg/digger"
	"digger/pkg/github/models"
	"digger/pkg/locking"
	"digger/pkg/reporting"
	"digger/pkg/storage"
	"digger/pkg/utils"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	configuration "github.com/diggerhq/lib-digger-config"
	dg_github "github.com/diggerhq/lib-orchestrator/github"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/go-github/v55/github"
	"github.com/stretchr/testify/assert"
)

func SkipCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}
}

func getProjectLockForTests() (error, *locking.PullRequestLock) {
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
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)
	reporter := reporting.CiReporter{
		CiService:      &githubPrService,
		PrNumber:       1,
		ReportStrategy: &reporting.MultipleCommentsStrategy{},
	}

	projectLock := &locking.PullRequestLock{
		InternalLock:     &dynamoDbLock,
		Reporter:         &reporter,
		CIService:        &githubPrService,
		ProjectName:      "test_dynamodb_lock",
		ProjectNamespace: repoOwner + "/" + repositoryName,
	}
	return err, projectLock
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
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
	  "repository": {
		"default_branch": "main"
	  },
      "pull_request": {
        "active_lock_reason": null,
		"number": 11,
        "merged": false,
        "additions": 0,
        "assignee": null,
        "assignees": [],
		"base": {
			"ref": "main"
		},
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

var githubContextUnknownEventJson = `{
  "job": "build",
  "ref": "refs/heads/main",
  "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
  "repository": "digger_demo",
  "repository_owner": "diggerhq",
  "repository_owner_id": "71334590",
  "workflow": "CI",
  "head_ref": "",
  "base_ref": "",
  "event_name": "non_existent_event",
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

func TestHappyPath(t *testing.T) {
	/*
		to be able to run this test following env vars should be configured:
		AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION, GITHUB_TOKEN
	*/

	SkipCI(t)

	dir := terraform.CreateTestTerraformProject()

	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	terraform.CreateValidTerraformTestFile(dir)
	terraform.CreateSingleEnvDiggerYmlFile(dir)

	diggerConfig, _, _, err := configuration.LoadDiggerConfig(dir)
	assert.NoError(t, err)

	lock, err := locking.GetLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock, "failed to create lock")

	ghToken := os.Getenv("GITHUB_TOKEN")
	assert.NotEmpty(t, ghToken)

	println("--- new pull request ---")
	newPullRequestContext := githubContextNewPullRequestMinJson
	parsedNewPullRequestContext, err := models.GetGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := models.GetGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := models.GetGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	diggerUnlockCommentContext := githubContextDiggerUnlockCommentMinJson
	parsedDiggerUnlockCommentContext, err := models.GetGitHubContext(diggerUnlockCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	//  new pr should lock the project
	impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	event := ghEvent.(github.PullRequestEvent)
	jobs, _, err := dg_github.ConvertGithubPullRequestEventToJobs(&event, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	zipManager := utils.Zipper{}
	planStorage := &storage.GithubPlanStorage{
		Client:            github.NewTokenClient(context.Background(), ghToken),
		Owner:             repoOwner,
		RepoName:          repositoryName,
		PullRequestNumber: prNumber,
		ZipManager:        zipManager,
	}

	reporter := &reporting.CiReporter{
		CiService: &githubPrService,
		PrNumber:  prNumber,
	}

	diggerProjectNamespace := repoOwner + "/" + repositoryName

	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock := &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	resource := repositoryName + "#" + projectLock.ProjectName
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.NotNil(t, transactionId)
	assert.Equal(t, 11, *transactionId, "TransactionId")

	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	prEvent := ghEvent.(github.PullRequestEvent)
	jobs, _, err = dg_github.ConvertGithubPullRequestEventToJobs(&prEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)

	cEvent := ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)

	println("--- digger unlock comment ---")
	ghEvent = parsedDiggerUnlockCommentContext.Event
	repoOwner = parsedDiggerUnlockCommentContext.RepositoryOwner
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent = ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestMultiEnvHappyPath(t *testing.T) {
	SkipCI(t)
	t.Skip()

	dir := terraform.CreateTestTerraformProject()

	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	terraform.CreateValidTerraformTestFile(dir)
	terraform.CreateMultiEnvDiggerYmlFile(dir)

	diggerConfig, _, _, err := configuration.LoadDiggerConfig(dir)
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
	parsedNewPullRequestContext, err := models.GetGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := models.GetGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := models.GetGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	diggerUnlockCommentContext := githubContextDiggerUnlockCommentMinJson
	parsedDiggerUnlockCommentContext, err := models.GetGitHubContext(diggerUnlockCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	diggerProjectNamespace := repoOwner + "/" + repositoryName
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// no files changed, no locks
	impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	pEvent := ghEvent.(github.PullRequestEvent)
	jobs, _, err := dg_github.ConvertGithubPullRequestEventToJobs(&pEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)

	zipManager := utils.Zipper{}
	planStorage := &storage.GithubPlanStorage{
		Client:            github.NewTokenClient(context.Background(), ghToken),
		Owner:             repoOwner,
		RepoName:          repositoryName,
		PullRequestNumber: prNumber,
		ZipManager:        zipManager,
	}

	reporter := &reporting.CiReporter{
		CiService: &githubPrService,
		PrNumber:  prNumber,
	}
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, &dynamoDbLock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock := &locking.PullRequestLock{
		InternalLock:     &dynamoDbLock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "digger_demo",
		ProjectNamespace: diggerProjectNamespace,
	}
	resource := "digger_demo#default"
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Equal(t, 11, *transactionId, "TransactionId")

	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent := ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, &dynamoDbLock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent = ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, &dynamoDbLock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     &dynamoDbLock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "digger_demo",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)

	println("--- digger unlock comment ---")
	ghEvent = parsedDiggerUnlockCommentContext.Event
	repoOwner = parsedDiggerUnlockCommentContext.RepositoryOwner
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent = ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, &dynamoDbLock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     &dynamoDbLock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "digger_demo",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestGetNonExistingLock(t *testing.T) {
	SkipCI(t)

	err, projectLock := getProjectLockForTests()
	resource := "test_dynamodb_non_existing_lock#default"
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.Nil(t, transactionId)
}

func TestGetExistingLock(t *testing.T) {
	SkipCI(t)

	err, projectLock := getProjectLockForTests()
	randString := randomString(8)
	resource := "test_dynamodb_existing_lock_" + randString + "#default"
	locked, err := projectLock.InternalLock.Lock(100, resource)
	assert.True(t, locked)

	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.NotNil(t, transactionId)
	assert.Equal(t, 100, *transactionId, "TransactionId")
}

func TestUnLock(t *testing.T) {
	SkipCI(t)

	err, projectLock := getProjectLockForTests()
	resource := "test_dynamodb_unlock#default"
	locked, err := projectLock.InternalLock.Lock(100, resource)
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

func TestNonExistentGitHubEvent(t *testing.T) {

	unknownEventContext := githubContextUnknownEventJson
	_, err := models.GetGitHubContext(unknownEventContext)
	println(err.Error())
	assert.Error(t, err)
	assert.Equal(t, "error parsing GitHub context JSON: unknown GitHub event: non_existent_event", err.Error())
}

func TestCustomCommandHappyPath(t *testing.T) {
	SkipCI(t)
	diggerCfg := `
projects:
- name: dev
  dir: infra/dev
  workflow: myworkflow

workflows:
  myworkflow:
    plan:
      steps:
      - run: echo "hello"
`

	dir := terraform.CreateTestTerraformProject()

	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	terraform.CreateValidTerraformTestFile(dir)
	terraform.CreateCustomDiggerYmlFile(dir, diggerCfg)

	diggerConfig, _, _, err := configuration.LoadDiggerConfig(dir)
	assert.NoError(t, err)

	assert.NotNil(t, diggerConfig.Workflows)
	assert.NotNil(t, diggerConfig.Workflows["myworkflow"])
	assert.NotNil(t, diggerConfig.Workflows["myworkflow"].Configuration)
	assert.NotNil(t, diggerConfig.Workflows["myworkflow"].Configuration.OnCommitToDefault)

	lock, err := locking.GetLock()
	assert.NoError(t, err)
	assert.NotNil(t, lock, "failed to create lock")

	ghToken := os.Getenv("GITHUB_TOKEN")
	assert.NotEmpty(t, ghToken)

	println("--- new pull request ---")
	newPullRequestContext := githubContextNewPullRequestMinJson
	parsedNewPullRequestContext, err := models.GetGitHubContext(newPullRequestContext)
	assert.NoError(t, err)

	diggerPlanCommentContext := githubContextDiggerPlanCommentMinJson
	parsedDiggerPlanCommentContext, err := models.GetGitHubContext(diggerPlanCommentContext)
	assert.NoError(t, err)

	diggerApplyCommentContext := githubContextDiggerApplyCommentMinJson
	parsedDiggerApplyCommentContext, err := models.GetGitHubContext(diggerApplyCommentContext)
	assert.NoError(t, err)

	diggerUnlockCommentContext := githubContextDiggerUnlockCommentMinJson
	parsedDiggerUnlockCommentContext, err := models.GetGitHubContext(diggerUnlockCommentContext)
	assert.NoError(t, err)

	ghEvent := parsedNewPullRequestContext.Event
	repoOwner := parsedNewPullRequestContext.RepositoryOwner
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := dg_github.NewGitHubService(ghToken, repositoryName, repoOwner)
	diggerProjectNamespace := repoOwner + "/" + repositoryName

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	//  new pr should lock the project
	impactedProjects, requestedProject, prNumber, err := dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	pEvent := ghEvent.(github.PullRequestEvent)
	jobs, _, err := dg_github.ConvertGithubPullRequestEventToJobs(&pEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)

	zipManager := utils.Zipper{}
	planStorage := &storage.GithubPlanStorage{
		Client:            github.NewTokenClient(context.Background(), ghToken),
		Owner:             repoOwner,
		RepoName:          repositoryName,
		PullRequestNumber: prNumber,
		ZipManager:        zipManager,
	}

	reporter := &reporting.CiReporter{
		CiService: &githubPrService,
		PrNumber:  prNumber,
	}

	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock := &locking.PullRequestLock{
		InternalLock:     lock,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	resource := repositoryName + "#" + projectLock.ProjectName
	transactionId, err := projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
	assert.NotNil(t, transactionId)
	assert.Equal(t, 42, *transactionId, "TransactionId")

	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	repoOwner = parsedDiggerPlanCommentContext.RepositoryOwner
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent := ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent = ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)

	println("--- digger unlock comment ---")
	ghEvent = parsedDiggerUnlockCommentContext.Event
	repoOwner = parsedDiggerUnlockCommentContext.RepositoryOwner
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	impactedProjects, requestedProject, prNumber, err = dg_github.ProcessGitHubEvent(ghEvent, diggerConfig, &githubPrService)
	assert.NoError(t, err)
	cEvent = ghEvent.(github.IssueCommentEvent)
	jobs, _, err = dg_github.ConvertGithubIssueCommentEventToJobs(&cEvent, impactedProjects, requestedProject, diggerConfig.Workflows)
	assert.NoError(t, err)
	_, _, err = digger.RunJobs(jobs, &githubPrService, &githubPrService, lock, reporter, planStorage, nil, nil, dir)
	assert.NoError(t, err)

	projectLock = &locking.PullRequestLock{
		InternalLock:     lock,
		Reporter:         reporter,
		CIService:        &githubPrService,
		ProjectName:      "dev",
		ProjectNamespace: diggerProjectNamespace,
	}
	transactionId, err = projectLock.InternalLock.GetLock(resource)
	assert.NoError(t, err)
}
