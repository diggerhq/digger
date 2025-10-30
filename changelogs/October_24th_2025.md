## Week ending 10/10

We’re closing in on launch date!!! Below are some of the highlights of what we’ve been cooking.

Latest version: **v0.6.128**

## 2327

**PR #2327: “feat/ui units”**  by @motatoes adds a complete **“**Units**” section to** OpenTaco’s dashboard, unifying Terraform state management, multi-org identity, and backend integration into a single UI.

It introduces a new route (`/dashboard/units`) and pages for listing, creating, locking, unlocking, and deleting Terraform state units. 

Each unit represents a state file with version history and dependencies. Users can view metadata (size, updated time, status), restore old versions, and see Terraform setup snippets. 

Behind the scenes, the PR connects the UI to Statesman, the state-storage microservice, via signed internal APIs (`statesman_units.ts`, `statesman_serverFunctions.ts`). 

Each operation (fetch, create, delete) authenticates through `STATESMAN_BACKEND_WEBHOOK_SECRET`. Parallel refactors modularize Orchestrator APIs into `orchestrator_orgs`, `orchestrator_users`, and `orchestrator_repos`, clarifying domain boundaries between configuration and state.

In short, this release turns OpenTaco from a Terraform automation runner into a self-hosted Terraform Cloud alternative with state versioning, locking, and multi-org identity support!


## 2328

PR #2328 “fix/contributing guide AI assistance” adds a new AI Assistance Disclosure Policy and integrates it into both the pull request template and the CONTRIBUTING.md file for the Digger repository

This change formalizes Digger’s stance on responsible AI-assisted contributions. Inspired by [Ghostty’s open-source policy](https://github.com/ghostty-org/ghostty/blob/main/CONTRIBUTING.md#ai-assistance-notice), the policy ensures transparency whenever AI tools are used during development or documentation.



## 2329


PR #2329 : “UUIDs and Org Scoping” by @breardon2011 is a major architectural migration that converts the entire OpenTaco backend from name-based identifiers to UUID-based, organization-scoped resources, enabling full multi-tenant isolation across orgs, users, units, roles, and permissions.

The PR introduces a unified UUID schema across all database tables (`units`, `roles`, `permissions`, `organizations`, `users`, `tags`) and refactors all APIs, repositories, and RBAC logic to use explicit `org_id` foreign keys.

Every state, token, or RBAC record now lives under a unique organization UUID rather than shared global namespaces
