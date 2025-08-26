---
title: API Reference
description: Management API endpoints and shapes.
---

# Management API (prefix /v1)

Auth
- All endpoints under `/v1` require `Authorization: Bearer <access>` unless the service is started with `-auth-disable`.
- Acquire tokens via the `taco login` CLI and see identity via `GET /v1/auth/me`.

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

Auth endpoints
- `GET  /v1/auth/config` — Server OIDC config (issuer, client_id, optional endpoints, redirect URIs).
- `POST /v1/auth/exchange` — Exchange OIDC ID token for OpenTaco access/refresh tokens.
- `POST /v1/auth/token` — Refresh to new access (rotates refresh).
- `POST /v1/auth/issue-s3-creds` — Issue stateless STS credentials (requires Bearer).
- `GET  /v1/auth/me` — Echo subject/roles/groups from Bearer.
- `GET  /oidc/jwks.json` — JWKS with current signing key.
