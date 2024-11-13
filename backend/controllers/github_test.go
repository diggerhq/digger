package controllers

import (
	"encoding/json"
	orchestrator "github.com/diggerhq/digger/libs/scheduler"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/google/go-github/v61/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var issueCommentPayload = `{
  "action": "created",
  "issue": {
    "url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2",
    "repository_url": "https://api.github.com/repos/diggerhq/github-job-scheduler",
    "labels_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2/labels{/name}",
    "comments_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2/comments",
    "events_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2/events",
    "html_url": "https://github.com/diggerhq/github-job-scheduler/pull/2",
    "id": 1882391909,
    "node_id": "33333",
    "number": 2,
    "title": "Update main.tf",
    "user": {
      "login": "veziak",
      "id": 2407061,
      "node_id": "4444=",
      "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/veziak",
      "html_url": "https://github.com/veziak",
      "followers_url": "https://api.github.com/users/veziak/followers",
      "following_url": "https://api.github.com/users/veziak/following{/other_user}",
      "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
      "organizations_url": "https://api.github.com/users/veziak/orgs",
      "repos_url": "https://api.github.com/users/veziak/repos",
      "events_url": "https://api.github.com/users/veziak/events{/privacy}",
      "received_events_url": "https://api.github.com/users/veziak/received_events",
      "type": "User",
      "site_admin": false
    },
    "labels": [
    ],
    "state": "open",
    "locked": false,
    "assignee": null,
    "assignees": [

    ],
    "milestone": null,
    "comments": 2,
    "created_at": "2023-09-05T16:53:52Z",
    "updated_at": "2023-09-11T14:33:42Z",
    "closed_at": null,
    "author_association": "CONTRIBUTOR",
    "active_lock_reason": null,
    "draft": false,
    "pull_request": {
      "url": "https://api.github.com/repos/diggerhq/github-job-scheduler/pulls/2",
      "html_url": "https://github.com/diggerhq/github-job-scheduler/pull/2",
      "diff_url": "https://github.com/diggerhq/github-job-scheduler/pull/2.diff",
      "patch_url": "https://github.com/diggerhq/github-job-scheduler/pull/2.patch",
      "merged_at": null
    },
    "body": null,
    "reactions": {
      "url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2/reactions",
      "total_count": 0,
      "+1": 0,
      "-1": 0,
      "laugh": 0,
      "hooray": 0,
      "confused": 0,
      "heart": 0,
      "rocket": 0,
      "eyes": 0
    },
    "timeline_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2/timeline",
    "performed_via_github_app": null,
    "state_reason": null
  },
  "comment": {
    "url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/comments/1714014480",
    "html_url": "https://github.com/diggerhq/github-job-scheduler/pull/2#issuecomment-1714014480",
    "issue_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/2",
    "id": 1714014480,
    "node_id": "44444",
    "user": {
      "login": "veziak",
      "id": 2407061,
      "node_id": "33333=",
      "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/veziak",
      "html_url": "https://github.com/veziak",
      "followers_url": "https://api.github.com/users/veziak/followers",
      "following_url": "https://api.github.com/users/veziak/following{/other_user}",
      "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
      "organizations_url": "https://api.github.com/users/veziak/orgs",
      "repos_url": "https://api.github.com/users/veziak/repos",
      "events_url": "https://api.github.com/users/veziak/events{/privacy}",
      "received_events_url": "https://api.github.com/users/veziak/received_events",
      "type": "User",
      "site_admin": false
    },
    "created_at": "2023-09-11T14:33:42Z",
    "updated_at": "2023-09-11T14:33:42Z",
    "author_association": "CONTRIBUTOR",
    "body": "digger plan",
    "reactions": {
      "url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/comments/1714014480/reactions",
      "total_count": 0,
      "+1": 0,
      "-1": 0,
      "laugh": 0,
      "hooray": 0,
      "confused": 0,
      "heart": 0,
      "rocket": 0,
      "eyes": 0
    },
    "performed_via_github_app": null
  },
  "repository": {
    "id": 686968600,
    "node_id": "222222",
    "name": "github-job-scheduler",
    "full_name": "diggerhq/github-job-scheduler",
    "private": true,
    "owner": {
      "login": "diggerhq",
      "id": 71334590,
      "node_id": "333",
      "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/diggerhq",
      "html_url": "https://github.com/diggerhq",
      "followers_url": "https://api.github.com/users/diggerhq/followers",
      "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
      "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
      "organizations_url": "https://api.github.com/users/diggerhq/orgs",
      "repos_url": "https://api.github.com/users/diggerhq/repos",
      "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
      "received_events_url": "https://api.github.com/users/diggerhq/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "html_url": "https://github.com/diggerhq/github-job-scheduler",
    "description": null,
    "fork": false,
    "url": "https://api.github.com/repos/diggerhq/github-job-scheduler",
    "forks_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/forks",
    "keys_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/keys{/key_id}",
    "collaborators_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/collaborators{/collaborator}",
    "teams_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/teams",
    "hooks_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/hooks",
    "issue_events_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/events{/number}",
    "events_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/events",
    "assignees_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/assignees{/user}",
    "branches_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/branches{/branch}",
    "tags_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/tags",
    "blobs_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/git/blobs{/sha}",
    "git_tags_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/git/tags{/sha}",
    "git_refs_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/git/refs{/sha}",
    "trees_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/git/trees{/sha}",
    "statuses_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/statuses/{sha}",
    "languages_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/languages",
    "stargazers_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/stargazers",
    "contributors_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/contributors",
    "subscribers_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/subscribers",
    "subscription_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/subscription",
    "commits_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/commits{/sha}",
    "git_commits_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/git/commits{/sha}",
    "comments_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/comments{/number}",
    "issue_comment_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues/comments{/number}",
    "contents_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/contents/{+path}",
    "compare_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/compare/{base}...{head}",
    "merges_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/merges",
    "archive_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/{archive_format}{/ref}",
    "downloads_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/downloads",
    "issues_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/issues{/number}",
    "pulls_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/pulls{/number}",
    "milestones_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/milestones{/number}",
    "notifications_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/notifications{?since,all,participating}",
    "labels_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/labels{/name}",
    "releases_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/releases{/id}",
    "deployments_url": "https://api.github.com/repos/diggerhq/github-job-scheduler/deployments",
    "created_at": "2023-09-04T10:29:28Z",
    "updated_at": "2023-09-05T16:06:16Z",
    "pushed_at": "2023-09-06T17:02:35Z",
    "git_url": "git://github.com/diggerhq/github-job-scheduler.git",
    "ssh_url": "git@github.com:diggerhq/github-job-scheduler.git",
    "clone_url": "https://github.com/diggerhq/github-job-scheduler.git",
    "svn_url": "https://github.com/diggerhq/github-job-scheduler",
    "homepage": null,
    "size": 9,
    "stargazers_count": 0,
    "watchers_count": 0,
    "language": "HCL",
    "has_issues": true,
    "has_projects": true,
    "has_downloads": true,
    "has_wiki": true,
    "has_pages": false,
    "has_discussions": false,
    "forks_count": 0,
    "mirror_url": null,
    "archived": false,
    "disabled": false,
    "open_issues_count": 1,
    "license": null,
    "allow_forking": false,
    "is_template": false,
    "web_commit_signoff_required": false,
    "topics": [

    ],
    "visibility": "private",
    "forks": 0,
    "open_issues": 1,
    "watchers": 0,
    "default_branch": "main"
  },
  "organization": {
    "login": "diggerhq",
    "id": 71334590,
    "node_id": "2222",
    "url": "https://api.github.com/orgs/diggerhq",
    "repos_url": "https://api.github.com/orgs/diggerhq/repos",
    "events_url": "https://api.github.com/orgs/diggerhq/events",
    "hooks_url": "https://api.github.com/orgs/diggerhq/hooks",
    "issues_url": "https://api.github.com/orgs/diggerhq/issues",
    "members_url": "https://api.github.com/orgs/diggerhq/members{/member}",
    "public_members_url": "https://api.github.com/orgs/diggerhq/public_members{/member}",
    "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
    "description": ""
  },
  "sender": {
    "login": "veziak",
    "id": 2407061,
    "node_id": "2222=",
    "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
    "gravatar_id": "",
    "url": "https://api.github.com/users/veziak",
    "html_url": "https://github.com/veziak",
    "followers_url": "https://api.github.com/users/veziak/followers",
    "following_url": "https://api.github.com/users/veziak/following{/other_user}",
    "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
    "organizations_url": "https://api.github.com/users/veziak/orgs",
    "repos_url": "https://api.github.com/users/veziak/repos",
    "events_url": "https://api.github.com/users/veziak/events{/privacy}",
    "received_events_url": "https://api.github.com/users/veziak/received_events",
    "type": "User",
    "site_admin": false
  },
  "installation": {
    "id": 41584295,
    "node_id": "111"
  }
}`

var installationRepositoriesAddedPayload = `{
  "action": "added",
  "installation": {
    "id": 41584295,
    "account": {
      "login": "diggerhq",
      "id": 71334590,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
      "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/diggerhq",
      "html_url": "https://github.com/diggerhq",
      "followers_url": "https://api.github.com/users/diggerhq/followers",
      "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
      "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
      "organizations_url": "https://api.github.com/users/diggerhq/orgs",
      "repos_url": "https://api.github.com/users/diggerhq/repos",
      "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
      "received_events_url": "https://api.github.com/users/diggerhq/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "repository_selection": "selected",
    "access_tokens_url": "https://api.github.com/app/installations/41584295/access_tokens",
    "repositories_url": "https://api.github.com/installation/repositories",
    "html_url": "https://github.com/organizations/diggerhq/settings/installations/41584295",
    "app_id": 360162,
    "app_slug": "digger-cloud-test-app",
    "target_id": 71334590,
    "target_type": "Organization",
    "permissions": {
      "issues": "write",
      "actions": "write",
      "secrets": "read",
      "metadata": "read",
      "statuses": "read",
      "workflows": "write",
      "pull_requests": "write",
      "actions_variables": "read"
    },
    "events": [
      "issues",
      "issue_comment",
      "pull_request",
      "pull_request_review",
      "pull_request_review_comment",
      "pull_request_review_thread",
      "status",
      "workflow_job"
    ],
    "created_at": "2023-09-08T11:34:17.000+01:00",
    "updated_at": "2023-09-18T11:29:18.000+01:00",
    "single_file_name": null,
    "has_multiple_single_files": false,
    "single_file_paths": [

    ],
    "suspended_by": null,
    "suspended_at": null
  },
  "repository_selection": "selected",
  "repositories_added": [
    {
      "id": 436580100,
      "node_id": "R_kgDOGgWvBA",
      "name": "test-github-action",
      "full_name": "diggerhq/test-github-action",
      "private": true
    }
  ],
  "repositories_removed": [

  ],
  "requester": null,
  "sender": {
    "login": "veziak",
    "id": 2407061,
    "node_id": "MDQ6VXNlcjI0MDcwNjE=",
    "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
    "gravatar_id": "",
    "url": "https://api.github.com/users/veziak",
    "html_url": "https://github.com/veziak",
    "followers_url": "https://api.github.com/users/veziak/followers",
    "following_url": "https://api.github.com/users/veziak/following{/other_user}",
    "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
    "organizations_url": "https://api.github.com/users/veziak/orgs",
    "repos_url": "https://api.github.com/users/veziak/repos",
    "events_url": "https://api.github.com/users/veziak/events{/privacy}",
    "received_events_url": "https://api.github.com/users/veziak/received_events",
    "type": "User",
    "site_admin": false
  }
}`

var installationRepositoriesDeletedPayload = `{
  "action": "removed",
  "installation": {
    "id": 41584295,
    "account": {
      "login": "diggerhq",
      "id": 71334590,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
      "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/diggerhq",
      "html_url": "https://github.com/diggerhq",
      "followers_url": "https://api.github.com/users/diggerhq/followers",
      "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
      "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
      "organizations_url": "https://api.github.com/users/diggerhq/orgs",
      "repos_url": "https://api.github.com/users/diggerhq/repos",
      "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
      "received_events_url": "https://api.github.com/users/diggerhq/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "repository_selection": "selected",
    "access_tokens_url": "https://api.github.com/app/installations/41584295/access_tokens",
    "repositories_url": "https://api.github.com/installation/repositories",
    "html_url": "https://github.com/organizations/diggerhq/settings/installations/41584295",
    "app_id": 360162,
    "app_slug": "digger-cloud-test-app",
    "target_id": 71334590,
    "target_type": "Organization",
    "permissions": {
      "issues": "write",
      "actions": "write",
      "secrets": "read",
      "metadata": "read",
      "statuses": "read",
      "workflows": "write",
      "pull_requests": "write",
      "actions_variables": "read"
    },
    "events": [
      "issues",
      "issue_comment",
      "pull_request",
      "pull_request_review",
      "pull_request_review_comment",
      "pull_request_review_thread",
      "status",
      "workflow_job"
    ],
    "created_at": "2023-09-08T10:34:17.000Z",
    "updated_at": "2023-09-18T11:23:48.000Z",
    "single_file_name": null,
    "has_multiple_single_files": false,
    "single_file_paths": [

    ],
    "suspended_by": null,
    "suspended_at": null
  },
  "repository_selection": "selected",
  "repositories_added": [

  ],
  "repositories_removed": [
    {
      "id": 436580100,
      "node_id": "R_kgDOGgWvBA",
      "name": "test-github-action",
      "full_name": "diggerhq/test-github-action",
      "private": true
    }
  ],
  "requester": null,
  "sender": {
    "login": "veziak",
    "id": 2407061,
    "node_id": "MDQ6VXNlcjI0MDcwNjE=",
    "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
    "gravatar_id": "",
    "url": "https://api.github.com/users/veziak",
    "html_url": "https://github.com/veziak",
    "followers_url": "https://api.github.com/users/veziak/followers",
    "following_url": "https://api.github.com/users/veziak/following{/other_user}",
    "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
    "organizations_url": "https://api.github.com/users/veziak/orgs",
    "repos_url": "https://api.github.com/users/veziak/repos",
    "events_url": "https://api.github.com/users/veziak/events{/privacy}",
    "received_events_url": "https://api.github.com/users/veziak/received_events",
    "type": "User",
    "site_admin": false
  }
}`

var installationCreatedEvent = `
{
  "action": "created",
  "installation": {
    "id": 41584295,
    "account": {
      "login": "diggerhq",
      "id": 71334590,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjcxMzM0NTkw",
      "avatar_url": "https://avatars.githubusercontent.com/u/71334590?v=4",
      "gravatar_id": "",
      "url": "https://api.github.com/users/diggerhq",
      "html_url": "https://github.com/diggerhq",
      "followers_url": "https://api.github.com/users/diggerhq/followers",
      "following_url": "https://api.github.com/users/diggerhq/following{/other_user}",
      "gists_url": "https://api.github.com/users/diggerhq/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/diggerhq/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/diggerhq/subscriptions",
      "organizations_url": "https://api.github.com/users/diggerhq/orgs",
      "repos_url": "https://api.github.com/users/diggerhq/repos",
      "events_url": "https://api.github.com/users/diggerhq/events{/privacy}",
      "received_events_url": "https://api.github.com/users/diggerhq/received_events",
      "type": "Organization",
      "site_admin": false
    },
    "repository_selection": "selected",
    "access_tokens_url": "https://api.github.com/app/installations/41584295/access_tokens",
    "repositories_url": "https://api.github.com/installation/repositories",
    "html_url": "https://github.com/organizations/diggerhq/settings/installations/41584295",
    "app_id": 392316,
    "app_slug": "digger-cloud",
    "target_id": 71334590,
    "target_type": "Organization",
    "permissions": {
      "issues": "write",
      "actions": "write",
      "secrets": "read",
      "metadata": "read",
      "statuses": "read",
      "workflows": "write",
      "pull_requests": "write",
      "actions_variables": "read"
    },
    "events": [
      "issues",
      "issue_comment",
      "pull_request",
      "pull_request_review",
      "pull_request_review_comment",
      "pull_request_review_thread",
      "status"
    ],
    "created_at": "2023-09-26T14:49:27.000+01:00",
    "updated_at": "2023-09-26T14:49:28.000+01:00",
    "single_file_name": null,
    "has_multiple_single_files": false,
    "single_file_paths": [

    ],
    "suspended_by": null,
    "suspended_at": null
  },
  "repositories": [
    {
      "id": 696378594,
      "node_id": "R_kgDOKYHk4g",
      "name": "parallel_jobs_demo",
      "full_name": "diggerhq/parallel_jobs_demo",
      "private": false
    }
  ],
  "requester": null,
  "sender": {
    "login": "motatoes",
    "id": 1627972,
    "node_id": "MDQ6VXNlcjE2Mjc5NzI=",
    "avatar_url": "https://avatars.githubusercontent.com/u/1627972?v=4",
    "gravatar_id": "",
    "url": "https://api.github.com/users/motatoes",
    "html_url": "https://github.com/motatoes",
    "followers_url": "https://api.github.com/users/motatoes/followers",
    "following_url": "https://api.github.com/users/motatoes/following{/other_user}",
    "gists_url": "https://api.github.com/users/motatoes/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/motatoes/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/motatoes/subscriptions",
    "organizations_url": "https://api.github.com/users/motatoes/orgs",
    "repos_url": "https://api.github.com/users/motatoes/repos",
    "events_url": "https://api.github.com/users/motatoes/events{/privacy}",
    "received_events_url": "https://api.github.com/users/motatoes/received_events",
    "type": "User",
    "site_admin": false
  }
}`

func setupSuite(tb testing.TB) (func(tb testing.TB), *models.Database) {
	log.Println("setup suite")

	// database file name
	dbName := "database_test.db"

	// remove old database
	e := os.Remove(dbName)
	if e != nil {
		if !strings.Contains(e.Error(), "no such file or directory") {
			log.Fatal(e)
		}
	}

	// open and create a new database
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// migrate tables
	err = gdb.AutoMigrate(&models.Policy{}, &models.Organisation{}, &models.Repo{}, &models.Project{}, &models.Token{},
		&models.User{}, &models.ProjectRun{}, &models.GithubAppInstallation{}, &models.GithubAppConnection{}, &models.GithubAppInstallationLink{},
		&models.GithubDiggerJobLink{}, &models.DiggerJob{}, &models.DiggerJobParentLink{}, &models.JobToken{})
	if err != nil {
		log.Fatal(err)
	}

	database := &models.Database{GormDB: gdb}
	models.DB = database

	// create an org
	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId)
	if err != nil {
		log.Fatal(err)
	}

	// create digger repo
	repoName := "test repo"
	repo, err := database.CreateRepo(repoName, "", "", "", "", org, "")
	if err != nil {
		log.Fatal(err)
	}

	// create test project
	projectName := "test project"
	_, err = database.CreateProject(projectName, org, repo, false, false)
	if err != nil {
		log.Fatal(err)
	}

	// create installation for issueComment payload
	var payload github.IssueCommentEvent
	err = json.Unmarshal([]byte(issueCommentPayload), &payload)
	if err != nil {
		log.Fatal(err)
	}
	installationId := *payload.Installation.ID

	_, err = database.CreateGithubInstallationLink(org, installationId)
	if err != nil {
		log.Fatal(err)
	}

	githubAppId := int64(1)
	login := "test"
	accountId := 1
	repoFullName := "diggerhq/github-job-scheduler"
	_, err = database.CreateGithubAppInstallation(installationId, githubAppId, login, accountId, repoFullName)
	if err != nil {
		log.Fatal(err)
	}

	diggerConfig := `projects:
- name: dev
  dir: dev
  workflow: default
- name: prod
  dir: prod
  workflow: default
  depends_on: ["dev"]
`

	diggerRepoName := strings.Replace(repoFullName, "/", "-", 1)
	_, err = database.CreateRepo(diggerRepoName, "", "", "", "", org, diggerConfig)
	if err != nil {
		log.Fatal(err)
	}

	models.DB = database
	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		err = os.Remove(dbName)
		if err != nil {
			log.Fatal(err)
		}
	}, database
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}
func TestGithubHandleIssueCommentEvent(t *testing.T) {
	t.Skip("!!TODO: Fix this failing test and unskip it")
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	files := make([]github.CommitFile, 2)
	files[0] = github.CommitFile{Filename: github.String("prod/main.tf")}
	files[1] = github.CommitFile{Filename: github.String("dev/main.tf")}
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposPullsByOwnerByRepoByPullNumber,
			github.PullRequest{
				Number: github.Int(1),
				Head:   &github.PullRequestBranch{Ref: github.String("main")},
			},
		),
		mock.WithRequestMatch(
			mock.GetReposPullsFilesByOwnerByRepoByPullNumber,
			files,
		),
		mock.WithRequestMatch(
			mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
			nil,
		),
	)

	gh := &utils.DiggerGithubClientMockProvider{}
	gh.MockedHTTPClient = mockedHTTPClient

	var payload github.IssueCommentEvent
	err := json.Unmarshal([]byte(issueCommentPayload), &payload)
	assert.NoError(t, err)
	err = handleIssueCommentEvent(gh, &payload, nil, 0, make([]IssueCommentHook, 0))
	assert.NoError(t, err)

	jobs, err := models.DB.GetPendingParentDiggerJobs(nil)
	assert.Equal(t, 0, len(jobs))
}

func TestJobsTreeWithOneJobsAndTwoProjects(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	jobs := make(map[string]orchestrator.Job)
	jobs["dev"] = orchestrator.Job{ProjectName: "dev"}

	var projects []configuration.Project
	project1 := configuration.Project{Name: "dev"}
	project2 := configuration.Project{Name: "prod", DependencyProjects: []string{"dev"}}
	projects = append(projects, project1, project2)

	projectMap := make(map[string]configuration.Project)
	projectMap["dev"] = project1

	graph, err := configuration.CreateProjectDependencyGraph(projects)
	assert.NoError(t, err)

	_, result, err := utils.ConvertJobsToDiggerJobs("", "github", 1, jobs, projectMap, graph, 41584295, "", 2, "diggerhq", "parallel_jobs_demo", "diggerhq/parallel_jobs_demo", "", 123, "test", 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result))
	parentLinks, err := models.DB.GetDiggerJobParentLinksChildId(&result["dev"].DiggerJobID)
	assert.NoError(t, err)
	assert.Empty(t, parentLinks)
	assert.NotContains(t, result, "prod")
}

func TestJobsTreeWithTwoDependantJobs(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	jobs := make(map[string]orchestrator.Job)
	jobs["dev"] = orchestrator.Job{ProjectName: "dev"}
	jobs["prod"] = orchestrator.Job{ProjectName: "prod"}

	var projects []configuration.Project
	project1 := configuration.Project{Name: "dev"}
	project2 := configuration.Project{Name: "prod", DependencyProjects: []string{"dev"}}
	projects = append(projects, project1, project2)

	graph, err := configuration.CreateProjectDependencyGraph(projects)
	assert.NoError(t, err)

	projectMap := make(map[string]configuration.Project)
	projectMap["dev"] = project1
	projectMap["prod"] = project2

	_, result, err := utils.ConvertJobsToDiggerJobs("", "github", 1, jobs, projectMap, graph, 123, "", 2, "", "", "test", "", 123, "test", 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result))

	parentLinks, err := models.DB.GetDiggerJobParentLinksChildId(&result["dev"].DiggerJobID)
	assert.NoError(t, err)
	assert.Empty(t, parentLinks)
	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["prod"].DiggerJobID)
	assert.NoError(t, err)

	assert.Equal(t, result["dev"].DiggerJobID, parentLinks[0].ParentDiggerJobId)
}

func TestJobsTreeWithTwoIndependentJobs(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	jobs := make(map[string]orchestrator.Job)
	jobs["dev"] = orchestrator.Job{ProjectName: "dev"}
	jobs["prod"] = orchestrator.Job{ProjectName: "prod"}

	var projects []configuration.Project
	project1 := configuration.Project{Name: "dev"}
	project2 := configuration.Project{Name: "prod"}
	projects = append(projects, project1, project2)

	graph, err := configuration.CreateProjectDependencyGraph(projects)
	assert.NoError(t, err)

	projectMap := make(map[string]configuration.Project)
	projectMap["dev"] = project1
	projectMap["prod"] = project2

	_, result, err := utils.ConvertJobsToDiggerJobs("", "github", 1, jobs, projectMap, graph, 123, "", 2, "", "", "test", "", 123, "test", 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(result))
	parentLinks, err := models.DB.GetDiggerJobParentLinksChildId(&result["dev"].DiggerJobID)
	assert.NoError(t, err)
	assert.Empty(t, parentLinks)
	assert.NotNil(t, result["dev"].SerializedJobSpec)
	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["prod"].DiggerJobID)
	assert.NoError(t, err)
	assert.Empty(t, parentLinks)
	assert.NotNil(t, result["prod"].SerializedJobSpec)
}

func TestJobsTreeWithThreeLevels(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	jobs := make(map[string]orchestrator.Job)
	jobs["111"] = orchestrator.Job{ProjectName: "111"}
	jobs["222"] = orchestrator.Job{ProjectName: "222"}
	jobs["333"] = orchestrator.Job{ProjectName: "333"}
	jobs["444"] = orchestrator.Job{ProjectName: "444"}
	jobs["555"] = orchestrator.Job{ProjectName: "555"}
	jobs["666"] = orchestrator.Job{ProjectName: "666"}

	var projects []configuration.Project
	project1 := configuration.Project{Name: "111"}
	project2 := configuration.Project{Name: "222", DependencyProjects: []string{"111"}}
	project3 := configuration.Project{Name: "333", DependencyProjects: []string{"111"}}
	project4 := configuration.Project{Name: "444", DependencyProjects: []string{"222"}}
	project5 := configuration.Project{Name: "555", DependencyProjects: []string{"222"}}
	project6 := configuration.Project{Name: "666", DependencyProjects: []string{"333"}}
	projects = append(projects, project1, project2, project3, project4, project5, project6)

	graph, err := configuration.CreateProjectDependencyGraph(projects)
	assert.NoError(t, err)

	projectMap := make(map[string]configuration.Project)
	projectMap["111"] = project1
	projectMap["222"] = project2
	projectMap["333"] = project3
	projectMap["444"] = project4
	projectMap["555"] = project5
	projectMap["666"] = project6

	_, result, err := utils.ConvertJobsToDiggerJobs("", "github", 1, jobs, projectMap, graph, 123, "", 2, "", "", "test", "", 123, "test", 0)
	assert.NoError(t, err)
	assert.Equal(t, 6, len(result))
	parentLinks, err := models.DB.GetDiggerJobParentLinksChildId(&result["111"].DiggerJobID)
	assert.NoError(t, err)
	assert.Empty(t, parentLinks)

	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["222"].DiggerJobID)
	assert.NoError(t, err)
	assert.Equal(t, result["111"].DiggerJobID, parentLinks[0].ParentDiggerJobId)

	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["333"].DiggerJobID)
	assert.NoError(t, err)
	assert.Equal(t, result["111"].DiggerJobID, parentLinks[0].ParentDiggerJobId)

	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["444"].DiggerJobID)
	assert.NoError(t, err)
	assert.Equal(t, result["222"].DiggerJobID, parentLinks[0].ParentDiggerJobId)

	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["555"].DiggerJobID)
	assert.NoError(t, err)
	assert.Equal(t, result["222"].DiggerJobID, parentLinks[0].ParentDiggerJobId)

	parentLinks, err = models.DB.GetDiggerJobParentLinksChildId(&result["666"].DiggerJobID)
	assert.NoError(t, err)
	assert.Equal(t, result["333"].DiggerJobID, parentLinks[0].ParentDiggerJobId)
}
