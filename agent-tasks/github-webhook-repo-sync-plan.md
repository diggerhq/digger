# GitHub repo sync via webhooks

## Goals
- Move repo sync away from the OAuth callback and drive updates from GitHub webhooks.
- Keep Digger’s repo list accurate when repos are added/removed from the app scope.
- On uninstall, soft-delete repos (and their installation records) so they disappear from the UI/API.

## Current behavior (source of truth today)
- OAuth callback (`backend/controllers/github_callback.go`) validates the install, links/creates org, then lists all repos via `Apps.ListRepos`, soft-deletes existing `github_app_installations` and repos for the org, and recreates them via `GithubRepoAdded` + `createOrGetDiggerRepoForGithubRepo`.
- Webhook handler (`backend/controllers/github.go`) only uses `installation` events with action `deleted` to mark installation links inactive and set `github_app_installations` status deleted for the repos in the payload. It does not touch `repos`. There is no handling for `installation_repositories` add/remove.
- Runtime lookups (`GetGithubService` / `GetGithubClient`) require an active record in `github_app_installations` for the repo.

## Target design
- Keep OAuth callback minimal: verify installation, create/link org, store the install id/app id, but do **not** list or mutate repos. It should return immediately and rely on webhooks for repo population.
- Webhook-driven reconciliation:
  - `installation` event (`created`, `unsuspended`, `new_permissions_accepted`): ensure installation link exists/active; reconcile repos using the payload’s `installation.repositories` list as authoritative. If the link is missing, log an error and return (no auto-create).
    - Soft-delete existing `github_app_installations` for that installation id, and soft-delete repos for the linked org (scoped to that installation) before re-adding.
    - Upsert each repo: mark/install via `GithubRepoAdded` and create/restore the Digger repo record (store app id, installation id, default branch, clone URL when available).
  - `installation_repositories` event: incrementally apply scope changes.
    - For `repositories_added`: fetch repo details (to get default branch + clone URL), then call `GithubRepoAdded` and create/restore the repo record.
    - For `repositories_removed`: mark `GithubRepoRemoved`, soft-delete the repo **and its projects**, and handle absence gracefully.
  - `installation` event (`deleted`): mark installation link inactive, mark installation records deleted, and soft-delete repos **and projects** for that installation’s org so they no longer appear in APIs/UI.
- Shared helpers:
  - `syncReposForInstallation(installationId, appId, reposPayload)` to wrap the add/remove logic and reuse between `installation` and `installation_repositories` handlers.
  - `softDeleteRepoAndProjects(orgId, repoFullName)` to encapsulate repo + project soft-deletion.
- Observability: structured logs per action, and possibly a metric for sync success/failure per installation.

## Migration plan
1) Add webhook handling for `installation_repositories` in `GithubAppWebHook` switch and wire to a new handler.
2) Extend `installation` handling to cover `created`/`unsuspended` (not just `deleted`) and call `syncReposForInstallation`.
3) Update uninstall handling to also soft-delete repos and projects.
4) Strip repo enumeration/deletion from the OAuth callback; leave only installation/org linking.
5) Add tests using existing payload fixtures (`installationRepositoriesAddedPayload`, `installationRepositoriesDeletedPayload`, `installationCreatedEvent`) to verify DB state changes (installation records + repos soft-delete/restore).
6) Backfill existing installations: one-off job/command or admin endpoint to resync repos via `Apps.ListRepos` and `syncReposForInstallation` to align data after deploying (manual trigger, no cron yet).

## Testing / validation
- Unit tests for add/remove/uninstall flows verifying:
  - `github_app_installations` status transitions.
  - Repos are created/restored with correct installation/app ids.
  - Repos and projects are soft-deleted on removal/uninstall.

## Open questions
- None right now (decided: log missing-link errors only; soft-delete repos and projects on removal/uninstall; add manual resync endpoint, no cron yet).
