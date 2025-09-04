output "pg_db_instance_address" {
  description = "The address of the RDS instance"
  value       = module.pg_db.this_db_instance_address
}

output "pg_db_instance_availability_zone" {
  description = "The availability zone of the RDS instance"
  value       = module.pg_db.this_db_instance_availability_zone
}

output "pg_db_instance_endpoint" {
  description = "The connection endpoint"
  value       = module.pg_db.this_db_instance_endpoint
}

output "pg_db_instance_hosted_zone_id" {
  description = "The canonical hosted zone ID of the DB instance (to be used in a Route 53 Alias record)"
  value       = module.pg_db.this_db_instance_hosted_zone_id
}

output "pg_db_instance_id" {
  description = "The RDS instance ID"
  value       = module.pg_db.this_db_instance_id
}

output "pg_db_instance_username" {
  description = "The master username for the database"
  value       = module.pg_db.this_db_instance_username
}

output "pg_db_instance_port" {
  description = "The database port"
  value       = module.pg_db.this_db_instance_port
}

output "pg_db_subnet_group_id" {
  description = "The db subnet group name"
  value       = module.pg_db.this_db_subnet_group_id
}

output "pg_db_parameter_group_id" {
  description = "The db parameter group id"
  value       = module.pg_db.this_db_parameter_group_id
}

output "pg_db_dns_canonical_name" {
  description = "The db parameter group id"
  value       = aws_route53_record.pg_db.fqdn
}
