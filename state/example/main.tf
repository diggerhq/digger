terraform {
  backend "http" {
    address        = "http://localhost:8080/state/example/terraform.tfstate"
    lock_address   = "http://localhost:8080/state/example/terraform.tfstate"
    unlock_address = "http://localhost:8080/state/example/terraform.tfstate"
  }
}

# Example resource
resource "aws_s3_bucket" "example" {
  bucket = "my-example-bucket-${random_id.bucket_suffix.hex}"
}

resource "random_id" "bucket_suffix" {
  byte_length = 4
}

output "bucket_name" {
  value = aws_s3_bucket.example.bucket
}
