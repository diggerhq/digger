# S3-Compatible Backend Example

This example shows how to use Terraform's native `s3` backend against OpenTaco's S3-compatible endpoint at `/s3`.

## How It Works (S3-compat only)
- Endpoint: `/s3/<bucket>/<unit-id>/terraform.tfstate[.lock|.tflock]`.
- Auth: AWS SigV4. The `X-Amz-Security-Token` must be an OpenTaco access token (audience includes `s3`).
- Credentials: Terraform obtains short‑lived “process credentials” by calling `taco creds --json` via `credential_process` in your AWS profile.
- Locking: Terraform writes a lockfile (`terraform.tfstate.tflock`). The service treats lock PUT as idempotent and allows DELETE without a body.
- Empty unit: GET/HEAD returns 404 so Terraform initializes the tfstate instead of polling a zero‑byte object.

## Prerequisites
- OpenTaco service running locally on `:8080` (memory storage is fine for a quick test):
  - `./statesman -storage memory`
- CLI built and logged in (saves tokens under `~/.config/opentaco/credentials.json`):
  - `./taco login --server http://localhost:8080`
- Terraform 1.13+ (for `use_lockfile = true`).

## Configure AWS Profile
Add this to `~/.aws/config` (use an absolute path and quote it):

```
[profile opentaco-state-backend]
region = auto
credential_process = "/absolute/path/to/taco" creds --json --server http://localhost:8080
```

Then in your shell:

```
export AWS_SDK_LOAD_CONFIG=1
export AWS_PROFILE=opentaco-state-backend
```

## Terraform Backend
This directory contains a minimal `main.tf` that uses the OpenTaco `/s3` endpoint and a trivial `null_resource` to exercise plan/apply.

Key parts of the backend block:

```hcl
terraform {
  backend "s3" {
    bucket  = "opentaco"
    key     = "s3compatdemo/terraform.tfstate"
    endpoints = { s3 = "http://localhost:8080/s3" }
    use_path_style                 = true
    skip_credentials_validation    = true
    skip_region_validation         = true
    skip_requesting_account_id     = true
    use_lockfile                   = true
    profile                        = "opentaco-state-backend"
  }
}
```

## Run
From this directory:

```
terraform init -reconfigure
terraform plan
terraform apply -auto-approve
```

You should see `/s3` requests in the service logs including:
- `PUT ... terraform.tfstate.tflock` (lock acquire)
- `PUT ... terraform.tfstate` (state write)
- `DELETE ... terraform.tfstate.tflock` (unlock)

## Troubleshooting
- `401` from `taco creds` → re-login: `./taco login --force-login` (or set a stable signing key on the server).
- `credential_process` exit 126 → use an absolute, quoted path to `taco` and `chmod +x` it.
- Init loops on GET `terraform.tfstate` → ensure you’re running a build where empty state returns 404 and `use_lockfile = true` is set.
- `423 Locked` during plan/apply → unlock via management API:
- `./taco unit unlock s3compatdemo <lock-id> --server http://localhost:8080`

For more details, see `docs/s3-compat.md`.
