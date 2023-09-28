package main

import (
	"digger/pkg/digger"
	"digger/pkg/github/models"
	ghmodels "digger/pkg/github/models"
	"digger/pkg/reporting"
	"digger/pkg/utils"
	"log"

	dggithub "github.com/diggerhq/lib-orchestrator/github"
	"github.com/google/go-github/v55/github"

	"testing"

	configuration "github.com/diggerhq/lib-digger-config"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var githubContextNewPullRequestJson = `{
    "token": "***",
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
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/comments"
          },
          "commits": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11/commits"
          },
          "html": {
            "href": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11"
          },
          "issue": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11"
          },
          "review_comment": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/comments{/number}"
          },
          "review_comments": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11/comments"
          },
          "self": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11"
          },
          "statuses": {
            "href": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/9d10ac8489bf70e466061f1042cde50db6027ffd"
          }
        },
        "active_lock_reason": null,
        "additions": 0,
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
            "archive_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/{archive_format}{/ref}",
            "archived": false,
            "assignees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/assignees{/user}",
            "blobs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/blobs{/sha}",
            "branches_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/branches{/branch}",
            "clone_url": "https://github.com/diggerhq/tfrun_demo_multienv.git",
            "collaborators_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/collaborators{/collaborator}",
            "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/comments{/number}",
            "commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/commits{/sha}",
            "compare_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/compare/{base}...{head}",
            "contents_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contents/{+path}",
            "contributors_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contributors",
            "created_at": "2023-03-08T11:06:31Z",
            "default_branch": "main",
            "delete_branch_on_merge": false,
            "deployments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/deployments",
            "description": null,
            "disabled": false,
            "downloads_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/downloads",
            "events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/events",
            "fork": false,
            "forks": 2,
            "forks_count": 2,
            "forks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/forks",
            "full_name": "diggerhq/tfrun_demo_multienv",
            "git_commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/commits{/sha}",
            "git_refs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/refs{/sha}",
            "git_tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/tags{/sha}",
            "git_url": "git://github.com/diggerhq/tfrun_demo_multienv.git",
            "has_discussions": false,
            "has_downloads": true,
            "has_issues": true,
            "has_pages": false,
            "has_projects": true,
            "has_wiki": true,
            "homepage": null,
            "hooks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/hooks",
            "html_url": "https://github.com/diggerhq/tfrun_demo_multienv",
            "id": 611213652,
            "is_template": false,
            "issue_comment_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments{/number}",
            "issue_events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/events{/number}",
            "issues_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues{/number}",
            "keys_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/keys{/key_id}",
            "labels_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/labels{/name}",
            "language": "HCL",
            "languages_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/languages",
            "license": null,
            "merge_commit_message": "PR_TITLE",
            "merge_commit_title": "MERGE_MESSAGE",
            "merges_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/merges",
            "milestones_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/milestones{/number}",
            "mirror_url": null,
            "name": "tfrun_demo_multienv",
            "node_id": "R_kgDOJG5hVA",
            "notifications_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/notifications{?since,all,participating}",
            "open_issues": 5,
            "open_issues_count": 5,
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
            "pulls_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls{/number}",
            "pushed_at": "2023-03-10T14:09:35Z",
            "releases_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/releases{/id}",
            "size": 22,
            "squash_merge_commit_message": "COMMIT_MESSAGES",
            "squash_merge_commit_title": "COMMIT_OR_PR_TITLE",
            "ssh_url": "git@github.com:diggerhq/tfrun_demo_multienv.git",
            "stargazers_count": 0,
            "stargazers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/stargazers",
            "statuses_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/{sha}",
            "subscribers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscribers",
            "subscription_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscription",
            "svn_url": "https://github.com/diggerhq/tfrun_demo_multienv",
            "tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/tags",
            "teams_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/teams",
            "topics": [],
            "trees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/trees{/sha}",
            "updated_at": "2023-03-08T11:18:18Z",
            "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv",
            "use_squash_pr_title_as_default": false,
            "visibility": "public",
            "watchers": 0,
            "watchers_count": 0,
            "web_commit_signoff_required": false
          },
          "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
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
            "allow_merge_commit": true,
            "allow_rebase_merge": true,
            "allow_squash_merge": true,
            "allow_update_branch": false,
            "archive_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/{archive_format}{/ref}",
            "archived": false,
            "assignees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/assignees{/user}",
            "blobs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/blobs{/sha}",
            "branches_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/branches{/branch}",
            "clone_url": "https://github.com/diggerhq/tfrun_demo_multienv.git",
            "collaborators_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/collaborators{/collaborator}",
            "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/comments{/number}",
            "commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/commits{/sha}",
            "compare_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/compare/{base}...{head}",
            "contents_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contents/{+path}",
            "contributors_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contributors",
            "created_at": "2023-03-08T11:06:31Z",
            "default_branch": "main",
            "delete_branch_on_merge": false,
            "deployments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/deployments",
            "description": null,
            "disabled": false,
            "downloads_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/downloads",
            "events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/events",
            "fork": false,
            "forks": 2,
            "forks_count": 2,
            "forks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/forks",
            "full_name": "diggerhq/tfrun_demo_multienv",
            "git_commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/commits{/sha}",
            "git_refs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/refs{/sha}",
            "git_tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/tags{/sha}",
            "git_url": "git://github.com/diggerhq/tfrun_demo_multienv.git",
            "has_discussions": false,
            "has_downloads": true,
            "has_issues": true,
            "has_pages": false,
            "has_projects": true,
            "has_wiki": true,
            "homepage": null,
            "hooks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/hooks",
            "html_url": "https://github.com/diggerhq/tfrun_demo_multienv",
            "id": 611213652,
            "is_template": false,
            "issue_comment_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments{/number}",
            "issue_events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/events{/number}",
            "issues_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues{/number}",
            "keys_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/keys{/key_id}",
            "labels_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/labels{/name}",
            "language": "HCL",
            "languages_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/languages",
            "license": null,
            "merge_commit_message": "PR_TITLE",
            "merge_commit_title": "MERGE_MESSAGE",
            "merges_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/merges",
            "milestones_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/milestones{/number}",
            "mirror_url": null,
            "name": "tfrun_demo_multienv",
            "node_id": "R_kgDOJG5hVA",
            "notifications_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/notifications{?since,all,participating}",
            "open_issues": 5,
            "open_issues_count": 5,
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
            "pulls_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls{/number}",
            "pushed_at": "2023-03-10T14:09:35Z",
            "releases_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/releases{/id}",
            "size": 22,
            "squash_merge_commit_message": "COMMIT_MESSAGES",
            "squash_merge_commit_title": "COMMIT_OR_PR_TITLE",
            "ssh_url": "git@github.com:diggerhq/tfrun_demo_multienv.git",
            "stargazers_count": 0,
            "stargazers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/stargazers",
            "statuses_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/{sha}",
            "subscribers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscribers",
            "subscription_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscription",
            "svn_url": "https://github.com/diggerhq/tfrun_demo_multienv",
            "tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/tags",
            "teams_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/teams",
            "topics": [],
            "trees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/trees{/sha}",
            "updated_at": "2023-03-08T11:18:18Z",
            "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv",
            "use_squash_pr_title_as_default": false,
            "visibility": "public",
            "watchers": 0,
            "watchers_count": 0,
            "web_commit_signoff_required": false
          },
          "sha": "9d10ac8489bf70e466061f1042cde50db6027ffd",
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
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11",
        "id": 1271219596,
        "issue_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
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
        "node_id": "PR_kwDOJG5hVM5LxUWM",
        "number": 11,
        "patch_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11.patch",
        "rebaseable": null,
        "requested_reviewers": [],
        "requested_teams": [],
        "review_comment_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/comments{/number}",
        "review_comments": 0,
        "review_comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11/comments",
        "state": "open",
        "statuses_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/9d10ac8489bf70e466061f1042cde50db6027ffd",
        "title": "trigger deploy",
        "updated_at": "2023-03-10T14:09:35Z",
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11",
        "user": {
          "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
          "events_url": "https://api.github.com/users/veziak/events{/privacy}",
          "followers_url": "https://api.github.com/users/veziak/followers",
          "following_url": "https://api.github.com/users/veziak/following{/other_user}",
          "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
          "gravatar_id": "",
          "html_url": "https://github.com/veziak",
          "id": 2407061,
          "login": "veziak",
          "node_id": "MDQ6VXNlcjI0MDcwNjE=",
          "organizations_url": "https://api.github.com/users/veziak/orgs",
          "received_events_url": "https://api.github.com/users/veziak/received_events",
          "repos_url": "https://api.github.com/users/veziak/repos",
          "site_admin": false,
          "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
          "type": "User",
          "url": "https://api.github.com/users/veziak"
        }
      },
      "repository": {
        "allow_forking": true,
        "archive_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/{archive_format}{/ref}",
        "archived": false,
        "assignees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/assignees{/user}",
        "blobs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/blobs{/sha}",
        "branches_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/branches{/branch}",
        "clone_url": "https://github.com/diggerhq/tfrun_demo_multienv.git",
        "collaborators_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/collaborators{/collaborator}",
        "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/comments{/number}",
        "commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/commits{/sha}",
        "compare_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/compare/{base}...{head}",
        "contents_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contents/{+path}",
        "contributors_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contributors",
        "created_at": "2023-03-08T11:06:31Z",
        "default_branch": "main",
        "deployments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/deployments",
        "description": null,
        "disabled": false,
        "downloads_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/downloads",
        "events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/events",
        "fork": false,
        "forks": 2,
        "forks_count": 2,
        "forks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/forks",
        "full_name": "diggerhq/tfrun_demo_multienv",
        "git_commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/commits{/sha}",
        "git_refs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/refs{/sha}",
        "git_tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/tags{/sha}",
        "git_url": "git://github.com/diggerhq/tfrun_demo_multienv.git",
        "has_discussions": false,
        "has_downloads": true,
        "has_issues": true,
        "has_pages": false,
        "has_projects": true,
        "has_wiki": true,
        "homepage": null,
        "hooks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/hooks",
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv",
        "id": 611213652,
        "is_template": false,
        "issue_comment_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments{/number}",
        "issue_events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/events{/number}",
        "issues_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues{/number}",
        "keys_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/keys{/key_id}",
        "labels_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/labels{/name}",
        "language": "HCL",
        "languages_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/languages",
        "license": null,
        "merges_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/merges",
        "milestones_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/milestones{/number}",
        "mirror_url": null,
        "name": "tfrun_demo_multienv",
        "node_id": "R_kgDOJG5hVA",
        "notifications_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/notifications{?since,all,participating}",
        "open_issues": 5,
        "open_issues_count": 5,
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
        "pulls_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls{/number}",
        "pushed_at": "2023-03-10T14:09:35Z",
        "releases_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/releases{/id}",
        "size": 22,
        "ssh_url": "git@github.com:diggerhq/tfrun_demo_multienv.git",
        "stargazers_count": 0,
        "stargazers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/stargazers",
        "statuses_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/{sha}",
        "subscribers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscribers",
        "subscription_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscription",
        "svn_url": "https://github.com/diggerhq/tfrun_demo_multienv",
        "tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/tags",
        "teams_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/teams",
        "topics": [],
        "trees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/trees{/sha}",
        "updated_at": "2023-03-08T11:18:18Z",
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv",
        "visibility": "public",
        "watchers": 0,
        "watchers_count": 0,
        "web_commit_signoff_required": false
      },
      "sender": {
        "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
        "events_url": "https://api.github.com/users/veziak/events{/privacy}",
        "followers_url": "https://api.github.com/users/veziak/followers",
        "following_url": "https://api.github.com/users/veziak/following{/other_user}",
        "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
        "gravatar_id": "",
        "html_url": "https://github.com/veziak",
        "id": 2407061,
        "login": "veziak",
        "node_id": "MDQ6VXNlcjI0MDcwNjE=",
        "organizations_url": "https://api.github.com/users/veziak/orgs",
        "received_events_url": "https://api.github.com/users/veziak/received_events",
        "repos_url": "https://api.github.com/users/veziak/repos",
        "site_admin": false,
        "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
        "type": "User",
        "url": "https://api.github.com/users/veziak"
      }
    },
    "server_url": "https://github.com",
    "api_url": "https://api.github.com",
    "graphql_url": "https://api.github.com/graphql",
    "ref_name": "11/merge",
    "ref_protected": false,
    "ref_type": "branch",
    "secret_source": "Actions",
    "workflow_ref": "diggerhq/tfrun_demo_multienv/.github/workflows/plan.yml@refs/pull/11/merge",
    "workflow_sha": "b8d885f7be8c742eccf037029b580dba7ab3d239",
    "workspace": "/home/runner/work/tfrun_demo_multienv/tfrun_demo_multienv",
    "action": "__diggerhq_tfrun",
    "event_path": "/home/runner/work/_temp/_github_workflow/event.json",
    "action_repository": "aws-actions/configure-aws-credentials",
    "action_ref": "v1",
    "path": "/home/runner/work/_temp/_runner_file_commands/add_path_d96e6365-db41-41ed-b60b-37e23ee7c516",
    "env": "/home/runner/work/_temp/_runner_file_commands/set_env_d96e6365-db41-41ed-b60b-37e23ee7c516",
    "step_summary": "/home/runner/work/_temp/_runner_file_commands/step_summary_d96e6365-db41-41ed-b60b-37e23ee7c516",
    "state": "/home/runner/work/_temp/_runner_file_commands/save_state_d96e6365-db41-41ed-b60b-37e23ee7c516",
    "output": "/home/runner/work/_temp/_runner_file_commands/set_output_d96e6365-db41-41ed-b60b-37e23ee7c516"
  }`

var githubContextCommentJson = `{
    "token": "***",
    "job": "build",
    "ref": "refs/heads/main",
    "sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
    "repository": "diggerhq/tfrun_demo_multienv",
    "repository_owner": "diggerhq",
    "repository_owner_id": "71334590",
    "repositoryUrl": "git://github.com/diggerhq/tfrun_demo_multienv.git",
    "run_id": "4406521640",
    "run_number": "69",
    "retention_days": "90",
    "run_attempt": "1",
    "artifact_cache_size_limit": "10",
    "repository_visibility": "public",
    "repository_id": "611213652",
    "actor_id": "2407061",
    "actor": "veziak",
    "triggering_actor": "veziak",
    "workflow": "CI",
    "head_ref": "",
    "base_ref": "",
    "event_name": "issue_comment",
    "event": {
      "action": "created",
      "comment": {
        "author_association": "CONTRIBUTOR",
        "body": "test",
        "created_at": "2023-03-13T15:14:08Z",
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11#issuecomment-1466341992",
        "id": 1466341992,
        "issue_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
        "node_id": "IC_kwDOJG5hVM5XZppo",
        "performed_via_github_app": null,
        "reactions": {
          "+1": 0,
          "-1": 0,
          "confused": 0,
          "eyes": 0,
          "heart": 0,
          "hooray": 0,
          "laugh": 0,
          "rocket": 0,
          "total_count": 0,
          "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments/1466341992/reactions"
        },
        "updated_at": "2023-03-13T15:14:08Z",
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments/1466341992",
        "user": {
          "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
          "events_url": "https://api.github.com/users/veziak/events{/privacy}",
          "followers_url": "https://api.github.com/users/veziak/followers",
          "following_url": "https://api.github.com/users/veziak/following{/other_user}",
          "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
          "gravatar_id": "",
          "html_url": "https://github.com/veziak",
          "id": 2407061,
          "login": "veziak",
          "node_id": "MDQ6VXNlcjI0MDcwNjE=",
          "organizations_url": "https://api.github.com/users/veziak/orgs",
          "received_events_url": "https://api.github.com/users/veziak/received_events",
          "repos_url": "https://api.github.com/users/veziak/repos",
          "site_admin": false,
          "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
          "type": "User",
          "url": "https://api.github.com/users/veziak"
        }
      },
      "issue": {
        "active_lock_reason": null,
        "assignee": null,
        "assignees": [],
        "author_association": "CONTRIBUTOR",
        "body": null,
        "closed_at": null,
        "comments": 5,
        "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/comments",
        "created_at": "2023-03-10T14:09:35Z",
        "draft": false,
        "events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/events",
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11",
        "id": 1619042081,
        "labels": [],
        "labels_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/labels{/name}",
        "locked": false,
        "milestone": null,
        "node_id": "PR_kwDOJG5hVM5LxUWM",
        "number": 11,
        "performed_via_github_app": null,
        "pull_request": {
          "diff_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11.diff",
          "html_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11",
          "merged_at": null,
          "patch_url": "https://github.com/diggerhq/tfrun_demo_multienv/pull/11.patch",
          "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls/11"
        },
        "reactions": {
          "+1": 0,
          "-1": 0,
          "confused": 0,
          "eyes": 0,
          "heart": 0,
          "hooray": 0,
          "laugh": 0,
          "rocket": 0,
          "total_count": 0,
          "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/reactions"
        },
        "repository_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv",
        "state": "open",
        "state_reason": null,
        "timeline_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11/timeline",
        "title": "trigger deploy",
        "updated_at": "2023-03-13T15:14:08Z",
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/11",
        "user": {
          "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
          "events_url": "https://api.github.com/users/veziak/events{/privacy}",
          "followers_url": "https://api.github.com/users/veziak/followers",
          "following_url": "https://api.github.com/users/veziak/following{/other_user}",
          "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
          "gravatar_id": "",
          "html_url": "https://github.com/veziak",
          "id": 2407061,
          "login": "veziak",
          "node_id": "MDQ6VXNlcjI0MDcwNjE=",
          "organizations_url": "https://api.github.com/users/veziak/orgs",
          "received_events_url": "https://api.github.com/users/veziak/received_events",
          "repos_url": "https://api.github.com/users/veziak/repos",
          "site_admin": false,
          "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
          "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
          "type": "User",
          "url": "https://api.github.com/users/veziak"
        }
      },
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
      "repository": {
        "allow_forking": true,
        "archive_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/{archive_format}{/ref}",
        "archived": false,
        "assignees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/assignees{/user}",
        "blobs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/blobs{/sha}",
        "branches_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/branches{/branch}",
        "clone_url": "https://github.com/diggerhq/tfrun_demo_multienv.git",
        "collaborators_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/collaborators{/collaborator}",
        "comments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/comments{/number}",
        "commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/commits{/sha}",
        "compare_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/compare/{base}...{head}",
        "contents_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contents/{+path}",
        "contributors_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/contributors",
        "created_at": "2023-03-08T11:06:31Z",
        "default_branch": "main",
        "deployments_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/deployments",
        "description": null,
        "disabled": false,
        "downloads_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/downloads",
        "events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/events",
        "fork": false,
        "forks": 2,
        "forks_count": 2,
        "forks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/forks",
        "full_name": "diggerhq/tfrun_demo_multienv",
        "git_commits_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/commits{/sha}",
        "git_refs_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/refs{/sha}",
        "git_tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/tags{/sha}",
        "git_url": "git://github.com/diggerhq/tfrun_demo_multienv.git",
        "has_discussions": false,
        "has_downloads": true,
        "has_issues": true,
        "has_pages": false,
        "has_projects": true,
        "has_wiki": true,
        "homepage": null,
        "hooks_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/hooks",
        "html_url": "https://github.com/diggerhq/tfrun_demo_multienv",
        "id": 611213652,
        "is_template": false,
        "issue_comment_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/comments{/number}",
        "issue_events_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues/events{/number}",
        "issues_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/issues{/number}",
        "keys_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/keys{/key_id}",
        "labels_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/labels{/name}",
        "language": "HCL",
        "languages_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/languages",
        "license": null,
        "merges_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/merges",
        "milestones_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/milestones{/number}",
        "mirror_url": null,
        "name": "tfrun_demo_multienv",
        "node_id": "R_kgDOJG5hVA",
        "notifications_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/notifications{?since,all,participating}",
        "open_issues": 5,
        "open_issues_count": 5,
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
        "pulls_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/pulls{/number}",
        "pushed_at": "2023-03-10T14:09:35Z",
        "releases_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/releases{/id}",
        "size": 24,
        "ssh_url": "git@github.com:diggerhq/tfrun_demo_multienv.git",
        "stargazers_count": 0,
        "stargazers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/stargazers",
        "statuses_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/statuses/{sha}",
        "subscribers_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscribers",
        "subscription_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/subscription",
        "svn_url": "https://github.com/diggerhq/tfrun_demo_multienv",
        "tags_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/tags",
        "teams_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/teams",
        "topics": [],
        "trees_url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv/git/trees{/sha}",
        "updated_at": "2023-03-08T11:18:18Z",
        "url": "https://api.github.com/repos/diggerhq/tfrun_demo_multienv",
        "visibility": "public",
        "watchers": 0,
        "watchers_count": 0,
        "web_commit_signoff_required": false
      },
      "sender": {
        "avatar_url": "https://avatars.githubusercontent.com/u/2407061?v=4",
        "events_url": "https://api.github.com/users/veziak/events{/privacy}",
        "followers_url": "https://api.github.com/users/veziak/followers",
        "following_url": "https://api.github.com/users/veziak/following{/other_user}",
        "gists_url": "https://api.github.com/users/veziak/gists{/gist_id}",
        "gravatar_id": "",
        "html_url": "https://github.com/veziak",
        "id": 2407061,
        "login": "veziak",
        "node_id": "MDQ6VXNlcjI0MDcwNjE=",
        "organizations_url": "https://api.github.com/users/veziak/orgs",
        "received_events_url": "https://api.github.com/users/veziak/received_events",
        "repos_url": "https://api.github.com/users/veziak/repos",
        "site_admin": false,
        "starred_url": "https://api.github.com/users/veziak/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/veziak/subscriptions",
        "type": "User",
        "url": "https://api.github.com/users/veziak"
      }
    },
    "server_url": "https://github.com",
    "api_url": "https://api.github.com",
    "graphql_url": "https://api.github.com/graphql",
    "ref_name": "main",
    "ref_protected": false,
    "ref_type": "branch",
    "secret_source": "Actions",
    "workflow_ref": "diggerhq/tfrun_demo_multienv/.github/workflows/plan.yml@refs/heads/main",
    "workflow_sha": "3eb61a47a873fc574c7c57d00cf47343b9ef3892",
    "workspace": "/home/runner/work/tfrun_demo_multienv/tfrun_demo_multienv",
    "action": "__diggerhq_tfrun",
    "event_path": "/home/runner/work/_temp/_github_workflow/event.json",
    "action_repository": "aws-actions/configure-aws-credentials",
    "action_ref": "v1",
    "path": "/home/runner/work/_temp/_runner_file_commands/add_path_3bccb717-fa6a-4679-92eb-1ed2fc0b89b9",
    "env": "/home/runner/work/_temp/_runner_file_commands/set_env_3bccb717-fa6a-4679-92eb-1ed2fc0b89b9",
    "step_summary": "/home/runner/work/_temp/_runner_file_commands/step_summary_3bccb717-fa6a-4679-92eb-1ed2fc0b89b9",
    "state": "/home/runner/work/_temp/_runner_file_commands/save_state_3bccb717-fa6a-4679-92eb-1ed2fc0b89b9",
    "output": "/home/runner/work/_temp/_runner_file_commands/set_output_3bccb717-fa6a-4679-92eb-1ed2fc0b89b9"
  }`

var githubInvalidContextJson = `{
    "token": "***",
    "job": "build",
    "ref": "refs/pull/11/merge",
    "sha": "b8d885f7be8c742eccf037029b580dba7ab3d239",
    "repository": "diggerhq/tfrun_demo_multienv",
    "repository_owner": "diggerhq",
`

func TestGitHubNewPullRequestContext(t *testing.T) {

	actionContext, err := models.GetGitHubContext(githubContextNewPullRequestJson)
	context := actionContext.ToEventPackage()

	assert.NoError(t, err)
	if err != nil {
		log.Println(err)
	}
	ghEvent := context.Event

	diggerConfig := configuration.DiggerConfig{}
	lock := &utils.MockLock{}
	prManager := &utils.MockPullRequestManager{ChangedFiles: []string{"dev/test.tf"}}
	planStorage := &utils.MockPlanStorage{}
	policyChecker := &utils.MockPolicyChecker{}
	backendApi := &utils.MockBackendApi{}

	impactedProjects, requestedProject, prNumber, err := dggithub.ProcessGitHubEvent(ghEvent, &diggerConfig, prManager)

	reporter := &reporting.CiReporter{
		CiService: prManager,
		PrNumber:  prNumber,
	}

	event := context.Event.(github.PullRequestEvent)
	jobs, _, err := dggithub.ConvertGithubPullRequestEventToJobs(&event, impactedProjects, requestedProject, map[string]configuration.Workflow{})
	_, _, err = digger.RunJobs(jobs, prManager, prManager, lock, reporter, planStorage, policyChecker, backendApi, "")

	assert.NoError(t, err)
	if err != nil {
		log.Println(err)
	}
}

func TestGitHubNewCommentContext(t *testing.T) {
	actionContext, err := ghmodels.GetGitHubContext(githubContextCommentJson)
	context := actionContext.ToEventPackage()
	assert.NoError(t, err)
	if err != nil {
		log.Println(err)
	}
	ghEvent := context.Event
	diggerConfig := configuration.DiggerConfig{}
	lock := &utils.MockLock{}
	prManager := &utils.MockPullRequestManager{ChangedFiles: []string{"dev/test.tf"}}
	planStorage := &utils.MockPlanStorage{}
	impactedProjects, requestedProject, prNumber, err := dggithub.ProcessGitHubEvent(ghEvent, &diggerConfig, prManager)
	reporter := &reporting.CiReporter{
		CiService: prManager,
		PrNumber:  prNumber,
	}

	policyChecker := &utils.MockPolicyChecker{}
	backendApi := &utils.MockBackendApi{}

	event := context.Event.(github.IssueCommentEvent)
	jobs, _, err := dggithub.ConvertGithubIssueCommentEventToJobs(&event, impactedProjects, requestedProject, map[string]configuration.Workflow{})
	_, _, err = digger.RunJobs(jobs, prManager, prManager, lock, reporter, planStorage, policyChecker, backendApi, "")
	assert.NoError(t, err)
	if err != nil {
		log.Println(err)
	}
}

func TestInvalidGitHubContext(t *testing.T) {
	_, err := ghmodels.GetGitHubContext(githubInvalidContextJson)
	require.Error(t, err)
	if err != nil {
		log.Println(err)
	}
}

func TestGitHubNewPullRequestInMultiEnvProjectContext(t *testing.T) {
	actionContext, err := models.GetGitHubContext(githubContextNewPullRequestJson)
	context := actionContext.ToEventPackage()
	assert.NoError(t, err)
	ghEvent := context.Event
	pullRequestNumber := 11
	dev := configuration.Project{Name: "dev", Dir: "dev", Workflow: "dev"}
	prod := configuration.Project{Name: "prod", Dir: "prod", Workflow: "prod"}
	workflows := map[string]configuration.Workflow{
		"dev": {
			Plan: &configuration.Stage{Steps: []configuration.Step{
				{Action: "init", ExtraArgs: []string{}},
				{Action: "plan", ExtraArgs: []string{"-var-file=dev.tfvars"}},
			}},
			Apply: &configuration.Stage{Steps: []configuration.Step{
				{Action: "init", ExtraArgs: []string{}},
				{Action: "apply", ExtraArgs: []string{"-var-file=dev.tfvars"}},
			}},
			Configuration: &configuration.WorkflowConfiguration{
				OnPullRequestPushed: []string{"digger plan"},
				OnPullRequestClosed: []string{"digger unlock"},
				OnCommitToDefault:   []string{"digger apply"},
			},
		},
		"prod": {
			Plan: &configuration.Stage{Steps: []configuration.Step{
				{Action: "init", ExtraArgs: []string{}},
				{Action: "plan", ExtraArgs: []string{"-var-file=dev.tfvars"}},
			}},
			Apply: &configuration.Stage{Steps: []configuration.Step{
				{Action: "init", ExtraArgs: []string{}},
				{Action: "apply", ExtraArgs: []string{"-var-file=dev.tfvars"}},
			}},
			Configuration: &configuration.WorkflowConfiguration{
				OnPullRequestPushed: []string{"digger plan"},
				OnPullRequestClosed: []string{"digger unlock"},
				OnCommitToDefault:   []string{"digger apply"},
			},
		},
	}
	projects := []configuration.Project{dev, prod}
	diggerConfig := configuration.DiggerConfig{Projects: projects}

	// PullRequestManager Mock
	prManager := &utils.MockPullRequestManager{ChangedFiles: []string{"dev/test.tf"}}
	lock := &utils.MockLock{}
	impactedProjects, requestedProject, prNumber, err := dggithub.ProcessGitHubEvent(ghEvent, &diggerConfig, prManager)
	assert.NoError(t, err)
	event := context.Event.(github.PullRequestEvent)
	jobs, _, err := dggithub.ConvertGithubPullRequestEventToJobs(&event, impactedProjects, requestedProject, workflows)
	spew.Dump(lock.MapLock)
	assert.Equal(t, pullRequestNumber, prNumber)
	assert.Equal(t, 1, len(jobs))
	assert.NoError(t, err)
}

func TestGitHubTestPRCommandCaseInsensitivity(t *testing.T) {
	issuenumber := 1
	ghEvent := github.IssueCommentEvent{
		Comment: &github.IssueComment{},
		Issue: &github.Issue{
			Number: &issuenumber,
		},
		Repo:   &github.Repository{FullName: github.String("asdd")},
		Sender: &github.User{Login: github.String("login")},
	}
	comment := "DiGGeR PlAn"
	ghEvent.Comment.Body = &comment

	project := configuration.Project{Name: "test project", Workflow: "default"}
	var impactedProjects []configuration.Project
	impactedProjects = make([]configuration.Project, 1)
	impactedProjects[0] = project
	var requestedProject = project
	workflows := make(map[string]configuration.Workflow, 1)
	workflows["default"] = configuration.Workflow{}
	jobs, _, err := dggithub.ConvertGithubIssueCommentEventToJobs(&ghEvent, impactedProjects, &requestedProject, workflows)

	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, "digger plan", jobs[0].Commands[0])
	assert.NoError(t, err)
}
