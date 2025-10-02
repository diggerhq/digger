region            = "us-west-2"
vpc_id            = "vpc-0123abcd"
public_subnet_ids = ["subnet-aaa", "subnet-bbb"]
container_image   = "ghcr.io/diggerhq/digger/taco-statesman:latest"

opentaco_s3_bucket   = "your-s3-bucket"
opentaco_s3_region   = "us-east-1"
opentaco_s3_prefix   = "your-prefix"



opentaco_auth_issuer    = "https://login.microsoftonline.com/your-tenant-id/v2.0" # no trailing slash! 
opentaco_auth_client_id = "your-application-client-id"
opentaco_auth_auth_url  = "https://login.microsoftonline.com/your-tenant-id/oauth2/v2.0/authorize"
opentaco_auth_token_url =  "https://login.microsoftonline.com/your-tenant-id/oauth2/v2.0/token"

opentaco_port         = 8080
opentaco_storage      = "s3"
opentaco_auth_disable = "false"
opentaco_public_base_url = "https://your-cloudfront-instance.cloudfront.net"
# Keep this out of git; set via TF_VAR_... or a secrets manager:
opentaco_auth_client_secret = "your-client-secret"
