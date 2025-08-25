---
title: Terraform Backend
description: HTTP backend proxy behavior and expectations.
---

# Terraform HTTP Backend Proxy (prefix /v1/backend/{id})

Supported methods
- `GET` — download state
- `POST` — upload/update state
- `PUT` — upload/update state (alias)
- `LOCK` — acquire lock
- `UNLOCK` — release lock

Lock semantics
- Lock ID is provided by client (Terraform) and sent in headers and `?ID=` query.
- Backend must verify lock ownership before writes and unlocks.

IDs & encoding
- Natural IDs: `myapp/prod`.
- Single-segment encoding: `myapp__prod`.

Routes wiring
- Custom verbs must be explicitly registered (Echo: `e.Add("LOCK"...)`, `e.Add("UNLOCK"...)`).
