---
title: Troubleshooting
description: Common issues and quick fixes.
---

# Troubleshooting

405 Method Not Allowed on LOCK/UNLOCK
- Cause: Terraform backend uses custom HTTP verbs; routes not wired.
- Fix: Add explicit routes in Echo: `e.Add("LOCK", "/v1/backend/*", ...)` and `e.Add("UNLOCK", "/v1/backend/*", ...)`. Rebuild and restart.

409 Failed to save state (POST/PUT)
- Cause: Backend update handler doesn’t read lock ID from query.
- Fix: Read lock ID from header `X-Terraform-Lock-ID` or query `ID`/`id`.

409 State already exists (provider Create)
- Cause: The `id` already exists remotely; renaming the Terraform resource block doesn’t change the remote ID.
- Fix: Import (`terraform import opentaco_state.NAME <id>`), change `id`, or delete the existing state (`./taco state rm <id>`).

Provider not found
- Fix (dev override):
```bash
ABS="$(pwd)/providers/terraform/opentaco"
cat > ./.terraformrc <<EOF
provider_installation {
  dev_overrides { "digger/opentaco" = "${ABS}" }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$PWD/.terraformrc"
terraform init -upgrade
```

401 Unauthorized on API/backend
- Cause: Auth is enforced by default; `/v1` and `/v1/backend/*` require `Authorization: Bearer <access>`.
- Fix:
  - Use `./taco login` and then run state commands; the CLI attaches Bearer automatically.
  - Ensure tokens exist in `~/.config/opentaco/credentials.json` under the current `--server`.
  - Temporarily disable auth for Terraform backend testing with: `./opentacosvc -auth-disable`.
