variable "env" {
  type = string
}

variable "region" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnets" {
  type = list(string)
}

variable "sg_vpc_local_id" {
  type = string
}

variable "dns_zone_id" {
  type = map(any)
}
