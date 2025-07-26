# data "aws_ami" "proxy_ami" {
#   most_recent = true
#   owners      = ["self"]

#   filter {
#     name = "name"
#     values = [
#       "debian-buster-proxy-ami*",
#     ]
#   }
# }

data "aws_ami" "ubuntu_focal_arm" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "architecture"
    values = ["arm64"]
  }

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal*"]
  }
}

data "aws_ami" "ubuntu_focal_x86" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal*"]
  }
}
