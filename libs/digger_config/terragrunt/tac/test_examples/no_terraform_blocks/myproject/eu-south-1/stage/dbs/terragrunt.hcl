include {
  path = find_in_parent_folders()
}

locals {
  # Automatically load environment-level variables
  environment_vars = read_terragrunt_config(find_in_parent_folders("env.hcl"))
  region_vars      = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Extract out common variables for reuse
  env    = local.environment_vars.locals.environment
  region = local.region_vars.locals.aws_region
}

dependency "network" {
  config_path                             = "../network"
  mock_outputs_allowed_terraform_commands = ["plan", "plan-all"]
  mock_outputs = {
    vpc_id          = "vpc-1234567890"
    public_subnets  = ["az-1", "az-2", "az-3"]
    sg_vpc_local_id = "sg-2443645664"
    dns_zone_id     = { "zone.local" : "PZ1234567890" }
  }
}

inputs = {
  env             = local.env
  region          = local.region
  vpc_id          = dependency.network.outputs.vpc_id
  public_subnets  = dependency.network.outputs.public_subnets
  sg_vpc_local_id = dependency.network.outputs.sg_vpc_local_id
  dns_zone_id     = dependency.network.outputs.dns_zone_id
}
