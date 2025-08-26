---
title: CLI
description: Taco CLI commands and usage.
---

# Taco CLI

Build:
```bash
make build-cli
```

Global flag:
- `--server <url>` sets the service endpoint (default `http://localhost:8080`).

Core commands:
- `taco state create <id>`: Create a state registration.
- `taco state ls [--prefix <pfx>]`: List states.
- `taco state rm <id>`: Delete a state.
- `taco state pull <id> [-f <file>]`: Download state to a file.
- `taco state push <id> [-f <file>] [--if-locked-by <uuid>]`: Upload state.
- `taco state lock <id> [--who --info]`: Acquire a lock.
- `taco state unlock <id> --id <uuid>`: Release a lock.
- `taco state acquire <id> [-f <file>]`: Lock + download.
- `taco state release <id> [-f <file>]`: Upload + unlock.

Provider bootstrap:
- `taco provider init [dir] [--system-state <id>] [--no-create] [--force]`
  - Scaffolds a Terraform workspace in `dir` (default `opentaco-config`).
  - Configures the HTTP backend to `__opentaco_system_state` by default.
  - Creates that system state unless `--no-create` is set.

Auth:
- `taco login [--force-login]` — Runs OIDC PKCE, saves tokens to `~/.config/opentaco/credentials.json` under the current `--server`.
  - No flags needed if the server exposes `/v1/auth/config` (OpenTaco does); the CLI will auto-discover issuer/client_id.
- `taco whoami` — Prints current identity.
- `taco creds --json` — Prints AWS Process Credentials JSON via `/v1/auth/issue-s3-creds`.
- `taco logout` — Removes saved tokens for the current `--server`.
