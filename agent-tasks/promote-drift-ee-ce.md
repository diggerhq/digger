# Promote Drift from EE to CE

- Target: CE availability, separate service under `drift/`
- Related: `ee/drift` (source), `backend` (models/middleware), `libs`

## Goal
Make drift functionality available in Community Edition by moving the existing EE drift service out of `ee/` and into top-level CE, with minimal disruption to existing deployments.

## Non-Goals
- Redesign of feature behavior or APIs.
- Merging the drift service into the monolithic `backend` app (can be a later refactor).
- Moving EE-only CLI features (advanced Slack aggregation) to CE.

## Approach
- Keep drift as a standalone CE service at `drift/` (new module). Alternative would be to merge drift endpoints into CE backend but we want to keep potential breaking changes to a minimum.

## Current State Summary
- `ee/drift` is a Go module that depends on CE code: `backend/models`, `backend/utils`, `backend/ci_backends`, and `libs` packages. It exposes:
  - `/_internal/process_drift`
  - `/_internal/process_drift_for_org`
  - `/_internal/trigger_drift_for_project`
  - `/_internal/process_notifications`
  - Slack test/real notification endpoints
- DB models already include drift fields in CE (`backend/models/orgs.go`, `Project`, `Organisation`). No migrations required.
- `ee/backend` imports `ee/drift/middleware` only for context keys; CE already defines these keys in `backend/middleware`.
- Build: `Dockerfile_drift` builds `./ee/drift` and copies `ee/backend/templates` (templates unused by drift).
- Workspace: `go.work` includes `./ee/drift`.


## Acceptance Criteria
- Code builds:
  - `go build ./drift`, `go build ./backend`, `go build ./ee/backend` succeed.
- No imports from `ee/drift` remain in CE modules.
- `go.work` references `./drift`; `./ee/drift` remains as a wrapper module present in workspace.
- Drift endpoints respond 200 with correct auth and perform expected side effects (project drift job creation and notifications).
- `ee/backend` uses `backend/middleware` for context keys and builds without `ee/drift` dependency.
- `Dockerfile_drift` successfully builds and runs the CE drift service.
- EE wrapper builds and runs: `go build ./ee/drift` and boot equivalently to CE.

## Risks & Mitigations
- Import drift: Missed references → repo-wide search and CI builds on all modules.
- Auth mismatch: Align on `InternalApiAuth` or accept both secrets to avoid outages.
- Duplicate GitHub client: Remove only after ensuring CE utils cover all use cases.


## Follow-ups (Later; not in this plan)
- Provide a dedicated EE wrapper image (`Dockerfile_drift_ee`) if distribution needs it.
- Consolidate middleware/utils and remove duplicates once parity is verified.
- Remove duplicate GitHub client from drift utils after verifying CE `backend/utils` covers all use-cases.
- Consider merging drift into CE `backend` later to reduce services.
- Promote EE advanced Slack aggregator to CE, or document as EE-only.
- Unify on a single internal secret (`DIGGER_INTERNAL_SECRET`) and deprecate `DIGGER_WEBHOOK_SECRET`.

## Commit-by-Commit Execution Plan

1) Add CE drift module
- Action: Copy `ee/drift` to `drift/`. Update `drift/go.mod` module path to `github.com/diggerhq/digger/drift`; keep `replace` entries to `../libs` and `../backend`. Update internal self-imports from `github.com/diggerhq/digger/ee/drift/...` to `github.com/diggerhq/digger/drift/...`.
- Files: `drift/**` (new), `drift/go.mod`, `drift/go.sum`.
- Verify: `go build ./drift` compiles.

2) Update Dockerfile for CE drift
- Action: Point `Dockerfile_drift` build target to `./drift`, copy entrypoint from `drift/scripts/entrypoint.sh`, and drop copying `ee/backend/templates` (not used).
- Files: `Dockerfile_drift`.
- Verify: `docker build -f Dockerfile_drift .` reaches build of `./drift` (local build or CI).

3) Add CE drift to workspace
- Action: Add `./drift` to `go.work` (keep `./ee/drift` for now; wrapper will replace it later). Run `go work sync` if applicable.
- Files: `go.work`.
- Verify: `go list` resolves `./drift` module in workspace.

4) Decouple EE backend from EE drift
- Action: In `ee/backend`, replace imports of `github.com/diggerhq/digger/ee/drift/middleware` with `github.com/diggerhq/digger/backend/middleware`. Remove `github.com/diggerhq/digger/ee/drift` requirement from `ee/backend/go.mod` and run `go mod tidy`.
- Files: `ee/backend/controllers/**`, `ee/backend/go.mod`, `ee/backend/go.sum`.
- Verify: `go build ./ee/backend` compiles without `ee/drift` dependency.

5) Add EE wrapper (tiny main)
- Action: Replace existing `ee/drift` implementation with a minimal `main.go` that mirrors CE drift startup but imports CE packages from `github.com/diggerhq/digger/drift/...`. Update `ee/drift/go.mod` to depend on CE drift module; remove old `controllers/`, `middleware/`, `services/`, `utils/`, `scripts/` (except keep any wrapper-specific entrypoint if needed).
- Files: `ee/drift/main.go` (new), `ee/drift/go.mod` (updated), remove old `ee/drift/**` dirs.
- Verify: `go build ./ee/drift` compiles and starts with same endpoints.

6) Documentation updates
- Action: Update docs to note drift is CE; list env vars; note EE wrapper intent and parity.
- Files: `README.md` (or relevant docs), `agent-tasks/promote-drift-ee-ce.md` (mark steps done as they land).
- Verify: Docs render and link paths are valid.

7) Promote GitHub Issues notifier to CE
- Action: Remove EE dependency for Issues notifications by promoting the Issues notifier into CE.
  - Add CE implementation: `cli/pkg/drift/github_issue.go` (copy/adapt from `ee/cli/pkg/drift/github_issue.go`).
  - Extend CE provider `cli/pkg/drift/Provider.go` to honor `INPUT_DRIFT_GITHUB_ISSUES` and return the CE `GithubIssueNotification` when set.
  - Keep Slack path unchanged; no changes to EE advanced Slack aggregation.
  - Update docs workflow to remove `ee: 'true'` requirement for GitHub Issues notifications.
- Files: `cli/pkg/drift/Provider.go`, `cli/pkg/drift/github_issue.go`, `docs/ce/features/drift-detection.mdx`.
- Verify:
  - `go build ./cli` succeeds.
  - Running with `INPUT_DRIFT_GITHUB_ISSUES='true'` uses CE notifier (no EE required).
  - Docs reflect CE-only workflow (no `ee: 'true'`).

## Verification Steps (per commit, non-code)
- Build touched modules: `go build ./drift`, `go build ./backend`, `go build ./ee/backend`, `go build ./ee/drift` (after step 5).
- Smoke tests for drift endpoints via curl with proper auth headers.
- CI passes for modules affected in each commit.

## Implementation Notes

Q: Functionality-wise, how is drift middleware different from backend middleware? Should we prefer using CE drift middleware (copy of EE) for safety?

A:
- Constants parity: Both expose the same context keys (`ORGANISATION_ID_KEY="organisation_ID"`, `ACCESS_LEVEL_KEY="access_level"`). EE backend only needed these keys, so switching to `backend/middleware` is safe.
- Webhook auth:
  - Drift’s `WebhookAuth()` validates `DIGGER_WEBHOOK_SECRET` and reads optional `X-Digger-Org-ID`.
  - Backend’s `InternalApiAuth()` validates `DIGGER_INTERNAL_SECRET` and also supports `X-Digger-Org-ID`.
  - EE backend wasn’t using drift’s webhook auth; only constants, so no behavior change.
- Job token auth:
  - Drift’s `JobTokenAuth()` accepts `cli:` tokens and sets org/access level.
  - Backend’s `JWTBearerTokenAuth()` and `NoopApiAuth()` already handle `cli:` tokens (and also API `t:` tokens and JWTs) with the same org/access-level semantics.
  - EE backend doesn’t depend on drift’s job token middleware.
- Integration boundary: Backend and drift integrate via shared DB (`backend/models`) and external calls; backend doesn’t call drift endpoints today.
- If closer drift parity is desired later, import CE drift middleware in relevant services or add a backend wrapper that accepts both `DIGGER_WEBHOOK_SECRET` and `DIGGER_INTERNAL_SECRET`.

New finding: EE CLI required for GitHub Issues notifications (new step 7)
- Observation: The GitHub Issues notification path for drift currently lives in the EE CLI (`ee/cli/pkg/drift/provider.go` and related), selected via `INPUT_DRIFT_GITHUB_ISSUES`. Our Action also toggles EE CLI usage via `ee: 'true'`.
- Impact: Although drift detection is now CE, using GitHub Issues notifications imposes an EE dependency (the example workflow requires `ee: 'true'`). Slack notifications remain CE-only and do not require EE.
- Proposed fix (upcoming commit): Promote the GitHub Issues drift notification provider to CE by adding a CE implementation (e.g., `cli/pkg/drift/github_issue.go`) and extending the CE provider to honor `INPUT_DRIFT_GITHUB_ISSUES`. Update docs to remove the `ee: 'true'` requirement for Issues after the change.
- Interim docs: Until the provider is promoted to CE, users need `ee: 'true'` in the workflow to enable GitHub Issues notifications for drift.
