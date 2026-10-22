# Allow Apply When Only `digger/apply` Blocks Mergeability

- Related issues: #1150, #959
- Areas: `libs/ci/github`, `cli/pkg/digger`, `backend/controllers`, `docs`

## Goal
Unblock `digger apply` when GitHub reports PR mergeability state as "blocked" but the only blocking status checks are Digger’s own aggregate/per‑project apply checks (`digger/apply` and `*/apply`).

## Problem / Context
- We gate apply on mergeability: `IsMergeable()` must be true, otherwise `digger apply` errors: "cannot perform Apply since the PR is not currently mergeable".
- When `digger/apply` is configured as a required status, GitHub reports the PR as `mergeable=false` with `mergeable_state=blocked` until apply completes.
- This creates a circular dependency: apply is needed to make the PR mergeable, but apply refuses to run because the PR is not mergeable.

Current code (GitHub):
- `IsMergeable(prNumber)` returns `pr.GetMergeable() && isMergeableState(pr.GetMergeableState())` where allowed states are `clean`, `unstable`, `has_hooks`.
- Apply gate uses this in two places:
  - `cli/pkg/digger/digger.go` before running apply.
  - `libs/apply_requirements.CheckApplyRequirements` when `apply_requirements` includes `mergeable` (default).

## Non-Goals
- Changing auto-merge behavior after successful applies (we already require combined status `success`).
- Implementing Checks API inspection (we only need to introspect commit statuses we set today).
- Changing mergeability logic for non-GitHub providers.

## Approach
Augment GitHub mergeability logic to handle the chicken‑and‑egg case:

1) Keep existing fast path: if REST `mergeable==true` and `mergeable_state` in {`clean`, `unstable`, `has_hooks`}, return true.
2) If `mergeable_state == "blocked"`, fetch combined commit statuses for the PR head SHA.
3) If every non‑success context is either:
   - `digger/apply` (aggregate), or
   - per‑project apply contexts matching `*/apply` (e.g., `projectA/apply`),
   then treat the PR as mergeable for the purpose of running apply. Otherwise, return false.
4) Log diagnostics: mergeable bool, mergeable_state, and the list of non‑success contexts checked.

Notes
- Reviews/draft can also yield `blocked` but won’t appear in commit statuses. If users want to require approvals, they should include `approved` in `apply_requirements` (separate gate already exists). We deliberately do not block here based on reviews unless configured via `apply_requirements`.
- We continue to respect `skip_merge_check` and project-level `apply_requirements` semantics.

## Acceptance Criteria
- When a PR is `mergeable_state=blocked` and the only non‑success statuses are `digger/apply` and `*/apply`, `digger apply` proceeds (no "not currently mergeable" error).
- If any other non‑success status exists (e.g., tests, lint, `digger/plan`, etc.), `digger apply` remains blocked.
- Auto-merge flow is unchanged and still requires combined status `success`.
- Logs include helpful diagnostics showing the evaluated mergeable state and non‑success contexts.
- Documentation reflects the new behavior and updates the known-issues note.

## Implementation Plan

1) Add pure evaluator (unit-testable)
- File: `libs/ci/github/github.go`
- Function: `isMergeableForApply(mergeable bool, mergeableState string, nonSuccessContexts []string) bool`
  - Return true if `(mergeable && isMergeableState(mergeableState))` OR `(mergeableState == "blocked" && all nonSuccessContexts ∈ {"digger/apply" ∪ contexts with suffix "/apply"})`.
  - Treat statuses with state in {`pending`, `failure`, `error`} as non‑success.

2) Gather non‑success contexts from GitHub
- In `GithubService.IsMergeable(prNumber)`:
  - Fetch PR via REST (already done).
  - Fetch combined statuses for the PR head SHA (we already have `GetCombinedPullRequestStatus`; add a sibling helper to return raw contexts and their states).
  - Build `nonSuccessContexts` from the combined statuses where state != `success`.
  - Return `isMergeableForApply(pr.GetMergeable(), pr.GetMergeableState(), nonSuccessContexts)`.
  - Add `slog.Debug` with fields: `mergeable`, `mergeableState`, and `nonSuccessContexts`.

3) Wire through existing gates (no interface changes)
- `cli/pkg/digger/digger.go`: keep using `prService.IsMergeable()` (behavior now includes the new allowance under GitHub).
- `libs/apply_requirements/apply_requirements.go`: no changes, it will inherit new behavior.

4) Tests
- Add unit tests for `isMergeableForApply` (table‑driven) covering:
  - mergeable true + clean/unstable
  - blocked + only `digger/apply`
  - blocked + `projectA/apply` and `projectB/apply`
  - blocked + additional failing context (e.g., `ci/test`) → false
  - non‑blocked + mergeable=false → false
- Optional: add an integration-ish test for `GithubService.IsMergeable` using a small seam to inject combined status data (or validate via a thin wrapper if direct client mocking is hard).

5) Docs
- Update `docs/ce/howto/apply-requirements.mdx`:
  - Adjust the note on mergeability to state that Digger now proceeds with apply if the only blocker is Digger’s own apply status.
  - Keep guidance about avoiding circular required checks for other providers or check-run based integrations.

## Code Pointers
- GitHub mergeability and statuses:
  - `libs/ci/github/github.go`: `IsMergeable`, `GetCombinedPullRequestStatus`, `isMergeableState`
- Apply gates:
  - `cli/pkg/digger/digger.go`: mergeability check before apply
  - `libs/apply_requirements/apply_requirements.go`: mergeable requirement
- Status producers:
  - `backend/utils/github.go`: sets aggregate `digger/plan` and `digger/apply`
  - per‑project statuses: same file via `job.GetProjectAlias()+"/apply"`

## Risks & Mitigations
- Missed non‑success contexts (Checks API): We only inspect commit statuses; if users require check runs (not statuses), behavior remains unchanged. Document this and consider future enhancement for Checks API.
- Over‑permissive in rare cases: If other blockers do not surface as statuses (e.g., code owners, required reviews) and users don’t add `approved` requirement, apply might run despite reviews missing. This aligns with current semantics and can be controlled via `apply_requirements`.

## Rollout
- Feature is effectively on by default via updated `IsMergeable` logic for GitHub only.
- Observe logs for `nonSuccessContexts` on real repos; rollback by reverting the helper usage if needed.

## Verification Steps
- Scenario A: Only `digger/apply` pending
  - Observe `mergeable_state=blocked`; statuses show only `digger/apply` (and/or `*/apply`) pending → `digger apply` runs.
- Scenario B: Additional pending status (e.g., tests)
  - With `ci/test` pending → `digger apply` remains blocked.
- Scenario C: Clean/unstable
  - `mergeable=true`, `state=clean|unstable` → unchanged, `digger apply` runs.

