terraform {
  backend "http" {
    address        = "http://localhost:8080/v1/backend/org/app/A"
    lock_address   = "http://localhost:8080/v1/backend/org/app/A"
    unlock_address = "http://localhost:8080/v1/backend/org/app/A"
  }
}

output "db_url" {
  # Changes on every apply to simulate new upstream output
  value = "postgres://a.example/db-${timestamp()}"
}

output "subnet_ids" {
  value = ["subnet-1234", "subnet-4567"]
}
