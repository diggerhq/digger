resource "null_resource" "test44" {}

variable "test" {
  default = "hello"
}

output "test" {
  value = var.test
}
