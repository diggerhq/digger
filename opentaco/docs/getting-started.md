---
title: Getting Started
description: Build, run, and use OpenTaco with Terraform quickly.
---

# Getting Started

Prerequisites:
- Go 1.25+
- Terraform 1.6+ (or OpenTofu)
- AWS creds set up if you want S3 persistence

Build all components from `opentaco/`:
```bash
make clean && make build
```

Run the service (S3 recommended):
```bash
OPENTACO_S3_BUCKET=<bucket> \
OPENTACO_S3_REGION=<region> \
OPENTACO_S3_PREFIX=<prefix> \
./opentacosvc
```

Health checks:
```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Scaffold a provider workspace and create the system state by convention:
```bash
./taco provider init opentaco-config --server http://localhost:8080
cd opentaco-config
terraform init
terraform apply -auto-approve
```

Use the created state in your own Terraform project (example backend):
```hcl
terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/myapp/prod"
    lock_address   = "http://localhost:8080/v1/backend/myapp/prod"
    unlock_address = "http://localhost:8080/v1/backend/myapp/prod"
  }
}
```

Troubleshooting quick tips:
- 405 on LOCK/UNLOCK → ensure service wires explicit routes for custom verbs.
- 409 on save → service must read lock ID from header or query `?ID=`.
- 409 on Create → state exists already; import, change `id`, or delete then apply.
