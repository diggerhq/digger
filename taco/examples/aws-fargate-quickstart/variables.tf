variable "region"            { type = string }
variable "name_prefix" {
  type    = string
  default = "statesman"
}

variable "vpc_id"            { type = string }
variable "public_subnet_ids" { type = list(string) }

variable "container_image"   { type = string }
variable "container_port" {
  type    = number
  default = 8080
}

# App config (non-secrets)
variable "opentaco_s3_bucket"  { type = string }
variable "opentaco_s3_region"  { type = string }
variable "opentaco_s3_prefix"  { type = string }

variable "opentaco_auth_issuer"    { type = string }
variable "opentaco_auth_client_id" { type = string }
variable "opentaco_auth_auth_url"  { type = string }
variable "opentaco_auth_token_url" { type = string }

variable "opentaco_port"         { type = number }
variable "opentaco_storage"      { type = string }
# Keep as string if your app expects "true"/"false"
variable "opentaco_auth_disable" { type = string }

variable "opentaco_public_base_url" { type = string }

# Secret
variable "opentaco_auth_client_secret" {
  type      = string
  sensitive = true
}
