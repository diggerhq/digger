# CE Backend: Port Drift Reconciliation Hook

## Goal
Enable drift reconciliation in Community Edition: when a user comments "digger apply" or "digger unlock" on a drift Issue (non-PR) created by the drift service, the CE backend should trigger the appropriate jobs and manage locks — parity with EE behavior.

## Findings
- EE registers `hooks.DriftReconcilliationHook` via `ee/backend/main.go` and implements it in `ee/backend/hooks/github.go`.
- CE exposes `GithubWebhookPostIssueCommentHooks` but does not register a hook; `backend/hooks` has no implementation. As a result, CE ignores drift Issue comments.

## Scope
- CE functional addition; no behavior change for PR comments.
- Wire the new hook into CE `backend` and dedupe EE to reuse the CE hook in the same change (not optional).

## Constraints
- Keep implementation as-is: copy the EE hook logic verbatim, only adjusting import/package paths for CE. Do not introduce new logic, behaviors, refactors, or changes to allowed commands.

## Plan
1) Implement CE hook
- Add `backend/hooks/drift_reconciliation.go` exporting:
  - `var DriftReconcilliationHook controllers.IssueCommentHook`
- Copy logic from `ee/backend/hooks/github.go` with CE imports only: `backend/*` and `libs/*`.
- Behavior:
  - Only handle IssueComment events on Issues (ignore PR comments).
  - Issue title must match `^Drift detected in project:\s*(\S+)` to extract `projectName`.
  - Accept commands: `digger apply` and `digger unlock`.
  - Lock project, run jobs for the target project on apply, then unlock (mirroring EE flow).
  - Post reactions and reporter comments as in EE.

2) Wire into CE backend
- Update `backend/main.go` to register:
  - `GithubWebhookPostIssueCommentHooks: []controllers.IssueCommentHook{hooks.DriftReconcilliationHook}`

3) Dedupe EE to reuse CE hook
- Switch `ee/backend/main.go` to import `github.com/diggerhq/digger/backend/hooks` and remove EE-local hook implementation.

4) Verification
- Build: `go build ./backend` and `go build ./ee/backend`.
- Manual: Comment `digger apply` on a generated drift Issue; verify locks and jobs are triggered; `digger unlock` removes locks.

## Acceptance Criteria
- CE backend reacts to drift Issue comments (not PRs) with title pattern above.
- `digger apply` triggers jobs for the extracted project and unlocks afterward.
- `digger unlock` removes locks and acknowledges success.
- No regressions to PR comment handling; existing PR workflows remain unchanged.
- `go build ./backend`, `go build ./ee/backend` succeed.

## Tasks Checklist
- [ ] Add `backend/hooks/drift_reconciliation.go` with CE implementation.
- [ ] Register hook in `backend/main.go`.
- [ ] Build CE and EE backends.
- [ ] Point EE to CE hook and delete EE duplicate.
- [ ] Smoke-test via Issue comments.

## Notes
- Hook name keeps EE spelling (`DriftReconcilliationHook`) for parity; consider a later rename only if safe.
- Keep allowed commands restricted to `digger apply` and `digger unlock` for drift Issues.
