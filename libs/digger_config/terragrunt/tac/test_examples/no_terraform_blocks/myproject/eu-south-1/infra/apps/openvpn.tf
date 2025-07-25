locals {
  vm_name           = "openvpn"
  user_data_openvpn = <<EOF
#cloud-config
hostname: ${local.vm_name}
fqdn: ${local.vm_name}.${var.env}.local
manage_etc_hosts: false
system_info:
  default_user:
    name: myuser
EOF
}

resource "random_shuffle" "subnet" {
  input        = var.public_subnets
  result_count = 1
}

module "openvpn" {
  source = "github.com/Sebor/terraform-aws-ec2-instance"

  name             = local.vm_name
  instance_count   = 1
  user_data_base64 = base64encode(local.user_data_openvpn)

  ami                         = data.aws_ami.ubuntu_focal_arm.id
  ebs_optimized               = true
  instance_type               = "t4g.micro"
  subnet_id                   = random_shuffle.subnet.result[0]
  associate_public_ip_address = true
  key_name                    = "test"

  root_block_device = [
    {
      volume_type = "gp3"
      volume_size = 30
    },
  ]

  vpc_security_group_ids = [var.sg_vpc_local_id, var.sg_ssh_global_id, var.sg_openvpn_global_id]
}

resource "aws_eip" "openvpn" {
  instance = module.openvpn.id[0]
  vpc      = true
}

resource "aws_route53_record" "openvpn" {
  zone_id = values(var.dns_zone_id)[0]
  name    = "${local.vm_name}.${var.env}.local"
  type    = "A"
  ttl     = "300"
  records = module.openvpn.private_ip
}
