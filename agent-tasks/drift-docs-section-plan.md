# Drift Documentation: New Section Plan

## Goals

- Create a dedicated Drift section (parallel to State Management / Self-host Digger) that clearly explains concepts, setup, operations, and troubleshooting.
- Reduce fragmentation by moving/adapting current content under a coherent information architecture (IA).
- Fill current gaps: org/project settings, scheduling strategies, remediation, and self-host specifics.

## Current Inventory (what exists today)

- Features → Drift Detection: `docs/ce/features/drift-detection.mdx`
  - Covers: GH Actions scheduled workflow for drift; Slack and GitHub Issues examples; note about 403/no-backend.
- How To → Scope Drift Detection: `docs/ce/howto/scope-drift-detection.mdx`
  - Covers: Using `digger-filename` to limit drift to certain projects.
- Reference → Action inputs: `docs/ce/reference/action-inputs.mdx`
  - Mentions: `mode: drift-detection`, `drift-detection-slack-notification-url`, `digger-filename`, `no-backend`.
- Codebase capabilities (underdocumented in docs):
  - Org-level drift settings: enable/disable, cron string, Slack webhook URL (`/api/orgs/settings/`).
  - Project-level drift enable flag (`/api/projects/:project_id`).
  - Internal drift service endpoints and scheduler hooks (for backend-driven scans and Slack rollups).
  - Plan snapshots and counts stored per project; status surfaced in UI/API.

## Proposed Section Name and Nav

- Group: Drift.
- Placement: Top-level group in `docs/mint.json` after “Getting Started” and “State Management”.
- Remove “Drift Detection” from Features; move/retitle into this new section.

## Information Architecture (pages and purpose)

1) `ce/drift/overview.mdx`
   - What is drift, why it matters, how Digger detects it (scheduled plans on default branch per project).
   - Two operating modes: GitHub Actions-only vs backend-orchestrated scans.
   - Optional high-level mention of drift states (no dedicated page).

2) `ce/drift/how-it-works.mdx`
   - Architecture: drift as a separate service with a cron-driven scheduler; internal endpoints and DB updates.
   - Backendless mode: running entirely via GitHub Actions with `mode: drift-detection`.
   - When to choose each approach.

3) `ce/drift/quickstart-github-actions.mdx`
   - Minimal GH Actions workflow using `mode: drift-detection` and a cron.
   - Includes a note that it is "backendless mode"
   - Tips: install Terraform (`setup-terraform: true`), store webhook secret, use `no-backend: true` when applicable.

4) `ce/drift/scoping-projects.mdx`
   - Consolidate/replace How To → Scope Drift Detection.
   - Techniques: dedicated `digger.yml` via `digger-filename`, project `drift_detection: false` in digger.yml, branch filters, dependency considerations.

5) `ce/drift/notifications.mdx`
   - Slack rollup notifications: format, frequency, org-level webhook, test endpoint, common errors.
   - Mentions GitHub Issues exist for drift, with a link to the dedicated page below.

6) `ce/drift/github-issues.mdx`
   - Dedicated setup and permissions for creating GitHub Issues during drift.
   - What’s in the issue, labels, ownership, and housekeeping guidance.

7) `ce/drift/remediation.mdx`
   - How to remediate drift using GitHub Issues integration.
   - Describe remediation by commenting `digger apply` in the GitHub issue; include a clear note that this flow is only relevant for the GitHub Issues integration.

8) `ce/drift/scheduling.mdx`
   - GH Actions cron examples (simple mode).
   - Backend scheduling (self-host/Cloud): org-level `drift_cron_tab` and how scans are batched; internal endpoints overview.

9) `ce/drift/self-host.mdx`
   - Running the drift service (`Dockerfile_drift`), required env vars: `DIGGER_HOSTNAME`, `DIGGER_WEBHOOK_SECRET`, `DIGGER_APP_URL`, `DIGGER_DRIFT_REPORTER_HOSTNAME`.
   - Database/scheduler notes (pg_cron/pg_net SQL jobs in `drift/scripts/cron/*.sql`).
   - Securing internal endpoints, topology, scaling.

10) `ce/drift/troubleshooting.mdx`
    - 403/permissions during drift reporting → often missing `no-backend: true` in Actions.
    - Slack webhook failures and how to test.
    - Issues creation failures (token scopes), large plan outputs, runner timeouts.

## Content Outlines (key sections per page)

- Overview
  - Concept, detection flow description, supported IaC; optional brief mention of states.

- How It Works
  - Separate service architecture and cron; internal endpoints; backendless mode explained.

- Quickstart (GH Actions)
  - YAML snippet; secrets; validation; verification steps; example outputs.

- Scoping Projects
  - `digger-filename` technique; per-project `drift_detection` flag; branch selection considerations.

- Notifications
  - Slack rollup design (aggregated per org), color legend, test endpoint; direct link to GitHub Issues page.

- GitHub Issues
  - Enablement, required env vars/permissions, example outputs, labels/assignment.

- Remediation
  - Comment `digger apply` in the GitHub issue to remediate drift (only relevant for Issues integration).

- Scheduling
  - GH Actions cron; backend scheduler overview (SQL jobs in `drift/scripts/cron/`), `drift_cron_tab` semantics; examples.

- Self-host
  - Deploying the drift service; required components; env vars; security; health checks; scaling.

- Troubleshooting
  - Symptoms, likely causes, resolutions; logs to check; sample curl commands for internal endpoints (self-host operators).

## Migration and Link Strategy

- Move: `ce/features/drift-detection` → `ce/drift/quickstart-github-actions` (rename and retitle).
- Move: `ce/howto/scope-drift-detection` → `ce/drift/scoping-projects` (update links site-wide).
- Update: cross-links in `action-inputs.mdx` to reference new “Quickstart”, “Notifications”, and “Scheduling” pages.

## Navigation Changes (mint.json)

- Add a top-level group:
  - Drift → pages:
    - `ce/drift/overview`
    - `ce/drift/how-it-works`
    - `ce/drift/quickstart-github-actions`
    - `ce/drift/scoping-projects`
    - `ce/drift/notifications`
    - `ce/drift/github-issues`
    - `ce/drift/remediation`
    - `ce/drift/scheduling`
    - `ce/drift/self-host`
    - `ce/drift/troubleshooting`
- Remove `ce/features/drift-detection` from “Features”.
- Remove `ce/howto/scope-drift-detection` from “How To”.

## Acceptance Criteria

- New Drift group appears as a dedicated section with the listed pages.
- Quickstart reproducible end-to-end (Slack and Issues options tested).
- Remediation page accurately reflects the supported comment flow for Issues integration.
- Self-host and How It Works pages list required env vars and components and reference the SQL scheduler snippets.

## Implementation Plan (steps)

1. Create new directory and placeholder pages under `docs/ce/drift/` with titles and short summaries.
2. Move/adapt content from `features/drift-detection` and `howto/scope-drift-detection` into the new pages.
3. Update `docs/mint.json` to add the Drift group; remove moved pages from Features/How To.
4. Update cross-links in the docs (search for references to old paths and fix).
5. QA pass: local build of docs, link checker, skim for tone and consistency.

## References (code pointers for authoring)

- Action inputs mentioning drift: `docs/ce/reference/action-inputs.mdx`.
- Current feature page: `docs/ce/features/drift-detection.mdx`.
- How-to: `docs/ce/howto/scope-drift-detection.mdx`.
- Backend endpoints and models: `backend/controllers/orgs.go`, `backend/controllers/projects.go`, `backend/bootstrap/main.go`, `backend/models/orgs.go`.
- Issue comment handling: `backend/controllers/github_comment.go`.
- Drift service and scheduler: `drift/controllers/*.go`, `drift/scripts/cron/*.sql`.
