## Week ending 10/17

This one’s a bit of a delayed update, we’ve been heads down working on an upcoming launch!

But here goes, here’s the highlights of what we shipped this week!

Latest version: **v0.6.128**

- DB Query Engine [#2265](https://github.com/diggerhq/digger/pull/2265) by @breardon2011
    
    This is one of the largest backend upgrades in OpenTaco’s architecture.
    
    It introduces a modular database-backed query engine for indexing and querying Terraform state (“units”) - enabling RBAC and analytics via SQL rather than raw S3 listing
    
    This PR adds a database abstraction layer (“query backend”) that sits between the blob store (S3 or memory) and higher-level features like:
    
    - RBAC (Role-Based Access Control)
    - Drift detection and indexing
    - Fast state listing and metadata querying
    - Future analytics and query APIs
    
    By default, it uses **SQLite**, but can be swapped for **PostgreSQL, MySQL, or MSSQL** via environment configuration. We’re VERY excited about this!
    
- UI [#2314](https://github.com/diggerhq/digger/pull/2314) by @motatoes
    
    PR #2314 establishes the foundational UI stack for OpenTaco, integrating authentication, org/repo management, drift settings, and future Slack + billing support: effectively transforming the digger project into a full-featured, self-hostable Terraform automation dashboard:
    
    - Authentication via WorkOS (AuthKit), with SSO support planned for Auth0 and Okta.
    - Core dashboard views for repositories, projects, and drift-detection settings.
    - Backend orchestration through environment variables (ORCHESTRATOR_BACKEND_URL, ORCHESTRATOR_BACKEND_SECRET).
    - UI stack based on TanStack Router, Tailwind, ShadCN components, Lucide icons, and Vite
- Drift Scoping [#2320](https://github.com/diggerhq/digger/pull/2320) by @zij
    - This PR introduces a new “How-To” guide (`ce/howto/scope-drift-detection.md`) and a short note in the main drift-detection guide describing how users can scope drift checks—for example, to only run for `dev`, `prod`, or `demo` projects—without affecting their main `digger.yml`. There isn’t yet a built-in per-project drift filter, so this doc provides a **workaround**.
    - This is a part of a larger effort to make setting up Drift Detection as simple as possible, and moving what was previously under EE to CE to make it more accessible.
