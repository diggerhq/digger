terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/org/app/B"
    lock_address   = "http://localhost:8080/v1/backend/org/app/B"
    unlock_address = "http://localhost:8080/v1/backend/org/app/B"
  }
}

locals {
  # Some trivial output to ensure state writes on apply
  image_tag = "v1.0.0"
}

output "image_tag" {
  value = local.image_tag
}
