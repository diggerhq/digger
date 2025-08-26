---
title: Service & Backend
description: HTTP service surfaces, Terraform backend proxy, health endpoints, and routing specifics.
---

# Service & Terraform Backend

The service exposes two surfaces:
- Management API under `/v1` for state CRUD and locking.
- Terraform HTTP backend proxy under `/v1/backend/{id}` supporting GET/POST/PUT/LOCK/UNLOCK.

Health endpoints:
- `GET /healthz` → 200 OK
- `GET /readyz` → 200 OK

Important routing notes:
- Explicitly wire custom methods: `LOCK` and `UNLOCK` must be added via `e.Add("LOCK", ...)` and `e.Add("UNLOCK", ...)`.
- Terraform sends the lock ID on writes via `?ID=<uuid>` (and headers). The backend should read both header `X-Terraform-Lock-ID` and query `ID`/`id`.

Backend state ID encoding:
- Human IDs: `myapp/prod`.
- Encoded single segment: `myapp__prod`.

S3 layout:
- `<prefix>/<state-id>/terraform.tfstate`
- `<prefix>/<state-id>/terraform.tfstate.lock`

System state:
- Default: `__opentaco_system_state`.
- Created by CLI; service does not auto‑create.

Auth
- Auth is enforced by default for `/v1` and `/v1/backend/*`. Clients must include `Authorization: Bearer <access>`.
- Use `taco login` to obtain tokens; the CLI will attach Bearer automatically to API calls.
- For Terraform provider/backend flows, use `-auth-disable` temporarily until the provider is wired to send Bearer headers.
