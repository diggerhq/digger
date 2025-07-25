# Terragrunt will copy the Terraform configurations specified by the source parameter, along with any files in the
# working directory, into a temporary folder, and execute your Terraform commands in that folder.
terraform {
  source = "git::git@github.com:gruntwork-io/terragrunt-infrastructure-modules-example.git//vpc?ref=v0.3.0"
}

# Include all settings from the root terragrunt.hcl file
include {
  path = find_in_parent_folders()
}

locals {
  # Automatically load environment-level variables
  env_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  # Extract out common variables for reuse
  stack_name = local.env_vars.locals.stack_name
}

dependency "tgw" {
  config_path = "../../../../../network-account/eu-west-1/network/transit-gateway/"
}

inputs = {
  name                      = local.stack_name
  transit_gateway_id        = dependency.tgw.outputs.this_ec2_transit_gateway_id
}
