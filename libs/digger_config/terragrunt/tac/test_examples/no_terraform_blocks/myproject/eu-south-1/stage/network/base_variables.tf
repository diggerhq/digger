variable "vpc_name" {
  type = string
}

variable "vpc_cidr" {
  type = string
}

variable "vpc_azs" {
  type = list(any)
}

variable "vpc_public_subnets" {
  type = list(any)
}

variable "aws_region" {
  type = string
}

variable "infra_vpc_cidr" {
  type = string
}
