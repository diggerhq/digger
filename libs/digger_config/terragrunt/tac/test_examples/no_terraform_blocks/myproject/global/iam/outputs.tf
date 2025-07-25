### CloudWatch Agent Outputs
output "cloudwatch_agent_iam_role_arn" {
  description = "ARN of IAM role"
  value       = module.cloudwatch_agent_role.this_iam_role_arn
}

output "cloudwatch_agent_iam_role_name" {
  description = "Name of IAM role"
  value       = module.cloudwatch_agent_role.this_iam_role_name
}

output "cloudwatch_agent_iam_role_path" {
  description = "Path of IAM role"
  value       = module.cloudwatch_agent_role.this_iam_role_path
}

output "cloudwatch_agent_iam_profile_id" {
  description = "ID of IAM profile"
  value       = aws_iam_instance_profile.cloudwatch_agent.id
}

output "cloudwatch_agent_iam_profile_arn" {
  description = "ARN of IAM profile"
  value       = aws_iam_instance_profile.cloudwatch_agent.arn
}

output "cloudwatch_agent_iam_profile_name" {
  description = "Name of IAM profile"
  value       = aws_iam_instance_profile.cloudwatch_agent.name
}
