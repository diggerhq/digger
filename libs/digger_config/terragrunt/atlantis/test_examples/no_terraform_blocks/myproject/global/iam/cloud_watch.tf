module "cloudwatch_agent_role" {
  source = "github.com/terraform-aws-modules/terraform-aws-iam/modules/iam-assumable-role"

  create_role       = true
  role_name         = "CloudWatchAgentServerRole"
  role_requires_mfa = false

  trusted_role_services = [
    "ec2.amazonaws.com"
  ]

  custom_role_policy_arns = [
    "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy",
  ]
}

resource "aws_iam_instance_profile" "cloudwatch_agent" {
  name = module.cloudwatch_agent_role.this_iam_role_name
  role = module.cloudwatch_agent_role.this_iam_role_name
}
