terraform {
  required_providers {
    opentaco = {
      source  = "digger/opentaco"
    }
  }
  backend "http" {
    address        = "http://localhost:8080/v1/backend/__opentaco_system"
    lock_address   = "http://localhost:8080/v1/backend/__opentaco_system"
    unlock_address = "http://localhost:8080/v1/backend/__opentaco_system"
  }
}

provider "opentaco" {
  endpoint = "http://localhost:8080"
}

# Explicitly declare units so they are registered and visible in listings
resource "opentaco_unit" "a" { id = "org/app/A" }
resource "opentaco_unit" "b" { id = "org/app/B" }
resource "opentaco_unit" "c" { id = "org/app/C" }

# A -> B on db_url
resource "opentaco_dependency" "a_to_b_dburl" {
  from_unit_id = "org/app/A"
  from_output   = "db_url"
  to_unit_id   = "org/app/B"
  to_input      = "db_url"

  depends_on = [opentaco_unit.a, opentaco_unit.b]
}

# B -> C on image_tag
resource "opentaco_dependency" "b_to_c_image" {
  from_unit_id = "org/app/B"
  from_output   = "image_tag"
  to_unit_id   = "org/app/C"
  to_input      = "image_tag"

  depends_on = [opentaco_unit.b, opentaco_unit.c]
}
