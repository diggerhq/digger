locals {
  # Automatically load environment-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))

  # Extract out common variables for reuse
  env = local.environment_vars.locals.env
}

# Terragrunt will copy the Terraform configurations specified by the source parameter, along with any files in the
# working directory, into a temporary folder, and execute your Terraform commands in that folder.
terraform {
  source = "git::git@github.com:gruntwork-io/terragrunt-infrastructure-modules-example.git//zone?ref=v0.3.0"
}

# Include all settings from the root terragrunt.hcl file
include {
  path = find_in_parent_folders()
}

dependency "vpc" {
  config_path = "../../../env-a/network/vpc/"
}

# These are the variables we have to pass in to use the module specified in the terragrunt configuration above
inputs = {
  name           = "test_zone_${local.env}"
}

