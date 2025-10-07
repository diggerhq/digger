terraform {
  required_version = ">= 1.4.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "name_prefix" {
  description = "Prefix/name for App Runner service"
  type        = string
  default     = "opentaco-statesman"
}

variable "ecr_repo_name" {
  description = "ECR repository name containing the Statesman image"
  type        = string
  default     = "opentaco-statesman"
}

variable "image_tag" {
  description = "Image tag to deploy"
  type        = string
  default     = "latest"
}

variable "container_port" {
  description = "Application port"
  type        = number
  default     = 8080
}

variable "bucket_name" {
  description = "S3 bucket for OpenTaco state"
  type        = string
}

variable "bucket_prefix" {
  description = "Prefix within the S3 bucket"
  type        = string
}

variable "opentaco_auth_disable" {
  description = "Disable auth for initial setup (recommended to start)"
  type        = bool
  default     = true
}

variable "opentaco_auth_issuer" {
  type        = string
  default     = ""
}

variable "opentaco_auth_client_id" {
  type        = string
  default     = ""
}

variable "opentaco_auth_client_secret" {
  type        = string
  default     = ""
  sensitive   = true
}

variable "opentaco_auth_auth_url" {
  type        = string
  default     = ""
}

variable "opentaco_auth_token_url" {
  type        = string
  default     = ""
}

variable "opentaco_public_base_url" {
  description = "Set to the App Runner service URL once known (second apply)"
  type        = string
  default     = ""
}

