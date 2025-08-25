# OpenTaco Agents Playbook (Milestone 1: Dummies)

- Table of Contents
  - Purpose (what OpenTaco is in one paragraph)
  - Milestone 1 Scope (what “dummies” means)
  - Project Placement & Constraints
  - Directory Structure (what & why for each path)
  - API Surfaces to Freeze (shapes only; all return 501)
  - CLI Commands (wire to endpoints; surface 501)
  - Terraform Provider (skeleton only)
  - Tooling & Versions (pin modern, battle-tested)
  - Files to Create (what & why; minimal examples where helpful)
  - Acceptance Criteria (Definition of Done)
  - Style & Guardrails
  - Next Milestones (context only, no action now)

## Purpose (what OpenTaco is in one paragraph)
OpenTaco is a self-hostable, open-source Terraform companion that starts with Layer‑0: state control (CRUD + lock + HTTP backend proxy) and grows into RBAC, policy, runs, and more. Milestone 1 is shape‑setting only: agents scaffold the service, CLI, SDK, and Terraform provider so all surfaces exist and compile, but business endpoints intentionally return Not Implemented. Keep the focus on state + RBAC as the core (see agents_context/opentaco-state-rbac.md), and defer automation/runs to later layers.

## Milestone 1 Scope (what “dummies” means)
- All business endpoints return HTTP 501 Not Implemented with a uniform JSON body (below).
- CLI calls those endpoints, prints the 501 message, and exits non‑zero.
- Terraform provider compiles; operations return diagnostics “not implemented”.
- Only /healthz and /readyz return 200 OK.
- No storage, S3, DB, auth, or external side effects outside opentaco/.

Uniform 501 error body:
```json
{ "error": "not_implemented", "message": "Milestone 1 dummy endpoint", "hint": "This route will be implemented in a later milestone." }
```

## Project Placement & Constraints
- Everything lives under <repo-root>/opentaco/.
- Do not touch files outside opentaco/ (no root go.work/CI/global configs).
- Keep temp/build artifacts inside opentaco/ (use .gitignore if needed).
- Note: repo currently contains working prototypes beyond M1; do not regress them. When stubbing new surfaces, follow the 501 pattern.

## Directory Structure (what & why for each path)
```
opentaco/
├─ README.md                  # Purpose, scope (dummies), how to run
├─ Makefile                   # build/lint/test/svc/cli/prov targets
├─ .golangci.yml              # linters config
├─ cmd/
│  ├─ opentacosvc/            # service entrypoint (Echo HTTP server)
│  └─ taco/                   # CLI entrypoint (Cobra)
├─ internal/
│  ├─ api/                    # management API handlers (return 501)
│  ├─ backend/                # Terraform backend verbs (LOCK/UNLOCK/GET/POST/PUT → 501)
│  ├─ domain/                 # pure types & tiny helpers (minimal in M1)
│  ├─ storage/                # interfaces only (no impl in M1)
│  └─ observability/          # healthz/readyz, metrics stub, logging adapters
├─ pkg/
│  └─ sdk/                    # typed HTTP client used by CLI & provider (Go module)
└─ providers/
   └─ terraform/
      └─ opentaco/            # terraform provider (Go module)
         ├─ provider/
         ├─ resources/
         ├─ datasources/
         └─ examples/
```

## API Surfaces to Freeze (shapes only; all return 501)
- Management API (prefix /v1):
  - POST /v1/states
  - GET /v1/states?prefix=<string>
  - GET /v1/states/*id
  - DELETE /v1/states/*id
  - GET /v1/states/*id/download
  - POST /v1/states/*id/upload[?if_locked_by=<uuid>]
  - POST /v1/states/*id:lock
  - DELETE /v1/states/*id:unlock
- Terraform HTTP backend proxy (prefix /v1/backend/*id): GET, POST, PUT, LOCK, UNLOCK.

Example Echo wiring for LOCK/UNLOCK and a 501 helper:
```go
// Router setup
e := echo.New()
e.GET("/healthz", healthz)
e.GET("/readyz", readyz)
e.Add("LOCK",   "/v1/backend/*", backendHandle)
e.Add("UNLOCK", "/v1/backend/*", backendHandle)

// 501 helper
func notImplemented(c echo.Context) error {
    return c.JSON(http.StatusNotImplemented, map[string]string{
        "error":   "not_implemented",
        "message": "Milestone 1 dummy endpoint",
        "hint":    "This route will be implemented in a later milestone.",
    })
}
```

## CLI Commands (wire to endpoints; surface 501)
```
taco state create <id> [-l key=val]
taco state ls [--prefix <pfx>]
taco state rm <id>
taco state pull <id> [-f terraform.tfstate]
taco state push <id> [-f terraform.tfstate] [--if-locked-by <uuid>]
taco state lock <id> [--who $USER --info "note"]
taco state unlock <id> --id <uuid>
taco state acquire <id> [-f terraform.tfstate]
taco state release <id> [-f terraform.tfstate]
```
Each command calls the corresponding endpoint, prints the 501 JSON, and exits with non‑zero status.

## Terraform Provider (skeleton only)
- Provider config: endpoint (string).
- Resource opentaco_state: schema { id (required), labels (optional map) }; CRUD → diag “not implemented”.
- Data source opentaco_state: input { id }; read → diag “not implemented”.
- Lives in opentaco/providers/terraform/opentaco as its own Go module; include examples/basic/main.tf (not executed in M1).

## Tooling & Versions (pin modern, battle-tested)
- Language: Go 1.25 (align with current repo). Use toolchain pinning if you introduce new modules.
- HTTP: github.com/labstack/echo/v4 with Recover, RequestID, Gzip, Secure, BodyLimit("10M"), sensible timeouts.
- CLI: github.com/spf13/cobra.
- SDK HTTP: standard net/http is fine (resty optional).
- Logging: prefer go.uber.org/zap when adding structured logs (optional in M1).
- Terraform: github.com/hashicorp/terraform-plugin-framework (+ terraform-plugin-log).
- Lint: golangci-lint (align with .golangci.yml: gofmt, govet, staticcheck, errcheck, ineffassign, prealloc, goimports, gosimple, unused).

## Files to Create (what & why; minimal examples where helpful)
- README.md: purpose, dummy scope, how to run service/CLI, where provider lives, constraints (no side effects outside opentaco/).
- Makefile: build, lint, test, svc, cli, prov targets.
- .golangci.yml: baseline linter configuration.
- cmd/opentacosvc/: main.go bootstraps Echo; /healthz and /readyz → 200; all other handlers use notImplemented.
- internal/api/: register Management API; handlers return 501 using the helper.
- internal/backend/: backend proxy GET/POST/PUT/LOCK/UNLOCK → 501.
- internal/domain/: tiny types (StateID, Lock, StateMeta, ErrorResponse) and the 501 error constant.
- internal/storage/: StateStore interface only (no impl in M1).
- internal/observability/: healthz/readyz, metrics stub (200 OK empty body), logging glue.
- pkg/sdk/: rest client; methods for each endpoint that forward requests and surface 501s as errors.
- cmd/taco/: Cobra root with --server; subcommands above; print 501 JSON on error.
- providers/terraform/opentaco/: main.go, provider/provider.go, resources/state_resource.go, datasources/state_data_source.go, examples/basic/main.tf — all methods return “not implemented”.

## Acceptance Criteria (Definition of Done)
- Service, CLI, and provider compile from within opentaco/.
- Service runs on :8080; /healthz and /readyz return 200 OK.
- All Management and Backend routes return 501 with the uniform JSON.
- CLI commands invoke endpoints and surface “Not Implemented” (non‑zero exit).
- Provider builds; CRUD/Read return diagnostics “not implemented”.
- golangci-lint passes on all code in opentaco/.
- Zero changes outside opentaco/.

## Style & Guardrails
- Keep handlers short; prefer pure helpers in internal/domain/.
- No auth, storage, external HTTP calls, or filesystem writes outside opentaco/.
- Include brief comments noting “Milestone 1 (dummies) — returns 501 intentionally”.
- When repo already contains working prototypes, treat this playbook as the shape contract and avoid regressing implemented behavior.

## Next Milestones (context only, no action now)
- Swap stubs for an S3 “bucket‑only” adapter while preserving shapes.
- Add RBAC/SSO and outputs hashing for dependency awareness.
- Keep Terraform HTTP backend proxy supported; consider S3‑compat endpoint later.
