terraform {
  required_providers {
    opentaco = {
      source = "digger/opentaco"
    }
  }
}

provider "opentaco" {
  endpoint = "http://127.0.0.1:8080"
}

# Create a state registration
resource "opentaco_state" "example" {
  id = "my-project/prod/vpc"
  
  labels = {
    environment = "production"
    team        = "infrastructure"
  }
}

# Read state metadata
data "opentaco_state" "example" {
  id = opentaco_state.example.id
}

output "state_info" {
  value = {
    id      = data.opentaco_state.example.id
    size    = data.opentaco_state.example.size
    locked  = data.opentaco_state.example.locked
    updated = data.opentaco_state.example.updated
  }
}