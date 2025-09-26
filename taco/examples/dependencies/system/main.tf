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
resource "opentaco_unit" "dev_vpc" { id = "dev-vpc" }
resource "opentaco_unit" "dev_cluster" { id = "dev-cluster" }
resource "opentaco_unit" "dev_db" { id = "dev-db" }

# A -> B on db_url
resource "opentaco_dependency" "a_to_b_vpc" {
  from_unit_id = "dev-vpc"
  from_output   = "vpc_id"
  to_unit_id   = "dev-cluster"
  to_input      = "vpc_id"

  depends_on = [opentaco_unit.dev_vpc, opentaco_unit.dev_cluster]
}

resource "opentaco_dependency" "b_to_c_dburl" {
  from_unit_id = "dev-cluster"
  from_output   = "dburl"
  to_unit_id   = "dev-database"
  to_input      = "dburl"

  depends_on = [opentaco_unit.dev_cluster, opentaco_unit.dev_db]
}
