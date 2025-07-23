module "sg_vpc_local" {
  source = "github.com/terraform-aws-modules/terraform-aws-security-group"

  name                = "all_local_vpc_traffic"
  vpc_id              = module.vpc.vpc_id
  ingress_cidr_blocks = [module.vpc.vpc_cidr_block]
  ingress_rules       = ["all-all"]
  egress_rules        = ["all-all"]
}

module "sg_ssh_global" {
  source = "github.com/terraform-aws-modules/terraform-aws-security-group"

  name   = "ssh_global_traffic"
  vpc_id = module.vpc.vpc_id

  ingress_cidr_blocks      = ["0.0.0.0/0"]
  ingress_ipv6_cidr_blocks = ["::/0"]
  ingress_rules            = ["ssh-tcp"]
  egress_rules             = ["all-all"]
}

module "sg_web_global" {
  source = "github.com/terraform-aws-modules/terraform-aws-security-group"

  name   = "web_global_traffic"
  vpc_id = module.vpc.vpc_id

  ingress_cidr_blocks      = ["0.0.0.0/0"]
  ingress_ipv6_cidr_blocks = ["::/0"]
  ingress_rules            = ["https-443-tcp", "http-80-tcp"]
  egress_rules             = ["all-all"]
}

module "sg_openvpn_global" {
  source = "github.com/terraform-aws-modules/terraform-aws-security-group"

  name   = "openvpn_global_traffic"
  vpc_id = module.vpc.vpc_id

  ingress_cidr_blocks      = ["0.0.0.0/0"]
  ingress_ipv6_cidr_blocks = ["::/0"]
  ingress_rules            = ["openvpn-udp", "openvpn-tcp"]
  egress_rules             = ["all-all"]
}
