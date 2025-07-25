include {
  path = find_in_parent_folders()
}

locals {
  # Automatically load environment-level and region-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  region_vars      = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Extract out common variables for reuse
  env           = local.environment_vars.locals.environment
  region        = local.region_vars.locals.aws_region
  azs           = local.region_vars.locals.region_azs
  cidr          = local.environment_vars.locals.vpc_cidr
  subnets       = local.environment_vars.locals.vpc_public_subnets
  domain_prefix = local.environment_vars.locals.environment
}

dependency "stage_network" {
  config_path                             = "../../stage/network"
  mock_outputs_allowed_terraform_commands = ["plan", "plan-all"]
  mock_outputs = {
    vpc_id                 = "vpc-1234567890"
    vpc_cidr_block         = "192.168.0.0/24"
    public_route_table_ids = ["rtb-0b041f70d102fhu37"]
  }
}

inputs = {
  vpc_name                         = local.env
  vpc_cidr                         = local.cidr
  aws_region                       = local.region
  vpc_azs                          = local.azs
  vpc_public_subnets               = local.subnets
  domain_prefix                    = local.domain_prefix
  stage_vpc_id                     = dependency.stage_network.outputs.vpc_id
  stage_vpc_cidr                   = dependency.stage_network.outputs.vpc_cidr_block
  stage_vpc_public_route_table_ids = dependency.stage_network.outputs.public_route_table_ids
}
