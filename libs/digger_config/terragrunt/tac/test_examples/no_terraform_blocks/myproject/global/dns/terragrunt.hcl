include {
  path = find_in_parent_folders()
}

locals {
  # Automatically load environment-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  region_vars      = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Extract out common variables for reuse
  env              = local.environment_vars.locals.environment
  region           = local.region_vars.locals.aws_region
  public_dns_zones = local.environment_vars.locals.public_dns_zones
}

inputs = {
  env              = local.env
  region           = local.region
  public_dns_zones = local.public_dns_zones
}
