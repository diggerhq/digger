---
title: Terraform Provider
description: Configure and use the OpenTaco Terraform provider.
---

# Terraform Provider

Build locally:
```bash
make build-prov
```

Configure provider:
```hcl
terraform {
  required_providers {
    opentaco = { source = "digger/opentaco" }
  }
}

provider "opentaco" {
  endpoint = "http://localhost:8080"
}
```

Resource example:
```hcl
resource "opentaco_state" "example" {
  id = "myapp/prod"
}
```

Dev override (if provider is not published):
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

