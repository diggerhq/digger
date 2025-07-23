include {
  path = find_in_parent_folders()
}

terraform {
  source = "git::git@github.com:transcend-io/terraform-aws-fargate-container?ref=v0.0.4"
}

locals {
  atlantis_workflow = "workflowSpecifiedInChild"
}

inputs = {
  foo = "bar"
}