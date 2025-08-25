---
title: Demo
description: End-to-end demo using the provider and backend.
---

# End-to-End Demo

1) Build binaries:
```bash
make clean && make build
```

2) Start the service (S3 recommended):
```bash
OPENTACO_S3_BUCKET=<bucket> OPENTACO_S3_REGION=<region> OPENTACO_S3_PREFIX=<prefix> ./opentacosvc
```

3) Scaffold provider workspace and apply:
```bash
./taco provider init opentaco-config --server http://localhost:8080
cd opentaco-config
terraform init
terraform apply -auto-approve
```

4) Point your own Terraform to the created state:
```hcl
terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/myapp/prod"
    lock_address   = "http://localhost:8080/v1/backend/myapp/prod"
    unlock_address = "http://localhost:8080/v1/backend/myapp/prod"
  }
}
```

Included example:
```bash
cd examples/demo-provider/my-app
terraform init
terraform apply -auto-approve
```

