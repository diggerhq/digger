module "vpc" {
  source = "github.com/terraform-aws-modules/terraform-aws-vpc"

  name = "${var.vpc_name}-vpc"
  cidr = var.vpc_cidr

  azs            = var.vpc_azs
  public_subnets = var.vpc_public_subnets

  map_public_ip_on_launch          = true
  enable_dns_hostnames             = true
  enable_dns_support               = true
  enable_dhcp_options              = true
  dhcp_options_domain_name         = "${var.vpc_name}.local"
  dhcp_options_domain_name_servers = ["169.254.169.253", "AmazonProvidedDNS"]
  dhcp_options_ntp_servers         = ["169.254.169.123"]

  tags = {
    Terraform = "true"
    Env       = var.vpc_name
  }
}
