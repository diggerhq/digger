include {
  path = find_in_parent_folders()
}

locals {
  # Automatically load environment-level and region-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  region_vars      = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Extract out common variables for reuse
  env            = local.environment_vars.locals.environment
  region         = local.region_vars.locals.aws_region
  azs            = local.region_vars.locals.region_azs
  cidr           = local.environment_vars.locals.vpc_cidr
  subnets        = local.environment_vars.locals.vpc_public_subnets
  infra_vpc_cidr = local.region_vars.locals.infra_vpc_cidr
}

inputs = {
  vpc_name           = local.env
  vpc_cidr           = local.cidr
  aws_region         = local.region
  vpc_azs            = local.azs
  vpc_public_subnets = local.subnets
  infra_vpc_cidr     = local.infra_vpc_cidr
}
