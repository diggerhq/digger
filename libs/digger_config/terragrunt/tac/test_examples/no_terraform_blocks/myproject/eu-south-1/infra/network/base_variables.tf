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

variable "domain_prefix" {
  type = string
}

# Stage VPC
variable "stage_vpc_id" {
  type = string
}

variable "stage_vpc_cidr" {
  type = string
}

variable "stage_vpc_public_route_table_ids" {
  type = list(string)
}
