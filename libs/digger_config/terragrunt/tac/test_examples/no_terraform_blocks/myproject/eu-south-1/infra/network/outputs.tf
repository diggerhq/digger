# VPC outputs
output "vpc_id" {
  description = "The ID of the VPC"
  value       = module.vpc.vpc_id
}

output "vpc_cidr_block" {
  description = "The CIDR block of the VPC"
  value       = module.vpc.vpc_cidr_block
}

output "public_subnets" {
  description = "List of IDs of public subnets"
  value       = module.vpc.public_subnets
}

output "azs" {
  description = "A list of availability zones"
  value       = module.vpc.azs
}

output "default_security_group_id" {
  description = "Default SG ID of VPC"
  value       = module.vpc.default_security_group_id
}

output "default_route_table_id" {
  description = "The ID of the default route table"
  value       = module.vpc.default_route_table_id
}

output "vpc_igw_id" {
  description = "The ID of the Internet Gateway"
  value       = module.vpc.igw_id
}

output "public_igw_route_id" {
  description = "ID of the internet gateway route"
  value       = module.vpc.public_internet_gateway_route_id
}

output "public_route_table_ids" {
  description = "List of IDs of public route tables"
  value       = module.vpc.public_route_table_ids
}

output "vpc_owner_id" {
  description = "The ID of the AWS account that owns the VPC"
  value       = module.vpc.vpc_owner_id
}

output "vpc_main_route_table_id" {
  description = "The ID of the main route table associated with this VPC"
  value       = module.vpc.vpc_main_route_table_id
}

# VPC peering outputs
output "stage_vpc_peering_id" {
  description = "The Id of the VPC peering connection"
  value       = aws_vpc_peering_connection.infra_stage.id
}

# Sg outputs
output "sg_vpc_local_id" {
  description = "The ID of the local security group"
  value       = module.sg_vpc_local.this_security_group_id
}

output "sg_vpc_local_name" {
  description = "The Name of the local security group"
  value       = module.sg_vpc_local.this_security_group_name
}

output "sg_ssh_global_id" {
  description = "The ID of the global SSH security group"
  value       = module.sg_ssh_global.this_security_group_id
}

output "sg_ssh_global_name" {
  description = "The Name of the global SSH security group"
  value       = module.sg_ssh_global.this_security_group_name
}

output "sg_web_global_id" {
  description = "The ID of the global WEB security group"
  value       = module.sg_web_global.this_security_group_id
}

output "sg_web_global_name" {
  description = "The Name of the global WEB security group"
  value       = module.sg_web_global.this_security_group_name
}

output "sg_openvpn_global_id" {
  description = "The ID of the global OpenVPN security group"
  value       = module.sg_openvpn_global.this_security_group_id
}

output "sg_openvpn_global_name" {
  description = "The Name of the global OpenVPN security group"
  value       = module.sg_openvpn_global.this_security_group_name
}

### DNS zones outputs
output "dns_zone_id" {
  description = "Zone ID of Route53 zone"
  value       = module.dns_zone.this_route53_zone_zone_id
}

output "dns_zone_name_servers" {
  description = "Zone ID of Route53 zone"
  value       = module.dns_zone.this_route53_zone_name_servers
}
