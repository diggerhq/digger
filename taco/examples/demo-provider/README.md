# Demo: OpenTaco Provider + Terraform Backend

This walkthrough boots the OpenTaco service, scaffolds a provider workspace, creates a demo unit via Terraform, and then shows how to point your own Terraform projects at that unit using the OpenTaco HTTP backend.

Full documentation: https://opentaco.mintlify.app/

## Prerequisites

- Go 1.25
- Terraform 1.6+ (or OpenTofu) installed
- AWS credentials configured if using S3 (recommended for persistence)

## 1) Build binaries

Run from the repo root `opentaco/`:

```bash
make build-svc build-cli build-prov
```

## 2) Start the service (S3 recommended)

```bash
OPENTACO_S3_BUCKET=<bucket> \
OPENTACO_S3_REGION=<region> \
OPENTACO_S3_PREFIX=<prefix> \
./opentacosvc
```

Notes:
- If S3 isn’t configured, the service falls back to in‑memory storage. That’s fine for a quick local demo, but state is not persisted across restarts.
- Health: `curl http://localhost:8080/healthz`

## 3) Scaffold provider workspace

Use the CLI to generate a ready‑to‑run provider config in this example directory:

```bash
./taco provider init opentaco-config --server http://localhost:8080
```

What this creates:
- `examples/demo-provider/opentaco-config/main.tf` with:
  - Terraform HTTP backend pointing to `/v1/backend/__opentaco_system` (system unit)
  - `opentaco` provider configured to your server endpoint
  - Example resource: `opentaco_unit "example" { id = "myapp/prod" }`
- `examples/demo-provider/opentaco-config/.gitignore`

By default the CLI also creates the system unit `__opentaco_system`. Use `--no-create` to skip.

## 4) (If needed) Local provider override

If Terraform can’t find the local provider, add a workspace‑local override and re‑init:

```bash
# From repo root
ABS="$(pwd)/providers/terraform/opentaco"

cat > examples/demo-provider/opentaco-config/.terraformrc <<EOF
provider_installation {
  dev_overrides { "digger/opentaco" = "${ABS}" }
  direct {}
}
EOF

export TF_CLI_CONFIG_FILE="$PWD/examples/demo-provider/opentaco-config/.terraformrc"
```

## 5) Initialize and apply the provider workspace

```bash
cd opentaco-config
terraform apply -auto-approve
```

This registers the demo unit ID `myapp/prod` in OpenTaco.

Verify:
- `./taco unit ls`
- If using S3: check objects under `$OPENTACO_S3_PREFIX/myapp/prod/`

## 6) Use the new unit from your own Terraform

Create another directory for your project, point its backend to the unit you just created, and run `terraform init`.

Alternatively, use the included example at `examples/demo-provider/my-app/main.tf`:

```bash
cd examples/demo-provider/my-app
terraform init
terraform apply -auto-approve
```

Example `examples/demo-provider/my-app/main.tf`:

```hcl
terraform {
  backend "http" {
    # Either use raw path with slashes…
    address        = "http://localhost:8080/v1/backend/myapp/prod"
    lock_address   = "http://localhost:8080/v1/backend/myapp/prod"
    unlock_address = "http://localhost:8080/v1/backend/myapp/prod"

    # …or the double-underscore variant if you prefer a single path segment:
    # address        = "http://localhost:8080/v1/backend/myapp__prod"
    # lock_address   = "http://localhost:8080/v1/backend/myapp__prod"
    # unlock_address = "http://localhost:8080/v1/backend/myapp__prod"
  }
}

# Add your own resources here.
```

Then (if you wrote your own `main.tf`):

```bash
cd examples/demo-provider/my-app
terraform init
# terraform apply  # (optional – add real resources first)
```

Your project will now read/write its Terraform tfstate via the OpenTaco backend under the `myapp/prod` unit ID.

## Troubleshooting (quick)

- 405 LOCK/UNLOCK during init/apply → service routes not wired. Ensure the service adds explicit routes for custom verbs (`LOCK`, `UNLOCK`).
- 409 Failed to save unit on POST/PUT → service must accept lock ID from header or query (`?ID=<uuid>`).
- 409 Create in provider → the unit ID already exists. Either import it (`terraform import opentaco_unit.NAME <id>`), change the `id`, or `./taco unit rm <id>` before applying.
