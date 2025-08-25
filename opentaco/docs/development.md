---
title: Development
description: Build, test, lint, and repo structure.
---

# Development

Build
```bash
make build           # svc + cli + provider
make build-svc       # ./opentacosvc
make build-cli       # ./taco
make build-prov      # ./terraform-provider-opentaco
```

Run
```bash
make svc             # runs service on :8080
```

Lint & tests
```bash
make lint
make test
```

Directory structure (key parts)
```
cmd/opentacosvc/   # service entrypoint
cmd/taco/          # CLI entrypoint
internal/api/      # management API
internal/backend/  # Terraform backend proxy
internal/storage/  # S3 adapter + memory fallback
pkg/sdk/           # Go SDK used by CLI & provider
providers/terraform/opentaco/   # Terraform provider
examples/demo-provider/         # End-to-end demo
```

Stubs convention (for shape-only work)
- When scaffolding endpoints without full implementation, return 501 with a uniform JSON body to keep clients predictable.
