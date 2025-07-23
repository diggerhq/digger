locals {
  # Automatically load account-level variables
  account_vars = read_terragrunt_config(find_in_parent_folders("account.hcl"))

  # Automatically load region-level variables
  region_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Automatically load environment-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))

  # Extract the variables we need for easy access
  account_name      = local.account_vars.locals.account_name
  account_id        = local.account_vars.locals.aws_account_id
  aws_region        = local.region_vars.locals.aws_region
  aws_profile       = local.account_vars.locals.aws_profile_name
  tf_s3_bucket      = local.account_vars.locals.tf_s3_bucket
  tf_dynamodb_table = local.account_vars.locals.tf_dynamodb_table

  # Get AWS_PROFILE
  tf_aws_profile_name = get_env("TF_AWS_PROFILE_NAME", "${local.aws_profile}")
}

terraform {
  extra_arguments "aws_profile" {
    commands = [
      "init",
      "apply",
      "refresh",
      "import",
      "plan",
      "taint",
      "untaint"
    ]

    env_vars = {
      AWS_PROFILE = "${local.tf_aws_profile_name}"
    }
  }
}

remote_state {
  backend = "s3"
  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
  config = {
    bucket         = "${local.tf_s3_bucket}"
    region         = "${local.aws_region}"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    encrypt        = true
    dynamodb_table = "${local.tf_dynamodb_table}"
    profile        = "${local.tf_aws_profile_name}"
  }
}

generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region                  = "${local.aws_region}"
  skip_region_validation  = true
  skip_metadata_api_check = true
  profile                 = "${local.tf_aws_profile_name}"
}

provider "aws" {
  alias                   = "prod"
  region                  = "eu-north-1"
  skip_region_validation  = true
  skip_metadata_api_check = true
  profile                 = "${local.tf_aws_profile_name}"
}
EOF
}

generate "versions" {
  path      = "versions.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.29.1"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1.0"
    }
    postgresql = {
      source  = "cyrilgdn/postgresql"
      version = "~> 1.11.2"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0.2"
    }
    helm = {
      source = "hashicorp/helm"
      version = "~> 2.0.2"
    }
    cloudflare = {
      source = "cloudflare/cloudflare"
      version = "~> 2.18.0"
    }
  }
  required_version = ">= 0.14"
}
EOF
}

inputs = merge(
  local.account_vars.locals,
  local.region_vars.locals,
  local.environment_vars.locals,
)
