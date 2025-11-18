# Sandbox Sidecar

This package hosts a lightweight Node.js/TypeScript service that exposes the
`/api/v1/sandboxes/runs` API consumed by OpenTaco. It is responsible for:

1. Accepting Terraform run payloads from the Go backend (archives, state, metadata).
2. Spinning up an execution environment (E2B or a local fallback) to run
   `terraform init/plan/apply`.
3. Streaming logs, plan metadata, and updated state back to the main service.

## Getting Started

```bash
cd sandbox-sidecar
npm install
npm run dev        # hot-reloads with tsx
# or build + run
npm run build
npm start
```

The service listens on `PORT` (default `9100`).

## Configuration

| Variable | Description |
| --- | --- |
| `PORT` | HTTP port for the sidecar (default `9100`). |
| `SANDBOX_RUNNER` | `local` or `e2b`. Defaults to `local`. |
| `E2B_API_KEY` | Required for `SANDBOX_RUNNER=e2b`. |
| `E2B_DEFAULT_TEMPLATE_ID` | E2B template ID (use base template like `rki5dems9wqfm4r03t7g`). Required for E2B. |
| `E2B_BAREBONES_TEMPLATE_ID` | Same as DEFAULT for now - both use runtime installation. Required for E2B. |
| `LOCAL_TERRAFORM_BIN` | Optional path to the `terraform` binary (defaults to `terraform` in `$PATH`). |

### Terraform Version Selection

The sidecar installs Terraform at runtime for any requested version:

- **Any version** (including 1.5.5 default): Installs Terraform on-demand (~1-2 seconds)
- Supports any Terraform version available from HashiCorp releases
- No pre-built templates needed - simple and reliable

Users can specify the Terraform version when creating a unit in the UI, or it defaults to 1.5.5.

### Local Runner

The bundled local runner is intended for development. It unpacks the provided
archive, writes the optional state payload, and shells out to a Terraform binary
installed on the same host. All stdout/stderr is captured and streamed back to
the requester.

### E2B Runner

An opinionated `E2BSandboxRunner` is included as a scaffold. Hook it up to the
official SDK by wiring the `runPlan`/`runApply` helpers with the appropriate E2B API
calls and file upload primitives (see `src/runners/e2bRunner.ts` for the TODOs).
Once implemented, switch `SANDBOX_RUNNER=e2b` and provide `E2B_API_KEY` plus a
template/blueprint identifier.

## API Surface

### `POST /api/v1/sandboxes/runs`

Accepts the payload emitted by the Go backend (`operation`, `run_id`, base64
archives, etc.) and responds with the created job ID:

```json
{ "id": "sbx_run_123" }
```

### `GET /api/v1/sandboxes/runs/:id`

Returns the tracked job status:

```json
{
  "id": "sbx_run_123",
  "operation": "plan",
  "status": "succeeded",
  "logs": "...",
  "result": {
    "has_changes": false,
    "plan_json": "<base64 json>",
    "resource_additions": 0,
    "resource_changes": 0,
    "resource_destructions": 0
  }
}
```

`status` transitions through `pending → running → (succeeded|failed)`. On
failure, `error` contains the reason string. A `failed` response never includes
`result`.

## Development Notes

- This package intentionally keeps job state in-memory. Use a persistent store
  (Redis, Postgres) before running multiple replicas.
- The local runner shell-outs to `terraform`. Sandbox machines therefore need
  Terraform installed and accessible in `$PATH`.
- The E2B runner is wired as an interchangeable strategy: extend it or add
  additional runners (Kubernetes, Nomad, etc.) as needed without touching
  the Go control plane.

