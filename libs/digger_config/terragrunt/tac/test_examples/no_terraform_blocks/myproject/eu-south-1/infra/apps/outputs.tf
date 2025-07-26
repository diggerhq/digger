### Slack outputs
output "slack_lamba_cw_log_group_arn" {
  description = "The ARN of lambda log group for slack"
  value       = module.main_slack.lambda_cloudwatch_log_group_arn
}

output "slack_lambda_iam_role_arn" {
  description = "The ARN of lambda IAM role for slack"
  value       = module.main_slack.lambda_iam_role_arn
}

output "slack_lambda_iam_role_name" {
  description = "The Name of lambda IAM role for slack"
  value       = module.main_slack.lambda_iam_role_name
}

output "slack_lambda_function_name" {
  description = "The name of the Lambda function for slack"
  value       = module.main_slack.notify_slack_lambda_function_name
}

output "slack_topic_arn" {
  description = "The ARN of the SNS topic from which messages will be sent to Slack"
  value       = module.main_slack.this_slack_topic_arn
}

### OpenVPN outputs
output "openvpn_private_ip" {
  description = "OpenVPN private address"
  value       = module.openvpn.private_ip
}

output "openvpn_private_dns" {
  description = "OpenVPN private DNS names"
  value       = module.openvpn.private_dns
}

output "openvpn_public_ip" {
  description = "OpenVPN public address"
  value       = module.openvpn.public_ip
}

output "openvpn_public_dns" {
  description = "OpenVPN public DNS names"
  value       = module.openvpn.public_dns
}

output "openvpn_public_eip_address" {
  description = "OpenVPN public address"
  value       = aws_eip.openvpn.public_ip
}

output "openvpn_subnet_id" {
  description = "OpenVPN subnet id"
  value       = module.openvpn.subnet_id
}

output "openvpn_availability_zone" {
  description = "OpenVPN availability zone"
  value       = module.openvpn.availability_zone
}

output "openvpn_instance_ids" {
  description = "OpenVPN instance ids"
  value       = module.openvpn.id
}
