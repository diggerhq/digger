variable "env" {
  type = string
}

variable "region" {
  type = string
}

variable "public_dns_zones" {
  type = set(string)
}
