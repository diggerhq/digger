locals {
  environment        = "infra"
  vpc_cidr           = "10.250.0.0/16"
  vpc_public_subnets = ["10.250.0.0/18", "10.250.64.0/18", "10.250.128.0/18"]
}
