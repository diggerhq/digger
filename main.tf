resource "null_resource" "test49" {}

variable "TEST" {
  default = "hello"
}

variable "SECRET" {
  default = "shhh"
}

output "test" {
  value = var.TEST
}

output "secret" {
  value = var.SECRET
}
