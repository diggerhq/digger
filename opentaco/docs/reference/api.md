---
title: API Reference
description: Management API endpoints and shapes.
---

# Management API (prefix /v1)

States
- `POST /v1/states` — create
- `GET /v1/states?prefix=<string>` — list
- `GET /v1/states/{id}` — get metadata
- `DELETE /v1/states/{id}` — delete
- `GET /v1/states/{id}/download` — download tfstate
- `POST /v1/states/{id}/upload[?if_locked_by=<uuid>]` — upload tfstate
- `POST /v1/states/{id}/lock` — acquire lock
- `DELETE /v1/states/{id}/unlock` — release lock

Notes
- IDs use natural paths like `myapp/prod`; clients may encode as `myapp__prod` for single-segment routes.
- Upload respects `if_locked_by` to avoid overwriting when held by a different lock.

