# Persist Detection Runs (Append‑Only)

This document outlines a minimal, append‑only design to persist every time we compute “impacted projects” for a PR. The goal is auditability and simple, reliable retrieval of the latest detection run, without extra counters or complex coordination.

## Summary
- Add a single append‑only table `digger_detection_runs`.
- Insert one row per detection run (PR and Issue Comment flows) with denormalized JSON payloads.
- Use timestamps (`created_at`) to identify the latest run for a given PR.
- No updates, no deletes.

## Scope
- Persist detection runs for:
  - GitHub Pull Request events (`handlePullRequestEvent`).
  - GitHub Issue Comment events (`handleIssueCommentEvent`).
- Denormalized JSON for impacted projects and source mappings.
- Minimal model + writer method; errors are logged but do not break main flow.

Out of scope:
- EE and OpenTaco.
- Additional VCS (GitLab/Bitbucket) wiring (can be added later similarly).
- Lock/PR inconsistency detection (future step once data is persisted).

## Schema (Postgres)
Create a single table for append‑only detection runs.

```sql
-- backend/migrations/20251107000100.sql
CREATE TABLE "public"."digger_detection_runs" (
  "id"               bigserial PRIMARY KEY,
  "created_at"       timestamptz NOT NULL DEFAULT now(),
  "updated_at"       timestamptz,
  "deleted_at"       timestamptz,

  "organisation_id"  bigint       NOT NULL,
  "repo_full_name"   text         NOT NULL,
  "pr_number"        integer      NOT NULL,

  -- What triggered this detection
  "trigger_type"     text         NOT NULL, -- 'pull_request' | 'issue_comment'
  "trigger_action"   text         NOT NULL, -- e.g. opened | synchronize | reopened | comment | closed | converted_to_draft

  -- Context
  "commit_sha"       text,
  "default_branch"   text,
  "target_branch"    text,

  -- Denormalized JSON payloads
  "labels_json"              jsonb,
  "changed_files_json"       jsonb,
  "impacted_projects_json"   jsonb NOT NULL, -- array of projects
  "source_mapping_json"      jsonb           -- project -> impacting_locations[]
);

-- Helpful indexes for lookups and listing latest runs per PR
CREATE INDEX IF NOT EXISTS idx_ddr_org_repo_pr_created_at
  ON "public"."digger_detection_runs" ("organisation_id", "repo_full_name", "pr_number", "created_at" DESC);

CREATE INDEX IF NOT EXISTS idx_ddr_repo_pr
  ON "public"."digger_detection_runs" ("repo_full_name", "pr_number");

CREATE INDEX IF NOT EXISTS idx_ddr_deleted_at
  ON "public"."digger_detection_runs" ("deleted_at");
```

Notes:
- We reuse GORM’s soft‑delete columns via `gorm.Model` pattern (created_at/updated_at/deleted_at). We will not update or delete rows in code.
- `impacted_projects_json` is required; empty array when zero impacted projects.

## JSON Shapes
- impacted_projects_json (array of objects) — subset of project fields we already have in memory:
```json
[
  {
    "name": "app-us-east-1",
    "dir": "infra/app",
    "workspace": "default",
    "layer": 1,
    "workflow": "default",
    "terragrunt": false,
    "opentofu": false,
    "pulumi": false
  }
]
```

- source_mapping_json (object of arrays):
```json
{
  "app-us-east-1": { "impacting_locations": ["infra/app/modules/sg", "infra/app/main.tf"] }
}
```

- labels_json / changed_files_json: arrays of strings. When unavailable (e.g., labels in comment flows), pass null or empty array.

## Model (backend/models)
Add a new model and writer. Keep it simple and append‑only.

```go
// backend/models/detection_runs.go
package models

import (
  "encoding/json"
  "gorm.io/datatypes"
  "gorm.io/gorm"
)

type DetectionRun struct {
  gorm.Model
  OrganisationID     uint
  RepoFullName       string
  PrNumber           int
  TriggerType        string
  TriggerAction      string
  CommitSHA          string
  DefaultBranch      string
  TargetBranch       string
  LabelsJSON         datatypes.JSON
  ChangedFilesJSON   datatypes.JSON
  ImpactedProjectsJSON datatypes.JSON // required
  SourceMappingJSON  datatypes.JSON
}

// CreateDetectionRun inserts an append‑only detection run row.
func (db *Database) CreateDetectionRun(run *DetectionRun) error {
  return db.GormDB.Create(run).Error
}
```

Helper mappers (in the same file) to convert from:
- `[]digger_config.Project` → lightweight `[]struct{...}` → `json.Marshal`.
- `map[string]digger_config.ProjectToSourceMapping` → `map[string]struct{ ImpactingLocations []string }` → `json.Marshal`.

## Controller Wiring
We add writes at the moment we compute impacted projects successfully — before any early returns — so runs are recorded even if later steps decide to skip work (e.g., draft PRs).

1) Pull Request events
- File: `backend/controllers/github_pull_request.go`
- After:
  - `impactedProjects, impactedProjectsSourceMapping, _, err := github2.ProcessGitHubPullRequestEvent(...)`
  - And after fetching `changedFiles` (already available)
- Insert:
  - Build the `DetectionRun` struct:
    - orgId, repoFullName, prNumber
    - trigger_type="pull_request", trigger_action=`*payload.Action`
    - commit_sha=payload.PullRequest.Head.GetSHA()
    - default_branch=`*payload.Repo.DefaultBranch`
    - target_branch=payload.PullRequest.Base.GetRef()
    - labels_json: PR label names (we already collect `labels` → `prLabelsStr`)
    - changed_files_json: from `changedFiles`
    - impacted_projects_json: from `impactedProjects`
    - source_mapping_json: from `impactedProjectsSourceMapping`
  - Call `models.DB.CreateDetectionRun(&run)`
  - On error: `slog.Error` and continue (do not fail the PR handler).

2) Issue Comment events
- File: `backend/controllers/github_comment.go`
- After:
  - `processEventResult, err := generic.ProcessIssueCommentEvent(...)`
  - Use `processEventResult.AllImpactedProjects` and `.ImpactedProjectsSourceMapping` (not the filtered subset)
  - We have `changedFiles` captured earlier in the handler
  - `prBranchName, _, targetBranch, _, err := ghService.GetBranchName(issueNumber)` → defaultBranch is `*payload.Repo.DefaultBranch`
  - `commitSha` available from earlier when loading config
- Insert `CreateDetectionRun(...)` with:
  - trigger_type="issue_comment", trigger_action="comment"
  - Same fields as PR event with the appropriate sources.

## Error Handling
- Persistence must be best‑effort: log and continue on errors to avoid impacting main workflows.
- Use concise log fields: orgId, repoFullName, prNumber, counts of impacted projects and changed files.

## Queries (examples)
- Latest detection run for a PR:
```sql
SELECT *
FROM public.digger_detection_runs
WHERE organisation_id = $1 AND repo_full_name = $2 AND pr_number = $3
ORDER BY created_at DESC
LIMIT 1;
```

- All runs for a PR:
```sql
SELECT *
FROM public.digger_detection_runs
WHERE organisation_id = $1 AND repo_full_name = $2 AND pr_number = $3
ORDER BY created_at DESC;
```

## Testing
- Unit tests:
  - Model round‑trip: marshal minimal and full payloads (empty impacted projects; multiple projects; multiple source locations) and `CreateDetectionRun` succeeds.
- Controller integration tests (lightweight):
  - Simulate a PR event with no impacted projects → one row with empty `impacted_projects_json`.
  - Simulate a PR event with 2 impacted projects → row with expected JSON arrays.
  - Simulate an issue comment event → row with trigger_type="issue_comment".

## Rollout
- Add migration.
- Add model + writer method.
- Wire controllers (PR and Issue Comment) to create detection runs.
- Deploy; no backfill required. Data accrues on subsequent events.

## Risks / Considerations
- Size of JSON fields: on very large PRs, `changed_files_json` can be big; acceptable for audit purposes, can be truncated later if needed.
- Ordering by timestamp: adequate for our needs; if we ever need strict monotonic ordering under rare clock drifts, we could fall back to ID ordering as a tie‑breaker (`ORDER BY created_at DESC, id DESC`).
- Privacy: Paths and labels are internal to the repo; acceptable within backend storage context.

## Work Items
1) Create migration file `backend/migrations/20251107000100.sql` with schema above.
2) Add `backend/models/detection_runs.go` with `DetectionRun` and `CreateDetectionRun`.
3) Add light mappers for JSON serialization of projects and source mapping.
4) PR controller: write detection run after computing impacts.
5) Comment controller: write detection run after computing impacts.
6) Add basic unit tests for model creation; optional controller tests.

