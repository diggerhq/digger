terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/org/app/C"
    lock_address   = "http://localhost:8080/v1/backend/org/app/C"
    unlock_address = "http://localhost:8080/v1/backend/org/app/C"
  }
}

# C does not need any particular outputs; any write will acknowledge incoming edges.
locals {
  name = "service-c"
}

output "name" {
  value = local.name
}
