locals {
  environment        = "stage"
  vpc_cidr           = "10.150.0.0/16"
  vpc_public_subnets = ["10.150.0.0/18", "10.150.64.0/18", "10.150.128.0/18"]
}
