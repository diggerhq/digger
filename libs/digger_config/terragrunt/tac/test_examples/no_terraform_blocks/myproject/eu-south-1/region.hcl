locals {
  aws_region      = "eu-south-1"
  region_azs      = ["${local.aws_region}a", "${local.aws_region}b", "${local.aws_region}c"]
  infra_vpc_cidr  = "10.250.0.0/16"
  stage_vpc_cidr  = "10.150.0.0/16"
}
