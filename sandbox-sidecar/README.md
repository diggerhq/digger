# Sandbox Sidecar

This package hosts a lightweight Node.js/TypeScript service that exposes the
`/api/v1/sandboxes/runs` API consumed by OpenTaco. It is responsible for:

1. Accepting Terraform run payloads from the Go backend (archives, state, metadata).
2. Spinning up an execution environment (E2B or a local fallback) to run
   `terraform init/plan/apply`.
3. Streaming logs, plan metadata, and updated state back to the main service.

## Getting Started

### Local Development

```bash
cd sandbox-sidecar
npm install
npm run dev        # hot-reloads with tsx
# or build + run
npm run build
npm start
```

The service listens on `PORT` (default `9100`).

### Docker

```bash
# Build the image
docker build -f Dockerfile_sidecar -t sandbox-sidecar:latest .

# Run the container
docker run -p 9100:9100 \
  -e SANDBOX_RUNNER=e2b \
  -e E2B_API_KEY=your-api-key \
  -e E2B_BAREBONES_TEMPLATE_ID=your-template-id \
  sandbox-sidecar:latest
```

### Using Pre-built Images

```bash
# Pull from GitHub Container Registry
docker pull ghcr.io/diggerhq/sandbox-sidecar:latest

# Run with environment file
docker run -p 9100:9100 \
  --env-file .env \
  ghcr.io/diggerhq/sandbox-sidecar:latest

# Or with Kubernetes/Helm (see helm-charts repo)
helm install taco-sidecar diggerhq/taco-sidecar
```

## Configuration

| Variable | Description |
| --- | --- |
| `PORT` | HTTP port for the sidecar (default `9100`). |
| `SANDBOX_RUNNER` | Must be `e2b`. |
| `E2B_API_KEY` | Required. Your E2B API key. |
| `E2B_BAREBONES_TEMPLATE_ID` | Required. Fallback template ID for runtime installation. |

### Terraform/OpenTofu Version Selection

The sidecar automatically selects the best execution environment:

1. **Pre-built templates** (instant startup): If a template exists for the requested version in `src/templateRegistry.ts`, it's used automatically
2. **Runtime installation** (~1-2 seconds): For versions not in the registry, Terraform/OpenTofu is installed on-demand

**Pre-built versions** (see `templates/manifest.ts`):
- Terraform: 1.0.11, 1.3.9, 1.5.5, 1.8.5
- OpenTofu: 1.6.0, 1.10.0

**Building templates**: Run `cd templates && npm run build` to build all templates defined in `manifest.ts`.

Users specify the version when creating a unit in the UI (defaults to 1.5.5).

### E2B Runner

The sidecar uses E2B sandboxes for secure, isolated Terraform/OpenTofu execution.
Each run creates an ephemeral sandbox, executes the IaC commands, and returns
results. Sandboxes are automatically cleaned up after execution.

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
- E2B sandboxes are ephemeral and isolated - each run gets a fresh environment.
- Pre-built templates provide instant startup; custom versions install at runtime (~1-2s).

