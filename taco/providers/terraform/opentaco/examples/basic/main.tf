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

# Create a unit registration
resource "opentaco_unit" "example" {
  id = "my-project/prod/vpc"
  
  labels = {
    environment = "production"
    team        = "infrastructure"
  }
}

# Read unit metadata
data "opentaco_unit" "example" {
  id = opentaco_unit.example.id
}

output "unit_info" {
  value = {
    id      = data.opentaco_unit.example.id
    size    = data.opentaco_unit.example.size
    locked  = data.opentaco_unit.example.locked
    updated = data.opentaco_unit.example.updated
  }
}
