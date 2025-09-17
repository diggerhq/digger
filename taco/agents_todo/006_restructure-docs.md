# 006 — Restructure Docs: “State Management” Now + Roadmap Buckets

## Goal
Reframe the documentation so it clearly communicates that OpenTaco today is primarily a Terraform/Tofu state manager, and introduce first‑class “Roadmap” buckets (Remote Runs, VCS Integration + UI, Drift Detection, Policies) with dedicated “How it would work” pages. Keep all current behavior accurate; do not over‑promise. This is a docs‑only change.

## Context & References
- Current docs live in `opentaco/docs/` (Mintlify)
- Background: `AGENTS.md`, `agents_context/*.md` (layering, state+RBAC focus)
- Current nav: `docs/mint.json`
- Current pages to reuse/tighten: `overview.md`, `getting-started.md`, `cli.md`, `service-backend.md`, `s3-compat.md`, `provider.md`, `storage.md`, `dependencies.md`, `rbac.md`, `auth_config_examples.md`, `final_spec_state_auth_sts.md`, `reference/*`, `demo.md`, `troubleshooting.md`

## Out of Scope
- No API/CLI semantics changes, no code changes; docs only.
- No promises of timelines; roadmap is sequencing and concept only.
- Do not regress or contradict current working behavior (S3 bucket‑only adapter, HTTP backend, S3‑compat endpoint, dependencies/status, Auth/STS, CLI/provider). 

## Deliverables
1) A reworked top‑level Information Architecture (nav) centered on “State Management” as the current core, plus a “Roadmap” section with future buckets.
2) New pages:
   - `docs/scope-today.md` — What exists today (capabilities, constraints, non‑goals). Status: Stable.
   - `docs/roadmap.md` — Layered roadmap overview. Status: Planned.
   - `docs/roadmap-remote-runs.md` — How Remote Runs would work (CLI‑only first). Status: Planned.
   - `docs/roadmap-vcs-ui.md` — VCS Integration + UI (repo connections begin here). Status: Planned.
   - `docs/roadmap-drift.md` — Drift Detection vision. Status: Planned.
   - `docs/roadmap-policies.md` — Policies vision. Status: Planned.
   - `docs/auth-sts.md` — Consolidated Auth & STS doc (link out to `auth_config_examples.md`). Status: Beta.
3) Page edits to reinforce the new message and add page‑top status cues across existing pages.
4) Updated `docs/mint.json` navigation reflecting the new structure (see below).
5) Fix `docs/rbac.md` merge conflict markers; scope it as Experimental, accurately reflecting current capabilities/limits.
6) README pointer: a single‑line positioning and links to “Scope Today” + “Roadmap”.

## Top‑Level IA (Mintlify)
Groups and pages (update `docs/mint.json` accordingly):

- Overview
  - `overview`
  - `scope-today`  (new)
  - `roadmap`      (new)

- State Management (current)
  - `getting-started`
  - `cli`
  - `service-backend`
  - `s3-compat`
  - `provider`
  - `storage`
  - `dependencies`

- Security
  - `auth-sts`     (new; consolidates auth content)
  - `rbac`         (cleaned up; Experimental)

- Roadmap
  - `roadmap-remote-runs`  (new)
  - `roadmap-vcs-ui`       (new)
  - `roadmap-drift`        (new)
  - `roadmap-policies`     (new)

- Reference
  - `reference/api`
  - `reference/cli-commands`
  - `reference/terraform-backend`

- Guides
  - `demo`
  - `troubleshooting`

- Development
  - `development`

Note: We only need to change nav (mint.json); file paths can stay in `docs/` as listed above.

## Status labels (add near top of each page)
- Stable: CLI, Service & HTTP backend, Provider, Storage (S3 adapter), Dependencies & Status.
- Beta: S3‑compatible Backend, Auth & STS.
- Experimental: RBAC (S3 storage required, permissive stub; scope tightly worded).
- Planned: All roadmap bucket pages.

Implementation: add a short “Status: <label>” line under the title/description of each page. Keep messaging concise.

## New pages: skeletons and content notes

1) `docs/scope-today.md` (Stable)
   - Title: Scope Today
   - Sections: Capabilities (bulleted), Non‑Goals (explicitly: remote runs, PR automation, UI, drift, policies), Constraints (stateless svc; S3 bucket‑only storage; auth defaults), Quick links to State Management pages.

2) `docs/roadmap.md` (Planned)
   - Title: Roadmap (Layers)
   - Sections: Layer‑0 (Now), Remote Runs (Next), VCS Integration + UI, Drift Detection, Policies; each 3–5 bullets and link to its bucket page; “subject to change” note.

3) `docs/roadmap-remote-runs.md` (Planned)
   - Problem/Goals; User journeys (CLI triggers plan/apply remotely; watch/logs); High‑level design (run submission API, locks, compute adapter starting with GitHub Actions); Rough API surfaces; Open questions.

4) `docs/roadmap-vcs-ui.md` (Planned)
   - Repo connections, mapping paths → units; webhooks for PR events; plan previews; minimal UI for connections and visibility; auth for VCS install; RBAC interactions.

5) `docs/roadmap-drift.md` (Planned)
   - Read‑only drift checks; scheduling/triggering; storing minimal drift metadata; surfacing in CLI/UI; not blocking writes; rate‑limit notes.

6) `docs/roadmap-policies.md` (Planned)
   - Policy packs (OPA/Sentinel‑like); evaluation before apply; CLI enforcement/server‑side checks; audit trail; storing policy state alongside system unit.

7) `docs/auth-sts.md` (Beta)
   - Consolidate auth overview, PKCE login, tokens, `/v1/auth/*` endpoints summary, and STS creds for S3‑compat; link to `auth_config_examples.md`; clarify dev flags and default auth posture.

## Edits to existing pages
- `docs/index.mdx`: lead with “OpenTaco is a state manager today.” Prominent links to Scope Today + Roadmap.
- `docs/overview.md`: clarify “state management” headline; add “Not in scope yet” bullets; link to Roadmap.
- `docs/getting-started.md`: one‑line preface that today focuses on state management.
- `docs/service-backend.md`, `docs/s3-compat.md`, `docs/dependencies.md`, `docs/cli.md`, `docs/provider.md`, `docs/storage.md`: add Status line; add cross‑links back to Scope Today.
- `docs/rbac.md`: remove conflict markers, scope it clearly (S3 storage required; permissive defaults; label Experimental); ensure command examples align with existing CLI/server capabilities.
- `README.md`: add one‑liner “State management today; Remote Runs → VCS+UI → Drift → Policies next.” Link to Scope Today and Roadmap.

## Acceptance Criteria
- Navigation
  - `docs/mint.json` shows the new groups and pages exactly as defined above.
  - All links resolve; pages render without errors.
- Content
  - New pages created with the described skeletons; they clearly say “Planned” where applicable.
  - All core pages (State Management + Security) have a visible Status label (Stable/Beta/Experimental).
  - `rbac.md` has no merge conflict markers; scope matches current implementation.
  - `auth-sts.md` consolidates auth content and cross‑links examples.
  - `index.mdx` and `overview.md` lead with “state manager today,” and link to Scope Today + Roadmap.
- Repo hygiene
  - Only files under `opentaco/docs/` (and README) are modified.
  - No code or API changes.

## Steps
1) Create new pages with initial content and Status labels.
2) Update `docs/mint.json` nav to new IA.
3) Add Status labels to existing core pages, tighten intros, and add cross‑links.
4) Clean up `docs/rbac.md` (remove conflict markers; scope as Experimental).
5) Add `docs/auth-sts.md` and link from S3‑compat + Getting Started.
6) Update `index.mdx`, `overview.md`, and `README.md` to reflect “state manager today” and link to Scope Today/Roadmap.
7) Click‑through QA for links and nav; fix any broken references.

## Notes & Gotchas
- Keep “Reference” pages as specs; keep conceptual/how‑to content in non‑Reference pages and cross‑link.
- Do not imply timelines on roadmap pages; include a neutral “subject to change” line.
- Maintain parity with current implemented behavior (e.g., explicit `LOCK/UNLOCK` routes; `?ID=` lock handling; S3 layout; dependency digests semantics).

