# ---------------------------------------------------------------------------------------------------------------------
# TERRAGRUNT CONFIGURATION
# This is the configuration for Terragrunt, a thin wrapper for Terraform that helps keep your code DRY and
# maintainable: https://github.com/gruntwork-io/terragrunt
# ---------------------------------------------------------------------------------------------------------------------

# ---------------------------------------------------------------------------------------------------------------------
# Include configurations that are common used across multiple environments.
# ---------------------------------------------------------------------------------------------------------------------

# Include the root `terragrunt.hcl` configuration. The root configuration contains settings that are common across all
# components and environments, such as how to configure remote state.
include "root" {
  path = find_in_parent_folders()
}

# Include the envcommon configuration for the component. The envcommon configuration contains settings that are common
# for the component across all environments.
include "envcommon" {
  path = "${dirname(find_in_parent_folders())}/_envcommon/webserver-cluster.hcl"
}


# ---------------------------------------------------------------------------------------------------------------------
# Override parameters for this environment
# ---------------------------------------------------------------------------------------------------------------------

# For production, we want to specify bigger instance classes and cluster, so we specify override parameters here. These
# inputs get merged with the common inputs from the root and the envcommon terragrunt.hcl
inputs = {
  instance_type = "t2.medium"

  min_size = 3
  max_size = 3
}
