terraform {
  # Configure OpenTaco as Terraform Cloud backend
  cloud {
    hostname = "localhost:8080"  # Replace with your OpenTaco server
    
    workspaces {
      name = "my-app-production"
    }
  }
}

# Example resource to demonstrate cloud block functionality
resource "random_string" "example" {
  length  = 16
  special = false
}

# Output to demonstrate state management
output "random_value" {
  value = random_string.example.result
}

# Demonstrate OpenTaco dependency tracking
resource "opentaco_dependency" "example" {
  from_unit   = "examples/cloud-block"
  from_output = "random_value"
  to_unit     = "another-workspace"
  to_input    = "shared_value"
}
