terraform {
backend "s3" {
    bucket  = "opentaco"
    key     = "s3compatdemo/terraform.tfstate"
    endpoints = { s3 = "http://localhost:8080/s3" }
    region    = "any"
    use_path_style                 = true
    skip_credentials_validation    = true
    skip_region_validation         = true
    skip_requesting_account_id     = true
    use_lockfile                   = true
    profile                        = "opentaco-state-backend"
}

required_providers {
    null = {
      source = "hashicorp/null"
    }
}
}

resource "null_resource" "smoke" {
triggers = {
    ts = timestamp()
}
}