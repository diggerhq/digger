# Migrate Repository Management from Callback to Webhooks

- Target: `backend/controllers/github*.go`, `backend/models/storage.go`
- Related: `ee/backend/controllers/github.go` (app manifest), `libs/ci/github/github_utils.go`

## Goal

Move repository synchronization entirely to webhook-driven updates. The OAuth callback should only handle organization/installation linking, not repository CRUD. Repositories should be created, soft-deleted, and restored exclusively via webhook events (`installation`, `installation_repositories`).

## Non-Goals

- Changing how other webhook events (push, PR, comments) are handled.
- Modifying the `GithubAppInstallation` model (per-repo installation records).
- Changing authentication or GitHub client provider logic.

## Current State Summary

### Callback Flow (`backend/controllers/github_callback.go`)

The `GithubAppCallbackPage()` handler currently:
1. Receives `installation_id`, `code`, and `state` (appId) from GitHub.
2. Validates the callback via `validateGithubCallback()` using the OAuth code.
3. Creates or fetches `GithubAppInstallationLink` (org ↔ installation mapping).
4. **Problem**: Fetches all repos via `github.ListGithubRepos(client)` and syncs them:
   - Soft-deletes ALL existing repos for the org (`models.DB.GormDB.Delete(ExistingRepos, "organisation_id=?", orgId)`).
   - Marks all `GithubAppInstallation` records as deleted.
   - Re-creates repos one by one via `createOrGetDiggerRepoForGithubRepo()`.
5. **Problem**: Fails if `code` parameter is missing (GitHub doesn't always send it on re-auth).

### Webhook Flow (`backend/controllers/github.go`)

The `GithubAppWebHook()` handler processes:
- `InstallationEvent` with action `deleted` → calls `handleInstallationDeletedEvent()`.
- `PushEvent`, `IssueCommentEvent`, `PullRequestEvent` → existing handlers.

**Missing**: No handler for `InstallationRepositoriesEvent` (fired when repos are added/removed from an existing installation).

### Existing Handlers

**`handleInstallationDeletedEvent()`** (`github_installation.go:9-49`):
- Makes `GithubAppInstallationLink` inactive.
- Calls `GithubRepoRemoved()` for each repo in the event payload.
- **Does NOT soft-delete `Repo` records** — only updates `GithubAppInstallation.Status`.

### Database Models

**`Repo`** (`backend/models/orgs.go:33-48`):
- Uses `gorm.Model` with built-in `DeletedAt` for soft-delete.
- Fields: `Name`, `RepoFullName`, `GithubAppInstallationId`, `GithubAppId`, `DefaultBranch`, `CloneUrl`.

**`GithubAppInstallation`** (`backend/models/github.go:40-48`):
- Per-repo record linking `GithubInstallationId` + `Repo` (full name) + `GithubAppId`.
- `Status`: `GithubAppInstallActive` (1) or `GithubAppInstallDeleted` (2).

**`GithubAppInstallationLink`** (`backend/models/github.go:57-64`):
- Links `GithubInstallationId` ↔ `OrganisationId`.
- `Status`: `GithubAppInstallationLinkActive` (1) or `GithubAppInstallationLinkInactive` (2).

**`Project`** (`backend/models/orgs.go`):
- Linked to repos via `RepoFullName`.
- Also uses `gorm.Model` with soft-delete support.
- **Must be soft-deleted alongside repos** on removal/uninstall.

### Helper Functions

**`createOrGetDiggerRepoForGithubRepo()`** (`github_helpers.go:914-976`):
- Looks up existing repo (including soft-deleted via `Unscoped()`).
- If found and deleted: restores by clearing `DeletedAt`.
- If not found: creates new repo.

**`GithubRepoAdded()`** (`storage.go:460-489`):
- Creates or reactivates `GithubAppInstallation` record.

**`GithubRepoRemoved()`** (`storage.go:491-512`):
- Sets `GithubAppInstallation.Status` to `GithubAppInstallDeleted`.

### GitHub App Manifest (`ee/backend/controllers/github.go:61-73`)

Currently subscribed events:
```
check_run, create, delete, issue_comment, issues, status,
pull_request_review_thread, pull_request_review_comment,
pull_request_review, pull_request, push
```

**Missing**: `installation_repositories` event not subscribed.

## Acceptance Criteria

1. **Callback resilience**: `GithubAppCallbackPage()` succeeds even when `code` is missing.
2. **No repo sync in callback**: Callback only creates/updates org and installation link; no repo listing or CRUD.
3. **Webhook handles app install**: New `installation` event with action `created`/`unsuspended`/`new_permissions_accepted` creates repos from `event.Repositories`.
4. **Webhook handles app uninstall**: Existing `installation` event with action `deleted` soft-deletes all repos **and their projects** for the installation.
5. **Webhook handles scope changes**: New `installation_repositories` event handler:
   - Action `added`: Creates or restores repos from `event.RepositoriesAdded`.
   - Action `removed`: Soft-deletes repos **and their projects** from `event.RepositoriesRemoved`.
6. **App manifest updated**: `installation_repositories` event added to default events list.
7. **Soft-delete for Repos and Projects**: Both `Repo` and `Project` records are soft-deleted on removal/uninstall.
8. **Retry logic for race conditions**: Webhook handlers retry fetching installation link using `github.com/sethvargo/go-retry` since webhook may arrive before callback creates the link.
9. **Manual resync endpoint**: Admin API endpoint to resync repos for an existing installation.

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Existing installations miss initial repos | Provide manual resync endpoint (`POST /api/github/resync`) |
| Webhook delivery failures | GitHub retries webhooks; handlers are idempotent |
| Race conditions between callback and webhook | Webhook handlers retry with exponential backoff using `go-retry` |
| Missing `code` breaks existing flow | Make `code` optional; skip OAuth validation if missing |
| Projects orphaned when repos deleted | Soft-delete projects alongside repos in same transaction |

## Implementation Plan

### Commit 1: Add go-retry dependency

**Action**: Add `github.com/sethvargo/go-retry` to `backend/go.mod`.

**Files**: `backend/go.mod`, `backend/go.sum`

**Changes**:
```go
require (
    github.com/sethvargo/go-retry v0.3.0
)
```

**Verify**: `go mod tidy` succeeds.

### Commit 2: Make callback resilient to missing `code`

**Action**: Modify `GithubAppCallbackPage()` to not fail when `code` is missing. GitHub omits `code` on some re-authorization flows.

**Files**: `backend/controllers/github_callback.go`

**Changes**:
- Check if `code` exists before validation.
- If missing, skip `validateGithubCallback()` but continue with installation link creation.
- Log info when code is missing ("repos will sync via webhook").
- Populate `vcsOwner` from OAuth validation only when code is present.

**Verify**: Callback succeeds with and without `code` parameter.

### Commit 3: Remove repo sync from callback

**Action**: Strip all repo-fetching and repo-creation logic from `GithubAppCallbackPage()`. The callback should only:
1. Parse `installation_id` and optional `code`/`state`.
2. Validate callback (if code present).
3. Create or fetch `GithubAppInstallationLink`.
4. Return success page.

**Files**: `backend/controllers/github_callback.go`

**Changes**:
- Remove `github.ListGithubRepos(client)` call.
- Remove soft-delete of existing repos.
- Remove soft-delete of `GithubAppInstallation` records.
- Remove repo creation loop.
- Keep org creation and link creation logic.
- Remove unused imports (`strings`, `github`, `utils`).

**Verify**: Callback completes without touching repos table.

### Commit 4: Add soft-delete helpers for repos and projects

**Action**: Add database helper functions to soft-delete repos and their associated projects.

**Files**: `backend/models/storage.go`

**Changes**:
- Add `SoftDeleteRepoAndProjects(orgId uint, repoFullName string) error`:
  - Use transaction to soft-delete projects first, then repo.
  - Query by `organisation_id` and `repo_full_name`.
- Add `SoftDeleteReposAndProjectsByInstallation(orgId uint, installationId int64) error`:
  - Fetch all repos for the org with matching `github_app_installation_id`.
  - Call `SoftDeleteRepoAndProjects` for each.

**Verify**: Unit tests for both functions pass.

### Commit 5: Add helper functions for repo upsert/remove

**Action**: Create reusable helper functions for webhook handlers.

**Files**: `backend/controllers/github_installation.go`

**Changes**:
- Add `getAccountDetails(account *github.User) (login string, accountId int64)`:
  - Safely extract login and ID with nil checks.
- Add `fetchRepoIdentifiers(ctx, client, repo, installationId) (fullName, owner, name, defaultBranch, cloneURL, error)`:
  - Extract identifiers from webhook payload.
  - If `defaultBranch` or `cloneURL` missing, fetch from GitHub API.
- Add `upsertRepo(ctx, client, repo, installationId, appId, accountLogin, accountId) error`:
  - Call `fetchRepoIdentifiers`.
  - Call `GithubRepoAdded`.
  - Call `createOrGetDiggerRepoForGithubRepo`.
- Add `removeRepo(ctx, repo, installationId, appId, orgId) error`:
  - Call `GithubRepoRemoved`.
  - Call `SoftDeleteRepoAndProjects`.

**Verify**: Helper functions compile and are ready for use.

### Commit 6: Add installation upsert handler with retry

**Action**: Handle `InstallationEvent` with actions `created`, `unsuspended`, `new_permissions_accepted` to sync repos.

**Files**: `backend/controllers/github.go`, `backend/controllers/github_installation.go`

**Changes**:
- In `GithubAppWebHook()` switch case for `InstallationEvent`:
  - Add handling for `created`, `unsuspended`, `new_permissions_accepted` actions.
  - Call new `handleInstallationUpsertEvent()`.
- Create `handleInstallationUpsertEvent(ctx, gh, event, appId) error`:
  - **Retry logic**: Use `retry.Do` with `retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))` to fetch installation link.
  - If link not found after retries, return error (webhook will be retried by GitHub).
  - Soft-delete existing `GithubAppInstallation` records for the installation.
  - Soft-delete existing repos and projects for the installation.
  - For each repo in `event.Repositories`, call `upsertRepo`.

**Verify**: Installing app creates repos via webhook; handles race condition gracefully.

### Commit 7: Update installation deleted handler

**Action**: Modify `handleInstallationDeletedEvent()` to soft-delete repos and projects, not just `GithubAppInstallation` records.

**Files**: `backend/controllers/github_installation.go`

**Changes**:
- Mark all `GithubAppInstallation` records as deleted for the installation.
- Call `SoftDeleteReposAndProjectsByInstallation` to soft-delete all repos and projects.
- Keep existing logic for individual repo removal from payload.

**Verify**: Uninstalling app soft-deletes repos and projects.

### Commit 8: Add installation_repositories event handler with retry

**Action**: Handle `InstallationRepositoriesEvent` for incremental repo changes (scope modifications).

**Files**: `backend/controllers/github.go`, `backend/controllers/github_installation.go`

**Changes**:
- In `GithubAppWebHook()`:
  - Add new case for `*github.InstallationRepositoriesEvent`.
  - Log action, added count, removed count.
  - Call new `handleInstallationRepositoriesEvent()`.
- Create `handleInstallationRepositoriesEvent(ctx, gh, event, appId) error`:
  - **Retry logic**: Use same retry pattern as upsert handler.
  - For `RepositoriesAdded`: call `upsertRepo` for each.
  - For `RepositoriesRemoved`: call `removeRepo` for each.
  - Collect errors but continue processing; return aggregated error at end.

**Verify**: Adding/removing repos from app scope creates/soft-deletes repos and projects.

### Commit 9: Add manual resync API endpoint

**Action**: Add admin API endpoint to resync repos for an existing installation.

**Files**: `backend/controllers/github_api.go`, `backend/bootstrap/main.go`

**Changes**:
- Add `ResyncGithubInstallationApi(c *gin.Context)`:
  - Accept `installation_id` in JSON body.
  - Fetch installation link; return 404 if not found.
  - Fetch a `GithubAppInstallation` record to get `appId`.
  - Create GitHub client and call `ListGithubRepos`.
  - Build synthetic `InstallationEvent` and call `handleInstallationUpsertEvent`.
  - Return success with repo count.
- Register route: `githubApiGroup.POST("/resync", controllers.ResyncGithubInstallationApi)`.

**Verify**: `POST /api/github/resync` resyncs repos for an installation.

### Commit 10: Update GitHub App manifest with new event

**Action**: Add `installation_repositories` to the default events list in app manifest.

**Files**: `ee/backend/controllers/github.go`

**Changes**:
- Add `"installation_repositories"` to the `Events` slice.

**Verify**: New app installations subscribe to `installation_repositories` webhook.

### Commit 11: Add unit tests

**Action**: Add tests for new handlers and storage functions.

**Files**: `backend/models/storage_test.go`, `backend/controllers/github_installation_test.go` (new)

**Changes**:
- Test `SoftDeleteRepoAndProjects`: verify repo and projects are soft-deleted.
- Test `SoftDeleteReposAndProjectsByInstallation`: verify only repos for specific installation are deleted.
- Test webhook handlers with mocked events.

**Verify**: All tests pass.

## Code Sketches

### Retry pattern for installation link lookup

```go
import (
    "context"
    "errors"
    "time"
    "github.com/sethvargo/go-retry"
)

// In handleInstallationUpsertEvent and handleInstallationRepositoriesEvent:
var link *models.GithubAppInstallationLink
backoff := retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))
err := retry.Do(ctx, backoff, func(ctx context.Context) error {
    var dbErr error
    link, dbErr = models.DB.GetGithubInstallationLinkForInstallationId(installationId)
    if dbErr != nil {
        return dbErr // permanent error, stop retrying
    }
    if link == nil {
        return retry.RetryableError(errors.New("installation link not found"))
    }
    return nil
})
if err != nil {
    slog.Error("Installation link not found after retries", "installationId", installationId, "error", err)
    return fmt.Errorf("installation link not found for installation %d after retries: %w", installationId, err)
}
```

### SoftDeleteRepoAndProjects

```go
func (db *Database) SoftDeleteRepoAndProjects(orgId uint, repoFullName string) error {
    return db.GormDB.Transaction(func(tx *gorm.DB) error {
        // Soft-delete projects first
        if err := tx.Where("organisation_id = ? AND repo_full_name = ?", orgId, repoFullName).Delete(&Project{}).Error; err != nil {
            slog.Error("failed to soft delete projects for repo", "orgId", orgId, "repoFullName", repoFullName, "error", err)
            return err
        }
        // Soft-delete repo
        if err := tx.Where("organisation_id = ? AND repo_full_name = ?", orgId, repoFullName).Delete(&Repo{}).Error; err != nil {
            slog.Error("failed to soft delete repo", "orgId", orgId, "repoFullName", repoFullName, "error", err)
            return err
        }
        return nil
    })
}
```

### SoftDeleteReposAndProjectsByInstallation

```go
func (db *Database) SoftDeleteReposAndProjectsByInstallation(orgId uint, installationId int64) error {
    var repos []Repo
    if err := db.GormDB.Where("organisation_id = ? AND github_app_installation_id = ?", orgId, installationId).Find(&repos).Error; err != nil {
        slog.Error("failed to fetch repos for soft delete", "orgId", orgId, "installationId", installationId, "error", err)
        return err
    }

    for _, repo := range repos {
        if err := db.SoftDeleteRepoAndProjects(orgId, repo.RepoFullName); err != nil {
            return err
        }
    }

    return nil
}
```

### handleInstallationUpsertEvent

```go
func handleInstallationUpsertEvent(ctx context.Context, gh utils.GithubClientProvider, installation *github.InstallationEvent, appId int64) error {
    installationId := installation.Installation.GetID()
    appIdFromPayload := appId
    if installation.Installation.AppID != nil {
        appIdFromPayload = installation.Installation.GetAppID()
    }

    accountLogin, accountId := getAccountDetails(installation.Installation.Account)

    // Retry fetching the link since webhook may arrive before OAuth callback creates it
    var link *models.GithubAppInstallationLink
    backoff := retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))
    err := retry.Do(ctx, backoff, func(ctx context.Context) error {
        var dbErr error
        link, dbErr = models.DB.GetGithubInstallationLinkForInstallationId(installationId)
        if dbErr != nil {
            return dbErr // permanent error, stop retrying
        }
        if link == nil {
            return retry.RetryableError(errors.New("installation link not found"))
        }
        return nil
    })
    if err != nil {
        slog.Error("Installation link not found after retries", "installationId", installationId, "error", err)
        return fmt.Errorf("installation link not found for installation %d after retries: %w", installationId, err)
    }

    repoList := installation.Repositories
    if len(repoList) == 0 {
        slog.Warn("No repositories found to sync for installation", "installationId", installationId)
        return nil
    }

    slog.Info("Syncing repositories for installation",
        "installationId", installationId,
        "appId", appIdFromPayload,
        "repoCount", len(repoList),
    )

    // Mark existing installations as deleted before resync
    if err := models.DB.GormDB.Model(&models.GithubAppInstallation{}).Where("github_installation_id = ?", installationId).Update("status", models.GithubAppInstallDeleted).Error; err != nil {
        slog.Error("Error marking installations deleted prior to resync", "installationId", installationId, "error", err)
        return err
    }

    // Soft-delete existing repos and projects
    if err := models.DB.SoftDeleteReposAndProjectsByInstallation(link.OrganisationId, installationId); err != nil {
        slog.Error("Error soft deleting existing repos/projects prior to resync", "installationId", installationId, "orgId", link.OrganisationId, "error", err)
        return err
    }

    ghClient, _, err := gh.Get(appIdFromPayload, installationId)
    if err != nil {
        slog.Error("Error creating GitHub client for repo sync", "installationId", installationId, "error", err)
        return err
    }

    for _, repo := range repoList {
        if err := upsertRepo(ctx, ghClient, repo, installationId, appIdFromPayload, accountLogin, accountId); err != nil {
            return err
        }
    }

    slog.Info("Successfully synced repositories for installation", "installationId", installationId)
    return nil
}
```

### handleInstallationRepositoriesEvent

```go
func handleInstallationRepositoriesEvent(ctx context.Context, gh utils.GithubClientProvider, event *github.InstallationRepositoriesEvent, appId int64) error {
    installationId := event.Installation.GetID()
    appIdFromPayload := appId
    if event.Installation.AppID != nil {
        appIdFromPayload = event.Installation.GetAppID()
    }

    accountLogin, accountId := getAccountDetails(event.Installation.Account)

    // Retry fetching the link since webhook may arrive before OAuth callback creates it
    var link *models.GithubAppInstallationLink
    backoff := retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))
    err := retry.Do(ctx, backoff, func(ctx context.Context) error {
        var dbErr error
        link, dbErr = models.DB.GetGithubInstallationLinkForInstallationId(installationId)
        if dbErr != nil {
            return dbErr // permanent error, stop retrying
        }
        if link == nil {
            return retry.RetryableError(errors.New("installation link not found"))
        }
        return nil
    })
    if err != nil {
        slog.Error("Installation link not found after retries", "installationId", installationId, "error", err)
        return fmt.Errorf("installation link not found for installation %d after retries: %w", installationId, err)
    }

    client, _, err := gh.Get(appIdFromPayload, installationId)
    if err != nil {
        slog.Error("Error creating GitHub client for installation_repositories event", "installationId", installationId, "error", err)
        return err
    }

    var errs []error
    for _, repo := range event.RepositoriesAdded {
        if err := upsertRepo(ctx, client, repo, installationId, appIdFromPayload, accountLogin, accountId); err != nil {
            errs = append(errs, err)
        }
    }

    for _, repo := range event.RepositoriesRemoved {
        if err := removeRepo(ctx, repo, installationId, appIdFromPayload, link.OrganisationId); err != nil {
            errs = append(errs, err)
        }
    }

    slog.Info("Handled installation_repositories event",
        "installationId", installationId,
        "addedCount", len(event.RepositoriesAdded),
        "removedCount", len(event.RepositoriesRemoved),
    )
    if len(errs) > 0 {
        return fmt.Errorf("one or more errors during installation_repositories handling: %v", errs)
    }
    return nil
}
```

### ResyncGithubInstallationApi

```go
func ResyncGithubInstallationApi(c *gin.Context) {
    type ResyncInstallationRequest struct {
        InstallationId string `json:"installation_id"`
    }

    var request ResyncInstallationRequest
    if err := c.BindJSON(&request); err != nil {
        slog.Error("Error binding JSON for resync", "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request format"})
        return
    }

    installationId, err := strconv.ParseInt(request.InstallationId, 10, 64)
    if err != nil {
        slog.Error("Failed to convert InstallationId to int64", "installationId", request.InstallationId, "error", err)
        c.JSON(http.StatusBadRequest, gin.H{"status": "installationID should be a valid integer"})
        return
    }

    link, err := models.DB.GetGithubAppInstallationLink(installationId)
    if err != nil {
        slog.Error("Could not get installation link for resync", "installationId", installationId, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Could not get installation link"})
        return
    }
    if link == nil {
        slog.Warn("Installation link not found for resync", "installationId", installationId)
        c.JSON(http.StatusNotFound, gin.H{"status": "Installation link not found"})
        return
    }

    // Get appId from an existing installation record
    var installationRecord models.GithubAppInstallation
    if err := models.DB.GormDB.Where("github_installation_id = ?", installationId).Order("updated_at desc").First(&installationRecord).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            slog.Warn("No installation records found for resync", "installationId", installationId)
            c.JSON(http.StatusNotFound, gin.H{"status": "No installation records found"})
            return
        }
        slog.Error("Failed to fetch installation record for resync", "installationId", installationId, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Could not fetch installation records"})
        return
    }

    appId := installationRecord.GithubAppId
    ghProvider := utils.DiggerGithubRealClientProvider{}

    client, _, err := ghProvider.Get(appId, installationId)
    if err != nil {
        slog.Error("Failed to create GitHub client for resync", "installationId", installationId, "appId", appId, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Failed to create GitHub client"})
        return
    }

    repos, err := ci_github.ListGithubRepos(client)
    if err != nil {
        slog.Error("Failed to list repos for resync", "installationId", installationId, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Failed to list repos for resync"})
        return
    }

    // Build synthetic InstallationEvent and call upsert handler
    installationPayload := &github.Installation{
        ID:    github.Int64(installationId),
        AppID: github.Int64(appId),
    }
    resyncEvent := &github.InstallationEvent{
        Installation: installationPayload,
        Repositories: repos,
    }

    if err := handleInstallationUpsertEvent(c.Request.Context(), ghProvider, resyncEvent, appId); err != nil {
        slog.Error("Resync failed", "installationId", installationId, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status": "Resync failed"})
        return
    }

    slog.Info("Resync completed", "installationId", installationId, "repoCount", len(repos))
    c.JSON(http.StatusOK, gin.H{"status": "Resync completed", "repoCount": len(repos)})
}
```

## Verification Steps

1. **Unit tests**: Add tests for new handlers with mocked events.
   - Test `SoftDeleteRepoAndProjects`: verify repo and projects are soft-deleted.
   - Test `SoftDeleteReposAndProjectsByInstallation`: verify only repos for specific installation are deleted, others untouched.
2. **Integration test**:
   - Install app → verify repos created via webhook.
   - Remove repo from scope → verify repo and projects soft-deleted.
   - Add repo back → verify restored.
   - Uninstall app → verify all repos and projects soft-deleted.
3. **Callback test**: Hit callback endpoint without `code` → should succeed.
4. **Race condition test**: Trigger webhook before callback completes → verify retry succeeds.
5. **Resync test**: Call `POST /api/github/resync` → verify repos are synced.
6. **Manual verification**: Use GitHub App settings to add/remove repos and observe DB changes.

## Follow-ups (Later; not in this plan)

- Consider adding `webhook_synced_at` timestamp to `Repo` for debugging.
- Rate limiting for webhook processing if needed.
- Metrics/observability for sync success/failure per installation.
- Document migration path for existing installations in user-facing docs.
