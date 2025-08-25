---
title: Storage
description: S3 adapter behavior, in-memory fallback, and state layout.
---

# Storage

Default: S3 (bucket‑only)
- Uses AWS default credential chain.
- Configure via env vars or flags:
  - `OPENTACO_S3_BUCKET`, `OPENTACO_S3_REGION`, `OPENTACO_S3_PREFIX`

Fallback: In‑memory
- If S3 is not configured or init fails, the service warns and falls back to memory.
- Suitable for local demos; state is not persisted across restarts.

Object layout per state:
- `<prefix>/<state-id>/terraform.tfstate`
- `<prefix>/<state-id>/terraform.tfstate.lock`

System state convention:
- Default system state: `__opentaco_system_state` (created by CLI).
- Reserved IDs start with `__opentaco_`.
