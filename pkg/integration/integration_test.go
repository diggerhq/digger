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
	"strings"
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
    "token": "***",
    "job": "build",
    "ref": "refs/pull/42/merge",
    "sha": "b9ba324329ed046ae07a163cccffb72c60db697c",
    "repository": "diggerhq/digger_demo",
    "repository_owner": "diggerhq",
    "repository_owner_id": "71334590",
    "repositoryUrl": "git://github.com/diggerhq/digger_demo.git",
    "run_id": "4545505853",
    "run_number": "800",
    "retention_days": "90",
    "run_attempt": "1",
    "artifact_cache_size_limit": "10",
    "repository_visibility": "public",
    "repository_id": "606119156",
    "actor_id": "1627972",
    "actor": "motatoes",
    "triggering_actor": "motatoes",
    "workflow": "CI",
    "head_ref": "motatoes-patch-23",
    "base_ref": "main",
    "event_name": "pull_request",
    "event": {
      "action": "opened",
      "number": 42,
      "organization": {
        "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
        "description": "",
        "events_url": "https://api.github.com/orgs/diggerhq/events",
        "hooks_url": "https://api.github.com/orgs/diggerhq/hooks",
        "id": 71334590,
        "issues_url": "https://api.github.com/orgs/diggerhq/issues",
        "login": "diggerhq",
        "members_url": "https://api.github.com/orgs/diggerhq/members{/member}",
        "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
        "public_members_url": "https://api.github.com/orgs/diggerhq/public_members{/member}",
        "repos_url": "https://api.github.com/orgs/diggerhq/repos",
        "url": "https://api.github.com/orgs/diggerhq"
      },
      "pull_request": {
        "_links": {
          "comments": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/issues/42/comments"
          },
          "commits": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42/commits"
          },
          "html": {
            "href": "https://github.com/diggerhq/digger_demo/pull/42"
          },
          "issue": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/issues/42"
          },
          "review_comment": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/pulls/comments{/number}"
          },
          "review_comments": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42/comments"
          },
          "self": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42"
          },
          "statuses": {
            "href": "https://api.github.com/repos/diggerhq/digger_demo/statuses/327f62ba30d8883cc0a1a603090e4331a18245fa"
          }
        },
        "active_lock_reason": null,
        "additions": 1,
        "assignee": null,
        "assignees": [],
        "author_association": "CONTRIBUTOR",
        "auto_merge": null,
        "base": {
          "label": "diggerhq:main",
          "ref": "main",
          "repo": {
            "allow_auto_merge": false,
            "allow_forking": true,
            "allow_merge_commit": true,
            "allow_rebase_merge": true,
            "allow_squash_merge": true,
            "allow_update_branch": false,
            "archive_url": "https://api.github.com/repos/diggerhq/digger_demo/{archive_format}{/ref}",
            "archived": false,
            "assignees_url": "https://api.github.com/repos/diggerhq/digger_demo/assignees{/user}",
            "blobs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/blobs{/sha}",
            "branches_url": "https://api.github.com/repos/diggerhq/digger_demo/branches{/branch}",
            "clone_url": "https://github.com/diggerhq/digger_demo.git",
            "collaborators_url": "https://api.github.com/repos/diggerhq/digger_demo/collaborators{/collaborator}",
            "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/comments{/number}",
            "commits_url": "https://api.github.com/repos/diggerhq/digger_demo/commits{/sha}",
            "compare_url": "https://api.github.com/repos/diggerhq/digger_demo/compare/{base}...{head}",
            "contents_url": "https://api.github.com/repos/diggerhq/digger_demo/contents/{+path}",
            "contributors_url": "https://api.github.com/repos/diggerhq/digger_demo/contributors",
            "created_at": "2023-02-24T16:35:52Z",
            "default_branch": "main",
            "delete_branch_on_merge": false,
            "deployments_url": "https://api.github.com/repos/diggerhq/digger_demo/deployments",
            "description": null,
            "disabled": false,
            "downloads_url": "https://api.github.com/repos/diggerhq/digger_demo/downloads",
            "events_url": "https://api.github.com/repos/diggerhq/digger_demo/events",
            "fork": false,
            "forks": 7,
            "forks_count": 7,
            "forks_url": "https://api.github.com/repos/diggerhq/digger_demo/forks",
            "full_name": "diggerhq/digger_demo",
            "git_commits_url": "https://api.github.com/repos/diggerhq/digger_demo/git/commits{/sha}",
            "git_refs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/refs{/sha}",
            "git_tags_url": "https://api.github.com/repos/diggerhq/digger_demo/git/tags{/sha}",
            "git_url": "git://github.com/diggerhq/digger_demo.git",
            "has_discussions": false,
            "has_downloads": true,
            "has_issues": true,
            "has_pages": false,
            "has_projects": true,
            "has_wiki": true,
            "homepage": null,
            "hooks_url": "https://api.github.com/repos/diggerhq/digger_demo/hooks",
            "html_url": "https://github.com/diggerhq/digger_demo",
            "id": 606119156,
            "is_template": true,
            "issue_comment_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/comments{/number}",
            "issue_events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/events{/number}",
            "issues_url": "https://api.github.com/repos/diggerhq/digger_demo/issues{/number}",
            "keys_url": "https://api.github.com/repos/diggerhq/digger_demo/keys{/key_id}",
            "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/labels{/name}",
            "language": "HCL",
            "languages_url": "https://api.github.com/repos/diggerhq/digger_demo/languages",
            "license": null,
            "merge_commit_message": "PR_TITLE",
            "merge_commit_title": "MERGE_MESSAGE",
            "merges_url": "https://api.github.com/repos/diggerhq/digger_demo/merges",
            "milestones_url": "https://api.github.com/repos/diggerhq/digger_demo/milestones{/number}",
            "mirror_url": null,
            "name": "digger_demo",
            "node_id": "R_kgDOJCCk9A",
            "notifications_url": "https://api.github.com/repos/diggerhq/digger_demo/notifications{?since,all,participating}",
            "open_issues": 2,
            "open_issues_count": 2,
            "owner": {
              "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
              "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
              "followers_url": "https://api.github.com/users/diggerhq/followers",
              "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
              "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
              "gravatar_id": "",
              "html_url": "https://github.com/diggerhq",
              "id": 71334590,
              "login": "diggerhq",
              "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
              "organizations_url": "https://api.github.com/users/diggerhq/orgs",
              "received_events_url": "https://api.github.com/users/diggerhq/received_events",
              "repos_url": "https://api.github.com/users/diggerhq/repos",
              "site_admin": false,
              "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
              "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
              "type": "Organization",
              "url": "https://api.github.com/users/diggerhq"
            },
            "private": false,
            "pulls_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls{/number}",
            "pushed_at": "2023-03-28T16:41:34Z",
            "releases_url": "https://api.github.com/repos/diggerhq/digger_demo/releases{/id}",
            "size": 51,
            "squash_merge_commit_message": "COMMIT_MESSAGES",
            "squash_merge_commit_title": "COMMIT_OR_PR_TITLE",
            "ssh_url": "git@github.com:diggerhq/digger_demo.git",
            "stargazers_count": 3,
            "stargazers_url": "https://api.github.com/repos/diggerhq/digger_demo/stargazers",
            "statuses_url": "https://api.github.com/repos/diggerhq/digger_demo/statuses/{sha}",
            "subscribers_url": "https://api.github.com/repos/diggerhq/digger_demo/subscribers",
            "subscription_url": "https://api.github.com/repos/diggerhq/digger_demo/subscription",
            "svn_url": "https://github.com/diggerhq/digger_demo",
            "tags_url": "https://api.github.com/repos/diggerhq/digger_demo/tags",
            "teams_url": "https://api.github.com/repos/diggerhq/digger_demo/teams",
            "topics": [],
            "trees_url": "https://api.github.com/repos/diggerhq/digger_demo/git/trees{/sha}",
            "updated_at": "2023-03-15T12:50:14Z",
            "url": "https://api.github.com/repos/diggerhq/digger_demo",
            "use_squash_pr_title_as_default": false,
            "visibility": "public",
            "watchers": 3,
            "watchers_count": 3,
            "web_commit_signoff_required": false
          },
          "sha": "99379e7bba527245bff28082dbfed4f0e67a0547",
          "user": {
            "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
            "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
            "followers_url": "https://api.github.com/users/diggerhq/followers",
            "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
            "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
            "gravatar_id": "",
            "html_url": "https://github.com/diggerhq",
            "id": 71334590,
            "login": "diggerhq",
            "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
            "organizations_url": "https://api.github.com/users/diggerhq/orgs",
            "received_events_url": "https://api.github.com/users/diggerhq/received_events",
            "repos_url": "https://api.github.com/users/diggerhq/repos",
            "site_admin": false,
            "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
            "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
            "type": "Organization",
            "url": "https://api.github.com/users/diggerhq"
          }
        },
        "body": null,
        "changed_files": 1,
        "closed_at": null,
        "comments": 0,
        "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/42/comments",
        "commits": 1,
        "commits_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42/commits",
        "created_at": "2023-03-28T16:41:33Z",
        "deletions": 1,
        "diff_url": "https://github.com/diggerhq/digger_demo/pull/42.diff",
        "draft": false,
        "head": {
          "label": "diggerhq:motatoes-patch-23",
          "ref": "motatoes-patch-23",
          "repo": {
            "allow_auto_merge": false,
            "allow_forking": true,
            "allow_merge_commit": true,
            "allow_rebase_merge": true,
            "allow_squash_merge": true,
            "allow_update_branch": false,
            "archive_url": "https://api.github.com/repos/diggerhq/digger_demo/{archive_format}{/ref}",
            "archived": false,
            "assignees_url": "https://api.github.com/repos/diggerhq/digger_demo/assignees{/user}",
            "blobs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/blobs{/sha}",
            "branches_url": "https://api.github.com/repos/diggerhq/digger_demo/branches{/branch}",
            "clone_url": "https://github.com/diggerhq/digger_demo.git",
            "collaborators_url": "https://api.github.com/repos/diggerhq/digger_demo/collaborators{/collaborator}",
            "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/comments{/number}",
            "commits_url": "https://api.github.com/repos/diggerhq/digger_demo/commits{/sha}",
            "compare_url": "https://api.github.com/repos/diggerhq/digger_demo/compare/{base}...{head}",
            "contents_url": "https://api.github.com/repos/diggerhq/digger_demo/contents/{+path}",
            "contributors_url": "https://api.github.com/repos/diggerhq/digger_demo/contributors",
            "created_at": "2023-02-24T16:35:52Z",
            "default_branch": "main",
            "delete_branch_on_merge": false,
            "deployments_url": "https://api.github.com/repos/diggerhq/digger_demo/deployments",
            "description": null,
            "disabled": false,
            "downloads_url": "https://api.github.com/repos/diggerhq/digger_demo/downloads",
            "events_url": "https://api.github.com/repos/diggerhq/digger_demo/events",
            "fork": false,
            "forks": 7,
            "forks_count": 7,
            "forks_url": "https://api.github.com/repos/diggerhq/digger_demo/forks",
            "full_name": "diggerhq/digger_demo",
            "git_commits_url": "https://api.github.com/repos/diggerhq/digger_demo/git/commits{/sha}",
            "git_refs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/refs{/sha}",
            "git_tags_url": "https://api.github.com/repos/diggerhq/digger_demo/git/tags{/sha}",
            "git_url": "git://github.com/diggerhq/digger_demo.git",
            "has_discussions": false,
            "has_downloads": true,
            "has_issues": true,
            "has_pages": false,
            "has_projects": true,
            "has_wiki": true,
            "homepage": null,
            "hooks_url": "https://api.github.com/repos/diggerhq/digger_demo/hooks",
            "html_url": "https://github.com/diggerhq/digger_demo",
            "id": 606119156,
            "is_template": true,
            "issue_comment_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/comments{/number}",
            "issue_events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/events{/number}",
            "issues_url": "https://api.github.com/repos/diggerhq/digger_demo/issues{/number}",
            "keys_url": "https://api.github.com/repos/diggerhq/digger_demo/keys{/key_id}",
            "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/labels{/name}",
            "language": "HCL",
            "languages_url": "https://api.github.com/repos/diggerhq/digger_demo/languages",
            "license": null,
            "merge_commit_message": "PR_TITLE",
            "merge_commit_title": "MERGE_MESSAGE",
            "merges_url": "https://api.github.com/repos/diggerhq/digger_demo/merges",
            "milestones_url": "https://api.github.com/repos/diggerhq/digger_demo/milestones{/number}",
            "mirror_url": null,
            "name": "digger_demo",
            "node_id": "R_kgDOJCCk9A",
            "notifications_url": "https://api.github.com/repos/diggerhq/digger_demo/notifications{?since,all,participating}",
            "open_issues": 2,
            "open_issues_count": 2,
            "owner": {
              "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
              "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
              "followers_url": "https://api.github.com/users/diggerhq/followers",
              "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
              "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
              "gravatar_id": "",
              "html_url": "https://github.com/diggerhq",
              "id": 71334590,
              "login": "diggerhq",
              "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
              "organizations_url": "https://api.github.com/users/diggerhq/orgs",
              "received_events_url": "https://api.github.com/users/diggerhq/received_events",
              "repos_url": "https://api.github.com/users/diggerhq/repos",
              "site_admin": false,
              "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
              "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
              "type": "Organization",
              "url": "https://api.github.com/users/diggerhq"
            },
            "private": false,
            "pulls_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls{/number}",
            "pushed_at": "2023-03-28T16:41:34Z",
            "releases_url": "https://api.github.com/repos/diggerhq/digger_demo/releases{/id}",
            "size": 51,
            "squash_merge_commit_message": "COMMIT_MESSAGES",
            "squash_merge_commit_title": "COMMIT_OR_PR_TITLE",
            "ssh_url": "git@github.com:diggerhq/digger_demo.git",
            "stargazers_count": 3,
            "stargazers_url": "https://api.github.com/repos/diggerhq/digger_demo/stargazers",
            "statuses_url": "https://api.github.com/repos/diggerhq/digger_demo/statuses/{sha}",
            "subscribers_url": "https://api.github.com/repos/diggerhq/digger_demo/subscribers",
            "subscription_url": "https://api.github.com/repos/diggerhq/digger_demo/subscription",
            "svn_url": "https://github.com/diggerhq/digger_demo",
            "tags_url": "https://api.github.com/repos/diggerhq/digger_demo/tags",
            "teams_url": "https://api.github.com/repos/diggerhq/digger_demo/teams",
            "topics": [],
            "trees_url": "https://api.github.com/repos/diggerhq/digger_demo/git/trees{/sha}",
            "updated_at": "2023-03-15T12:50:14Z",
            "url": "https://api.github.com/repos/diggerhq/digger_demo",
            "use_squash_pr_title_as_default": false,
            "visibility": "public",
            "watchers": 3,
            "watchers_count": 3,
            "web_commit_signoff_required": false
          },
          "sha": "327f62ba30d8883cc0a1a603090e4331a18245fa",
          "user": {
            "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
            "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
            "followers_url": "https://api.github.com/users/diggerhq/followers",
            "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
            "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
            "gravatar_id": "",
            "html_url": "https://github.com/diggerhq",
            "id": 71334590,
            "login": "diggerhq",
            "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
            "organizations_url": "https://api.github.com/users/diggerhq/orgs",
            "received_events_url": "https://api.github.com/users/diggerhq/received_events",
            "repos_url": "https://api.github.com/users/diggerhq/repos",
            "site_admin": false,
            "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
            "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
            "type": "Organization",
            "url": "https://api.github.com/users/diggerhq"
          }
        },
        "html_url": "https://github.com/diggerhq/digger_demo/pull/42",
        "id": 1293328757,
        "issue_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/42",
        "labels": [],
        "locked": false,
        "maintainer_can_modify": false,
        "merge_commit_sha": null,
        "mergeable": null,
        "mergeable_state": "unknown",
        "merged": false,
        "merged_at": null,
        "merged_by": null,
        "milestone": null,
        "node_id": "PR_kwDOJCCk9M5NFqF1",
        "number": 42,
        "patch_url": "https://github.com/diggerhq/digger_demo/pull/42.patch",
        "rebaseable": null,
        "requested_reviewers": [],
        "requested_teams": [],
        "review_comment_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/comments{/number}",
        "review_comments": 0,
        "review_comments_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42/comments",
        "state": "open",
        "statuses_url": "https://api.github.com/repos/diggerhq/digger_demo/statuses/327f62ba30d8883cc0a1a603090e4331a18245fa",
        "title": "do not close me",
        "updated_at": "2023-03-28T16:41:34Z",
        "url": "https://api.github.com/repos/diggerhq/digger_demo/pulls/42",
        "user": {
          "avatar_url": "https://avatars.githubusercontent.com/u/1627972?v=4",
          "events_url": "https://api.github.com/users/motatoes/events{/privacy}",
          "followers_url": "https://api.github.com/users/motatoes/followers",
          "following_url": "https://api.github.com/users/motatoes/following{/other_user}",
          "gists_url": "https://api.github.com/users/motatoes/gists{/gist_id}",
          "gravatar_id": "",
          "html_url": "https://github.com/motatoes",
          "id": 1627972,
          "login": "motatoes",
          "node_id": "MDQ6VXNlcjE2Mjc5NzI=",
          "organizations_url": "https://api.github.com/users/motatoes/orgs",
          "received_events_url": "https://api.github.com/users/motatoes/received_events",
          "repos_url": "https://api.github.com/users/motatoes/repos",
          "site_admin": false,
          "starred_url": "https://api.github.com/users/motatoes/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/motatoes/subscriptions",
          "type": "User",
          "url": "https://api.github.com/users/motatoes"
        }
      },
      "repository": {
        "allow_forking": true,
        "archive_url": "https://api.github.com/repos/diggerhq/digger_demo/{archive_format}{/ref}",
        "archived": false,
        "assignees_url": "https://api.github.com/repos/diggerhq/digger_demo/assignees{/user}",
        "blobs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/blobs{/sha}",
        "branches_url": "https://api.github.com/repos/diggerhq/digger_demo/branches{/branch}",
        "clone_url": "https://github.com/diggerhq/digger_demo.git",
        "collaborators_url": "https://api.github.com/repos/diggerhq/digger_demo/collaborators{/collaborator}",
        "comments_url": "https://api.github.com/repos/diggerhq/digger_demo/comments{/number}",
        "commits_url": "https://api.github.com/repos/diggerhq/digger_demo/commits{/sha}",
        "compare_url": "https://api.github.com/repos/diggerhq/digger_demo/compare/{base}...{head}",
        "contents_url": "https://api.github.com/repos/diggerhq/digger_demo/contents/{+path}",
        "contributors_url": "https://api.github.com/repos/diggerhq/digger_demo/contributors",
        "created_at": "2023-02-24T16:35:52Z",
        "default_branch": "main",
        "deployments_url": "https://api.github.com/repos/diggerhq/digger_demo/deployments",
        "description": null,
        "disabled": false,
        "downloads_url": "https://api.github.com/repos/diggerhq/digger_demo/downloads",
        "events_url": "https://api.github.com/repos/diggerhq/digger_demo/events",
        "fork": false,
        "forks": 7,
        "forks_count": 7,
        "forks_url": "https://api.github.com/repos/diggerhq/digger_demo/forks",
        "full_name": "diggerhq/digger_demo",
        "git_commits_url": "https://api.github.com/repos/diggerhq/digger_demo/git/commits{/sha}",
        "git_refs_url": "https://api.github.com/repos/diggerhq/digger_demo/git/refs{/sha}",
        "git_tags_url": "https://api.github.com/repos/diggerhq/digger_demo/git/tags{/sha}",
        "git_url": "git://github.com/diggerhq/digger_demo.git",
        "has_discussions": false,
        "has_downloads": true,
        "has_issues": true,
        "has_pages": false,
        "has_projects": true,
        "has_wiki": true,
        "homepage": null,
        "hooks_url": "https://api.github.com/repos/diggerhq/digger_demo/hooks",
        "html_url": "https://github.com/diggerhq/digger_demo",
        "id": 606119156,
        "is_template": true,
        "issue_comment_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/comments{/number}",
        "issue_events_url": "https://api.github.com/repos/diggerhq/digger_demo/issues/events{/number}",
        "issues_url": "https://api.github.com/repos/diggerhq/digger_demo/issues{/number}",
        "keys_url": "https://api.github.com/repos/diggerhq/digger_demo/keys{/key_id}",
        "labels_url": "https://api.github.com/repos/diggerhq/digger_demo/labels{/name}",
        "language": "HCL",
        "languages_url": "https://api.github.com/repos/diggerhq/digger_demo/languages",
        "license": null,
        "merges_url": "https://api.github.com/repos/diggerhq/digger_demo/merges",
        "milestones_url": "https://api.github.com/repos/diggerhq/digger_demo/milestones{/number}",
        "mirror_url": null,
        "name": "digger_demo",
        "node_id": "R_kgDOJCCk9A",
        "notifications_url": "https://api.github.com/repos/diggerhq/digger_demo/notifications{?since,all,participating}",
        "open_issues": 2,
        "open_issues_count": 2,
        "owner": {
          "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
          "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
          "followers_url": "https://api.github.com/users/diggerhq/followers",
          "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
          "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
          "gravatar_id": "",
          "html_url": "https://github.com/diggerhq",
          "id": 71334590,
          "login": "diggerhq",
          "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
          "organizations_url": "https://api.github.com/users/diggerhq/orgs",
          "received_events_url": "https://api.github.com/users/diggerhq/received_events",
          "repos_url": "https://api.github.com/users/diggerhq/repos",
          "site_admin": false,
          "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
          "type": "Organization",
          "url": "https://api.github.com/users/diggerhq"
        },
        "private": false,
        "pulls_url": "https://api.github.com/repos/diggerhq/digger_demo/pulls{/number}",
        "pushed_at": "2023-03-28T16:41:34Z",
        "releases_url": "https://api.github.com/repos/diggerhq/digger_demo/releases{/id}",
        "size": 51,
        "ssh_url": "git@github.com:diggerhq/digger_demo.git",
        "stargazers_count": 3,
        "stargazers_url": "https://api.github.com/repos/diggerhq/digger_demo/stargazers",
        "statuses_url": "https://api.github.com/repos/diggerhq/digger_demo/statuses/{sha}",
        "subscribers_url": "https://api.github.com/repos/diggerhq/digger_demo/subscribers",
        "subscription_url": "https://api.github.com/repos/diggerhq/digger_demo/subscription",
        "svn_url": "https://github.com/diggerhq/digger_demo",
        "tags_url": "https://api.github.com/repos/diggerhq/digger_demo/tags",
        "teams_url": "https://api.github.com/repos/diggerhq/digger_demo/teams",
        "topics": [],
        "trees_url": "https://api.github.com/repos/diggerhq/digger_demo/git/trees{/sha}",
        "updated_at": "2023-03-15T12:50:14Z",
        "url": "https://api.github.com/repos/diggerhq/digger_demo",
        "visibility": "public",
        "watchers": 3,
        "watchers_count": 3,
        "web_commit_signoff_required": false
      },
      "sender": {
        "avatar_url": "https://avatars.githubusercontent.com/u/1627972?v=4",
        "events_url": "https://api.github.com/users/motatoes/events{/privacy}",
        "followers_url": "https://api.github.com/users/motatoes/followers",
        "following_url": "https://api.github.com/users/motatoes/following{/other_user}",
        "gists_url": "https://api.github.com/users/motatoes/gists{/gist_id}",
        "gravatar_id": "",
        "html_url": "https://github.com/motatoes",
        "id": 1627972,
        "login": "motatoes",
        "node_id": "MDQ6VXNlcjE2Mjc5NzI=",
        "organizations_url": "https://api.github.com/users/motatoes/orgs",
        "received_events_url": "https://api.github.com/users/motatoes/received_events",
        "repos_url": "https://api.github.com/users/motatoes/repos",
        "site_admin": false,
        "starred_url": "https://api.github.com/users/motatoes/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/motatoes/subscriptions",
        "type": "User",
        "url": "https://api.github.com/users/motatoes"
      }
    },
    "server_url": "https://github.com",
    "api_url": "https://api.github.com",
    "graphql_url": "https://api.github.com/graphql",
    "ref_name": "42/merge",
    "ref_protected": false,
    "ref_type": "branch",
    "secret_source": "Actions",
    "workflow_ref": "diggerhq/digger_demo/.github/workflows/plan.yml@refs/pull/42/merge",
    "workflow_sha": "b9ba324329ed046ae07a163cccffb72c60db697c",
    "workspace": "/home/runner/work/digger_demo/digger_demo",
    "action": "__diggerhq_tfrun",
    "event_path": "/home/runner/work/_temp/_github_workflow/event.json",
    "action_repository": "aws-actions/configure-aws-credentials",
    "action_ref": "v1",
    "path": "/home/runner/work/_temp/_runner_file_commands/add_path_e0025c43-224e-452c-a2cf-4198a7299a20",
    "env": "/home/runner/work/_temp/_runner_file_commands/set_env_e0025c43-224e-452c-a2cf-4198a7299a20",
    "step_summary": "/home/runner/work/_temp/_runner_file_commands/step_summary_e0025c43-224e-452c-a2cf-4198a7299a20",
    "state": "/home/runner/work/_temp/_runner_file_commands/save_state_e0025c43-224e-452c-a2cf-4198a7299a20",
    "output": "/home/runner/work/_temp/_runner_file_commands/set_output_e0025c43-224e-452c-a2cf-4198a7299a20"
  }`

func TestHappyPath(t *testing.T) {
	skipCI(t)

	dir := terraform.CreateTestTerraformProject()

	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

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
	splitRepositoryName := strings.Split(parsedNewPullRequestContext.Repository, "/")
	repoOwner, repositoryName := splitRepositoryName[0], splitRepositoryName[1]
	prBranch := parsedNewPullRequestContext.HeadRef
	SHA := parsedNewPullRequestContext.SHA
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// new pr should lock the project
	impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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
	assert.Equal(t, 42, *transactionId, "TransactionId")
	return
	println("--- digger plan comment ---")
	ghEvent = parsedDiggerPlanCommentContext.Event
	eventName = parsedDiggerPlanCommentContext.EventName
	splitRepositoryName = strings.Split(parsedNewPullRequestContext.Repository, "/")
	repoOwner, repositoryName = splitRepositoryName[0], splitRepositoryName[1]
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA

	// 'digger plan' comment should trigger terraform execution
	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	eventName = parsedDiggerApplyCommentContext.EventName
	splitRepositoryName = strings.Split(parsedNewPullRequestContext.Repository, "/")
	repoOwner, repositoryName = splitRepositoryName[0], splitRepositoryName[1]
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA

	// 'digger apply' comment should trigger terraform execution and unlock the project
	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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
	splitRepositoryName = strings.Split(parsedNewPullRequestContext.Repository, "/")
	repoOwner, repositoryName = splitRepositoryName[0], splitRepositoryName[1]
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA

	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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

	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

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
	prBranch := parsedNewPullRequestContext.HeadRef
	SHA := parsedNewPullRequestContext.SHA
	repositoryName := parsedNewPullRequestContext.Repository
	githubPrService := github.NewGithubPullRequestService(ghToken, repositoryName, repoOwner)

	assert.Equal(t, "pull_request", parsedNewPullRequestContext.EventName)

	// no files changed, no locks
	impactedProjects, prNumber, err := digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err := digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA
	repositoryName = parsedDiggerPlanCommentContext.Repository

	// 'digger plan' comment should trigger terraform execution
	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
	assert.NoError(t, err)

	println("--- digger apply comment ---")
	ghEvent = parsedDiggerApplyCommentContext.Event
	eventName = parsedDiggerApplyCommentContext.EventName
	repoOwner = parsedDiggerApplyCommentContext.RepositoryOwner
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA
	repositoryName = parsedDiggerApplyCommentContext.Repository

	// 'digger apply' comment should trigger terraform execution and unlock the project
	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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
	prBranch = parsedNewPullRequestContext.HeadRef
	SHA = parsedNewPullRequestContext.SHA
	repositoryName = parsedDiggerUnlockCommentContext.Repository

	impactedProjects, prNumber, err = digger.ProcessGitHubEvent(ghEvent, diggerConfig, githubPrService)
	assert.NoError(t, err)
	commandsToRunPerProject, err = digger.ConvertGithubEventToCommands(ghEvent, impactedProjects)
	assert.NoError(t, err)
	err = digger.RunCommandsPerProject(commandsToRunPerProject, prBranch, SHA, repoOwner, repositoryName, eventName, prNumber, diggerConfig, githubPrService, &dynamoDbLock, dir)
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
