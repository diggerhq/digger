// Example Terraform project using the OpenTaco HTTP backend

terraform {
  backend "http" {
    # Backend pointing to the state you created via the provider workspace
    address        = "http://localhost:8080/v1/backend/myapp/prod5"
    lock_address   = "http://localhost:8080/v1/backend/myapp/prod5"
    unlock_address = "http://localhost:8080/v1/backend/myapp/prod5"

    # Alternatively, use the double-underscore variant if you prefer a single path segment:
    # address        = "http://localhost:8080/v1/backend/myapp__prod5"
    # lock_address   = "http://localhost:8080/v1/backend/myapp__prod5"
    # unlock_address = "http://localhost:8080/v1/backend/myapp__prod5"
  }
}

# Minimal config so `terraform apply` writes state via the backend
output "demo" {
  value = "OpenTaco backend is working 77777777777"
}

