# OpenTaco - Layer-0 State Control

OpenTaco (Layer-0) is a CLI + lightweight service for state control—create/read/update/delete and lock Terraform/OpenTofu state files, and act as an HTTP state backend proxy, paving the way for dependency awareness and RBAC. Today, the service runs stateless with an S3 “bucket-only” adapter for state storage (with an in-memory fallback for local demos).

## Documentation

- Live docs: https://opentaco.mintlify.app/
 - Source: `opentaco/docs/` (Mintlify). When changing APIs, CLI behavior, storage semantics, or examples, update the relevant docs pages (overview, getting-started, cli, backend-service, provider, storage, demo, troubleshooting, reference) in the same PR.
  - See `docs/backend-service.md` for HTTP backend and the S3‑compatible shim.
  - Roadmap → `docs/roadmap.mdx` (milestones and future buckets).

## What is OpenTaco?

OpenTaco is an open-source "Terraform Companion" that starts with state control: a CLI + lightweight service focused on managing state files and access to them, not CI jobs. The long game is a self-hostable alternative to Terraform Cloud / Enterprise: state + RBAC first, then remote execution, PR automation, drift, and policy as later layers. This repo already includes a working S3 adapter, a Terraform HTTP backend proxy (GET/POST/PUT/LOCK/UNLOCK), and usable CLI + provider.

State management today; Remote Runs → VCS Integration + UI → Drift → Policies coming next. See `docs/scope-today.md` and `docs/roadmap.md`.

## Philosophy

- **Layer-0 = State control** (CRUD + lock, plus a backend proxy). No runs, no PR automation, no UI in this layer.
- **CLI-first** to settle semantics; UI comes later.
- **Self-hosted, bucket-only later** (S3 as the only stateful store when we add real storage; the service remains stateless).
- **Backwards compatibility** with existing S3 layouts (incl. Terragrunt) when we wire storage later; adoption should be drop-in.
- **Import from TFC should be easy** (later); keep shapes friendly to that path.

## Quick Start

### Prerequisites

- Go 1.25 or later
- Make

### Build Everything

```bash
make all
```

### Run the Service

```bash
make svc
# Service starts on http://localhost:8080
# Health: curl http://localhost:8080/healthz
# Ready:  curl http://localhost:8080/readyz
```

Auth is enabled by default. To temporarily bypass it (e.g., provider dev):

```bash
./opentacosvc -auth-disable -storage memory
```

### Use the CLI

```bash
# Build the CLI
make cli

# Create a state
./taco unit create myapp/prod

# List states
./taco unit ls

# Get state metadata (size, lock status, last updated)
./taco unit info myapp/prod

# Delete a state
./taco unit rm myapp/prod

# Download state data
./taco unit pull myapp/prod output.tfstate

# Upload state data  
./taco unit push myapp/prod input.tfstate

# Lock a state manually
./taco unit lock myapp/prod

# Unlock a state
./taco unit unlock myapp/prod

# Acquire (lock + download in one operation)
./taco unit acquire myapp/prod output.tfstate

# Release (upload + unlock in one operation)
./taco unit release myapp/prod input.tfstate

# Auth commands
./taco login --issuer <OIDC_ISSUER> --client-id <CLIENT_ID>  # Runs PKCE flow and saves tokens
# or simply:
# ./taco login --server http://localhost:8080                  # CLI fetches issuer/client_id from /v1/auth/config
./taco whoami                                              # Prints current identity (if logged in)
./taco creds --json                                        # Prints AWS Process Credentials JSON via service
./taco logout                                              # Removes saved tokens for --server
```

### Authentication (OIDC quick start)

Configure OIDC so `taco login` works and protected endpoints require login.

1) WorkOS setup
- Create a User Management project and a Native (PKCE) OAuth application.
- Add redirect URI: `http://127.0.0.1:8585/callback`.
- Note values:
  - Client ID: `<WORKOS_CLIENT_ID>`
  - Issuer: `https://api.workos.com/user_management`
  - Authorization endpoint: `https://api.workos.com/user_management/authorize`
  - Token endpoint: `https://api.workos.com/user_management/token`

2) Service config (verifies ID tokens and issues OpenTaco tokens)

```bash
export OPENTACO_AUTH_ISSUER="https://api.workos.com/user_management"
export OPENTACO_AUTH_CLIENT_ID="<WORKOS_CLIENT_ID>"
./opentacosvc -storage memory
```

3) Login via CLI (PKCE)

```bash
./taco login \
  --server http://localhost:8080 \
  --issuer https://api.workos.com/user_management \
  --client-id <WORKOS_CLIENT_ID> \
  --auth-url https://api.workos.com/user_management/authorize \
  --token-url https://api.workos.com/user_management/token
```

This opens a browser (also prints the URL). After you authenticate, the CLI exchanges the OIDC ID token with the service and saves OpenTaco tokens to `~/.config/opentaco/credentials.json`. To force the login box even if an SSO session exists, add `--force-login`.

Auth0 variant:

```bash
export OPENTACO_AUTH_ISSUER="https://<TENANT>.auth0.com"     # or <region>.auth0.com
export OPENTACO_AUTH_CLIENT_ID="<AUTH0_NATIVE_APP_CLIENT_ID>"
./opentacosvc -storage memory

# No flags needed; CLI uses discovery via /v1/auth/config
./taco login --server http://localhost:8080
```

4) Verify auth

```bash
# Without login (or in a fresh shell without saved tokens): should return 401
curl -i http://localhost:8080/v1/units

# Using CLI (adds bearer automatically)
./taco unit ls
```

### Terraform Provider

```bash
# Build the provider
make build-prov

# Example usage
cd providers/terraform/opentaco/examples/basic

# Configure the provider
cat > main.tf << 'EOF'
terraform {
  required_providers {
    opentaco = {
      source = "digger/opentaco"
    }
  }
}

provider "opentaco" {
  endpoint = "http://localhost:8080"  # Or use OPENTACO_ENDPOINT env var
}

# Create a state registration
resource "opentaco_unit" "example" {
  id = "myapp/prod"
  
  labels = {
    environment = "production"
    team        = "infrastructure"
  }
}

# Read unit metadata
data "opentaco_unit" "example" {
  id = opentaco_unit.example.id
}

output "unit_info" {
  value = {
    id      = data.opentaco_unit.example.id
    size    = data.opentaco_unit.example.size
    locked  = data.opentaco_unit.example.locked
    updated = data.opentaco_unit.example.updated
  }
}
EOF

# Run Terraform
terraform init
terraform apply

#### Local Provider Install (for development)

When using the local, unpublished provider:

Option A — dev overrides in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides { "digger/opentaco" = "/absolute/path/to/opentaco/providers/terraform/opentaco" }
  direct {}
}
```

Option B — install into the plugin directory:

```
~/.terraform.d/plugins/digger/opentaco/0.0.0/<os>_<arch>/terraform-provider-opentaco
```

Then run `terraform init` again to pick up the local build.
```

### Dependencies & Unit Status

OpenTaco supports output-level dependencies between units without any external database. All metadata is stored as digests in a dedicated graph workspace unit.

- Graph workspace ID: `__opentaco_system` (a normal tfstate managed via the OpenTaco backend)
- Provider resource: `opentaco_dependency` (one resource per edge)
- Service updates: Any unit write updates relevant edges in the graph tfstate (source refresh and target acknowledge)
- Status API: `GET /v1/units/{id}/status`
- CLI:
  - `taco unit status [<id> | --prefix <pfx>]` shows a table across units.
  - Status is printed as friendly, color-coded labels:
    - up to date (green) — no incoming pending
    - needs re-apply (red) — at least one incoming pending
    - might need re-apply (yellow) — clean incoming, but an upstream is red

See a full runnable example under `examples/dependencies/`.

### Provider Bootstrap (taco provider init)

Quickly scaffold a Terraform workspace which:
- Stores its own TF state in the OpenTaco HTTP backend at `__opentaco_system`.
- Configures the OpenTaco provider against your server.
- Includes a demo `opentaco_unit` resource (e.g., `myapp/prod`).

Steps:

```bash
# 1) Start the service on S3
OPENTACO_S3_BUCKET=<bucket> \
OPENTACO_S3_REGION=<region> \
OPENTACO_S3_PREFIX=<prefix> \
./opentacosvc

# 2) Scaffold the provider workspace
./taco provider init opentaco-config --server http://localhost:8080

# 3) Initialize and apply
cd opentaco-config
terraform init
terraform apply -auto-approve

# 4) Verify in S3
aws s3 ls s3://$OPENTACO_S3_BUCKET/$OPENTACO_S3_PREFIX/__opentaco_system/
aws s3 ls s3://$OPENTACO_S3_BUCKET/$OPENTACO_S3_PREFIX/myapp/prod/
```

Notes:
- System unit defaults to `__opentaco_system`. Override with `--system-unit <id>` if desired.
- The CLI creates the system unit by convention (skip with `--no-create`).
 - You can scaffold into the current directory with: `./taco provider init . --server http://localhost:8080`.

### System Unit Convention

- Reserved names starting with `__opentaco_` are platform‑owned and should not be used for user stacks.
- Default system unit ID is `__opentaco_system`, stored alongside user units under the same S3 prefix.
- The backend treats this like any other unit; the CLI drives creation by convention.

### Using as Terraform Backend

```hcl
terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/myapp/prod"
    lock_address   = "http://localhost:8080/v1/backend/myapp/prod"
    unlock_address = "http://localhost:8080/v1/backend/myapp/prod"
  }
}
```

## Architecture

### Components

1. **Service** (`cmd/opentacosvc/`) - HTTP server with two surfaces:
   - Management API (`/v1`) for CRUD operations on units
   - Terraform HTTP backend proxy (`/v1/backend/{id}`) for Terraform/OpenTofu

2. **CLI** (`cmd/taco/`) - Command-line interface that calls the service for all operations

3. **SDK** (`pkg/sdk/`) - Typed HTTP client used by both CLI and Terraform provider

4. **Terraform Provider** (`providers/terraform/opentaco/`) - Manage units as Terraform resources

### Storage

- **S3 Store (default)**: Uses your AWS account “bucket-only” layout. Configure via flags or env (standard AWS SDK chain is used for auth).
- **Memory Store (fallback)**: Automatically used if S3 configuration is missing or fails at startup; resets on restart.

S3 object layout per unit:
- `<prefix>/<unit-id>/terraform.tfstate`
- `<prefix>/<unit-id>/terraform.tfstate.lock` (present only while locked)

## API Endpoints

### Management API

Auth: All management endpoints require `Authorization: Bearer <access>`, unless the service is started with `-auth-disable`.

Note: Unit IDs containing slashes (e.g., `myapp/prod`) are URL-encoded by replacing `/` with `__` in the path.

- `POST /v1/units` - Create a new unit
  - Body: `{"id": "myapp/prod"}`
  - Response: `{"id": "myapp/prod", "created": "2025-01-01T00:00:00Z"}`

- `GET /v1/units?prefix=` - List units with optional prefix filter
  - Response: `{"units": [...], "count": 10}`

- `GET /v1/units/{encoded_id}` - Get unit metadata
  - Example: `/v1/units/myapp__prod`
  - Response: `{"id": "myapp/prod", "size": 1024, "updated": "...", "locked": false}`

- `DELETE /v1/units/{encoded_id}` - Delete a unit

- `GET /v1/units/{encoded_id}/download` - Download unit tfstate
  - Returns: Raw tfstate content

- `POST /v1/units/{encoded_id}/upload` - Upload unit tfstate
  - Body: Raw tfstate content
  - Query param: `?if_locked_by={lock_id}` (optional)

- `POST /v1/units/{encoded_id}/lock` - Lock a unit
  - Body: `{"id": "lock-uuid", "who": "user@host", "version": "1.0.0"}` (optional)
  - Response: Lock info or 409 Conflict with current lock info

- `DELETE /v1/units/{encoded_id}/unlock` - Unlock a unit
  - Body: `{"id": "lock-uuid"}`

### Terraform Backend API

- `GET /v1/backend/{id}` - Get state for Terraform
- `POST /v1/backend/{id}` - Update state from Terraform
- `PUT /v1/backend/{id}` - Update state from Terraform (alias of POST)
- `LOCK /v1/backend/{id}` - Acquire lock for Terraform
- `UNLOCK /v1/backend/{id}` - Release lock from Terraform

Note: Terraform lock coordination uses the `X-Terraform-Lock-ID` header; the service respects this header on update and unlock operations.

### Auth

- `GET  /v1/auth/config` – Server OIDC config (issuer, client_id, optional endpoints, redirect URIs)
- `POST /v1/auth/exchange` – Exchange OIDC ID token for OpenTaco access/refresh
- `POST /v1/auth/token` – Refresh to new access (rotates refresh)
- `POST /v1/auth/issue-s3-creds` – Issue stateless STS creds; requires `Authorization: Bearer <access>`
- `GET  /v1/auth/me` – Echo subject/roles/groups from Bearer if present

## CLI Commands Reference

### Unit Management

- `taco unit create <id>` - Register a new unit
- `taco unit ls [prefix]` - List units, optionally filtered by prefix
- `taco unit info <id>` - Show unit metadata (aliases: `show`, `describe`)
- `taco unit rm <id>` - Delete a unit (aliases: `delete`, `remove`)

### Data Operations

- `taco unit pull <id> [file]` - Download unit tfstate (stdout if no file specified)
- `taco unit push <id> <file>` - Upload unit tfstate from file

### Lock Management

- `taco unit lock <id>` - Manually lock a unit
- `taco unit unlock <id> [lock-id]` - Unlock a unit (uses saved lock ID if not provided)

### Combined Operations

- `taco unit acquire <id> [file]` - Lock + download in one operation
- `taco unit release <id> <file>` - Upload + unlock in one operation

### Global Options

- `--server URL` - OpenTaco server URL (default: `http://localhost:8080`, env: `OPENTACO_SERVER`)
- `-v, --verbose` - Enable verbose output

### Provider Tools

- `taco provider init [dir]` - Scaffold a Terraform workspace for the OpenTaco provider
  - Flags:
    - `--dir <path>`: Output directory (default `opentaco-config`; positional `[dir]` takes precedence if given)
    - `--system-unit <id>`: System unit for the backend (default `__opentaco_system`)
    - `--force`: Overwrite files if they exist
    - `--no-create`: Do not create the system unit (scaffold only)

### Environment Variables

- CLI: `OPENTACO_SERVER` sets the default server URL for `taco`.
- Terraform provider: `OPENTACO_ENDPOINT` sets the default provider endpoint.

### Auth

- `taco login [--force-login]` – PKCE login; saves tokens to `~/.config/opentaco/credentials.json`
- `taco whoami` – Prints current identity
- `taco creds --json` – Prints AWS Process Credentials JSON via `/v1/auth/issue-s3-creds`
- `taco logout` – Removes saved tokens for `--server`

## Development

### Project Structure

```
opentaco/
├── cmd/
│   ├── opentacosvc/    # Service binary
│   └── taco/           # CLI binary
│       └── commands/   # Cobra commands package
├── internal/
│   ├── api/            # HTTP handlers
│   ├── backend/        # Terraform backend proxy
│   ├── domain/         # Business logic
│   ├── auth/           # JWT auth handlers
│   ├── oidc/           # OIDC verifier abstraction (stub)
│   ├── sts/            # STS issuer interface (stub)
│   ├── rbac/           # RBAC checker (permissive stub)
│   ├── middleware/     # AuthN/AuthZ middlewares, 501 helper
│   ├── storage/        # Storage interfaces
│   └── observability/  # Health/metrics
├── pkg/
│   └── sdk/            # Go client library
└── providers/
    └── terraform/      # Terraform provider
        └── opentaco/
```

### Building from Source

```bash
# Initialize modules (first time only; skip if go.mod files exist)
make init

# Build everything
make all

# Build individual components
make build-svc   # Service only
make build-cli   # CLI only
make build-prov  # Provider only
```

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

### Clean Build Artifacts

```bash
make clean
```

### Configuration (auth stubs)

- Example auth config shape is provided in `configs/auth.yaml` (not yet enforced).
- Docs placeholders for upcoming auth/STS work:
  - `docs/backend_profile_guide.md`
  - `docs/auth_config_examples.md`
  - `docs/final_spec_state_auth_sts.md`

### Storage Options

```bash
# Run with S3 storage (default)
# Uses standard AWS credential/config chain (env, shared config, IAM role)
OPENTACO_S3_BUCKET=my-bucket \
OPENTACO_S3_PREFIX=opentaco \
OPENTACO_S3_REGION=us-east-1 \
./opentacosvc

# Explicit flags (optional)
./opentacosvc -storage s3 \
  -s3-bucket my-bucket \
  -s3-prefix opentaco \
  -s3-region us-east-1

# Force in-memory storage
./opentacosvc -storage memory
```

## Troubleshooting

### Backend/Provider Issues

- 405 on LOCK/UNLOCK during `terraform init/apply`:
  - Cause: routes for custom HTTP verbs not wired.
  - Fix (service): add `e.Add("LOCK", "/v1/backend/*", handler)` and `e.Add("UNLOCK", "/v1/backend/*", handler)`, rebuild, restart.

- 409 on POST/PUT ("Failed to save state"):
  - Cause: backend not reading lock ID from Terraform query `?ID=<uuid>`.
  - Fix (service): in update handler, read lock ID from header `X-Terraform-Lock-ID` OR query `ID`/`id`.

- 409 on Create in provider ("Unit already exists"):
  - Cause: remote state with same `id` already exists; renaming the Terraform resource block does not change the backend ID.
  - Fix options:
    - Import: `terraform import opentaco_unit.NAME <id>`.
    - Change ID: update `id = "..."` to a new value.
    - Remove remote: `./taco --server <url> unit rm <id>` then apply.

### Local provider override (no plugin copying)

If Terraform cannot find the local provider, add a workspace-local CLI config and re-init:

```bash
# From repo root (path to provider source dir)
ABS="$(pwd)/providers/terraform/opentaco"

# Write a local override inside your scaffolded dir
cat > opentaco-config/.terraformrc <<EOF
provider_installation {
  dev_overrides { "digger/opentaco" = "${ABS}" }
  direct {}
}
EOF

export TF_CLI_CONFIG_FILE="$PWD/opentaco-config/.terraformrc"
cd opentaco-config && terraform init -upgrade
```

## Implementation Notes

### Unit ID Encoding

- User-facing IDs use natural paths: `myapp/prod`
- HTTP routes encode slashes as double underscores: `myapp__prod`
- This is handled automatically by the CLI and SDK

### Lock Behavior

- Locks are cooperative - clients must respect them
- Lock IDs are UUIDs generated by clients
- The CLI saves lock IDs locally in `.taco/` for convenience
- Terraform backend operations handle locking automatically

### Storage Behavior

- Default storage: S3 (bucket-only). Uses AWS default credential chain.
- Fallback: if S3 is not configured or init fails, the service warns and falls back to in-memory storage.
- S3 object layout per unit:
  - `<prefix>/<unit-id>/terraform.tfstate`
  - `<prefix>/<unit-id>/terraform.tfstate.lock`
- System unit convention:
  - Reserved IDs start with `__opentaco_`.
  - Default system unit is `__opentaco_system` and is created by the CLI (not auto-created by the service).

## Future Roadmap

### Near Term
- **S3 Adapter**: Production storage backend maintaining compatibility with existing tfstate layouts
- **Unit Versioning**: Keep history of tfstate changes
- **Metrics**: Prometheus metrics for monitoring

### Medium Term
- **Dependency Graph**: Track outputs and dependencies between units
- **RBAC**: Organizations, teams, users with SSO integration
- **Audit Logging**: Track all unit operations

### Long Term
- **Remote Execution**: Run Terraform in controlled environments
- **PR Automation**: GitOps workflows with unit/tfstate management
- **Policy Engine**: OPA-based policy enforcement on unit changes
- **UI**: Web interface for state management

## License

[License information to be added]
### S3‑compatible Backend (Terraform backend "s3")

You can point Terraform’s S3 backend at OpenTaco’s `/s3` endpoint using process credentials minted by the CLI.

1) AWS profile (~/.aws/config):

```
[profile opentaco-state-backend]
region = auto
credential_process = "/absolute/path/to/taco" creds --json --server http://localhost:8080
```

2) Terraform backend block:

```hcl
terraform {
  backend "s3" {
    bucket  = "opentaco"
    key     = "myapp/prod/terraform.tfstate"
    endpoints = { s3 = "http://localhost:8080/s3" }
    use_path_style                 = true
    skip_credentials_validation    = true
    skip_region_validation         = true
    skip_requesting_account_id     = true
    use_lockfile                   = true  # Terraform 1.13+
    profile                        = "opentaco-state-backend"
  }
}
```

3) Run:

```bash
./taco login --server http://localhost:8080
export AWS_SDK_LOAD_CONFIG=1
export AWS_PROFILE=opentaco-state-backend
terraform init -reconfigure && terraform apply -auto-approve
```

More details in `docs/s3-compat.md`.
