# OpenTaco - Layer-0 State Control

OpenTaco (Layer-0) is a CLI + lightweight service for state control—create/read/update/delete and lock Terraform/OpenTofu state files, and act as an HTTP state backend proxy, paving the way for dependency awareness and RBAC. Today, the service runs stateless with an S3 “bucket-only” adapter for state storage (with an in-memory fallback for local demos).

## Documentation

- Live docs: https://opentaco.mintlify.app/
- Source: `opentaco/docs/` (Mintlify). When changing APIs, CLI behavior, storage semantics, or examples, update the relevant docs pages (overview, getting-started, cli, service-backend, provider, storage, demo, troubleshooting, reference) in the same PR.

## What is OpenTaco?

OpenTaco is an open-source "Terraform Companion" that starts with state control: a CLI + lightweight service focused on managing state files and access to them, not CI jobs. The long game is a self-hostable alternative to Terraform Cloud / Enterprise: state + RBAC first, then remote execution, PR automation, drift, and policy as later layers. This repo already includes a working S3 adapter, a Terraform HTTP backend proxy (GET/POST/PUT/LOCK/UNLOCK), and usable CLI + provider.

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

### Use the CLI

```bash
# Build the CLI
make cli

# Create a state
./taco state create myapp/prod

# List states
./taco state ls

# Get state metadata (size, lock status, last updated)
./taco state info myapp/prod

# Delete a state
./taco state rm myapp/prod

# Download state data
./taco state pull myapp/prod output.tfstate

# Upload state data  
./taco state push myapp/prod input.tfstate

# Lock a state manually
./taco state lock myapp/prod

# Unlock a state
./taco state unlock myapp/prod

# Acquire (lock + download in one operation)
./taco state acquire myapp/prod output.tfstate

# Release (upload + unlock in one operation)
./taco state release myapp/prod input.tfstate

# Auth commands (stubs)
./taco login           # OIDC PKCE (stub message)
./taco whoami          # Prints anonymous payload (stub)
./taco creds --json    # Prints placeholder AWS Process Credentials JSON (stub)
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
resource "opentaco_state" "example" {
  id = "myapp/prod"
  
  labels = {
    environment = "production"
    team        = "infrastructure"
  }
}

# Read state metadata
data "opentaco_state" "example" {
  id = opentaco_state.example.id
}

output "state_info" {
  value = {
    id      = data.opentaco_state.example.id
    size    = data.opentaco_state.example.size
    locked  = data.opentaco_state.example.locked
    updated = data.opentaco_state.example.updated
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

### Provider Bootstrap (taco provider init)

Quickly scaffold a Terraform workspace which:
- Stores its own TF state in the OpenTaco HTTP backend at `__opentaco_system_state`.
- Configures the OpenTaco provider against your server.
- Includes a demo `opentaco_state` resource (e.g., `myapp/prod`).

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
aws s3 ls s3://$OPENTACO_S3_BUCKET/$OPENTACO_S3_PREFIX/__opentaco_system_state/
aws s3 ls s3://$OPENTACO_S3_BUCKET/$OPENTACO_S3_PREFIX/myapp/prod/
```

Notes:
- System state defaults to `__opentaco_system_state`. Override with `--system-state <id>` if desired.
- The CLI creates the system state by convention (skip with `--no-create`).
 - You can scaffold into the current directory with: `./taco provider init . --server http://localhost:8080`.

### System State Convention

- Reserved names starting with `__opentaco_` are platform‑owned and should not be used for user stacks.
- Default system state ID is `__opentaco_system_state`, stored alongside user states under the same S3 prefix.
- The backend treats this like any other state; the CLI drives creation by convention.

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
   - Management API (`/v1`) for CRUD operations on states
   - Terraform HTTP backend proxy (`/v1/backend/{id}`) for Terraform/OpenTofu

2. **CLI** (`cmd/taco/`) - Command-line interface that calls the service for all operations

3. **SDK** (`pkg/sdk/`) - Typed HTTP client used by both CLI and Terraform provider

4. **Terraform Provider** (`providers/terraform/opentaco/`) - Manage states as Terraform resources

### Storage

- **S3 Store (default)**: Uses your AWS account “bucket-only” layout. Configure via flags or env (standard AWS SDK chain is used for auth).
- **Memory Store (fallback)**: Automatically used if S3 configuration is missing or fails at startup; resets on restart.

S3 object layout per state:
- `<prefix>/<state-id>/terraform.tfstate`
- `<prefix>/<state-id>/terraform.tfstate.lock` (present only while locked)

## API Endpoints

### Management API

**Note**: State IDs containing slashes (e.g., `myapp/prod`) are URL-encoded by replacing `/` with `__` in the path.

- `POST /v1/states` - Create a new state
  - Body: `{"id": "myapp/prod"}`
  - Response: `{"id": "myapp/prod", "created": "2025-01-01T00:00:00Z"}`

- `GET /v1/states?prefix=` - List states with optional prefix filter
  - Response: `{"states": [...], "count": 10}`

- `GET /v1/states/{encoded_id}` - Get state metadata
  - Example: `/v1/states/myapp__prod`
  - Response: `{"id": "myapp/prod", "size": 1024, "updated": "...", "locked": false}`

- `DELETE /v1/states/{encoded_id}` - Delete a state

- `GET /v1/states/{encoded_id}/download` - Download state file
  - Returns: Raw state file content

- `POST /v1/states/{encoded_id}/upload` - Upload state file
  - Body: Raw state file content
  - Query param: `?if_locked_by={lock_id}` (optional)

- `POST /v1/states/{encoded_id}/lock` - Lock a state
  - Body: `{"id": "lock-uuid", "who": "user@host", "version": "1.0.0"}` (optional)
  - Response: Lock info or 409 Conflict with current lock info

- `DELETE /v1/states/{encoded_id}/unlock` - Unlock a state
  - Body: `{"id": "lock-uuid"}`

### Terraform Backend API

- `GET /v1/backend/{id}` - Get state for Terraform
- `POST /v1/backend/{id}` - Update state from Terraform
- `PUT /v1/backend/{id}` - Update state from Terraform (alias of POST)
- `LOCK /v1/backend/{id}` - Acquire lock for Terraform
- `UNLOCK /v1/backend/{id}` - Release lock from Terraform

Note: Terraform lock coordination uses the `X-Terraform-Lock-ID` header; the service respects this header on update and unlock operations.

### Auth (stubs)

- `POST /v1/auth/exchange` – 501 Not Implemented (shape stub)
- `POST /v1/auth/token` – 501 Not Implemented
- `POST /v1/auth/issue-s3-creds` – 501 Not Implemented
- `GET /v1/auth/me` – 200 stub payload
- `GET /oidc/jwks.json` – 200 with empty `keys`

## CLI Commands Reference

### State Management

- `taco state create <id>` - Register a new state
- `taco state ls [prefix]` - List states, optionally filtered by prefix
- `taco state info <id>` - Show state metadata (aliases: `show`, `describe`)
- `taco state rm <id>` - Delete a state (aliases: `delete`, `remove`)

### Data Operations

- `taco state pull <id> [file]` - Download state data (stdout if no file specified)
- `taco state push <id> <file>` - Upload state data from file

### Lock Management

- `taco state lock <id>` - Manually lock a state
- `taco state unlock <id> [lock-id]` - Unlock a state (uses saved lock ID if not provided)

### Combined Operations

- `taco state acquire <id> [file]` - Lock + download in one operation
- `taco state release <id> <file>` - Upload + unlock in one operation

### Global Options

- `--server URL` - OpenTaco server URL (default: `http://localhost:8080`, env: `OPENTACO_SERVER`)
- `-v, --verbose` - Enable verbose output

### Provider Tools

- `taco provider init [dir]` - Scaffold a Terraform workspace for the OpenTaco provider
  - Flags:
    - `--dir <path>`: Output directory (default `opentaco-config`; positional `[dir]` takes precedence if given)
    - `--system-state <id>`: System state for the backend (default `__opentaco_system_state`)
    - `--force`: Overwrite files if they exist
    - `--no-create`: Do not create the system state (scaffold only)

### Environment Variables

- CLI: `OPENTACO_SERVER` sets the default server URL for `taco`.
- Terraform provider: `OPENTACO_ENDPOINT` sets the default provider endpoint.

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
│   ├── auth/           # Auth handlers & JWKS (stub)
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

- 409 on Create in provider ("State already exists"):
  - Cause: remote state with same `id` already exists; renaming the Terraform resource block does not change the backend ID.
  - Fix options:
    - Import: `terraform import opentaco_state.NAME <id>`.
    - Change ID: update `id = "..."` to a new value.
    - Remove remote: `./taco --server <url> state rm <id>` then apply.

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

### State ID Encoding

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
- S3 object layout per state:
  - `<prefix>/<state-id>/terraform.tfstate`
  - `<prefix>/<state-id>/terraform.tfstate.lock`
- System state convention:
  - Reserved IDs start with `__opentaco_`.
  - Default system state is `__opentaco_system_state` and is created by the CLI (not auto-created by the service).

## Future Roadmap

### Near Term
- **S3 Adapter**: Production storage backend maintaining compatibility with existing state layouts
- **State Versioning**: Keep history of state changes
- **Metrics**: Prometheus metrics for monitoring

### Medium Term
- **State Graph**: Track outputs and dependencies between states
- **RBAC**: Organizations, teams, users with SSO integration
- **Audit Logging**: Track all state operations

### Long Term
- **Remote Execution**: Run Terraform in controlled environments
- **PR Automation**: GitOps workflows with state management
- **Policy Engine**: OPA-based policy enforcement on state changes
- **UI**: Web interface for state management

## License

[License information to be added]
