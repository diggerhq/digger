# Digger HTTP State Backend

A basic HTTP state backend for Terraform that stores state files in an S3 bucket. This implementation provides a simple HTTP API that Terraform can use as a remote state backend.

## Features

- **HTTP API**: RESTful endpoints for state operations
- **S3 Storage**: All state files are stored in an AWS S3 bucket
- **Terraform Compatible**: Implements the standard Terraform HTTP backend interface
- **Logging**: Structured logging with slog
- **Health Checks**: Built-in health check endpoint

## API Endpoints

### Health Check

```
GET /health
```

Returns the health status of the service.

### State Operations

#### Get State

```
GET /state/{key}
```

Retrieves a state file from S3.

#### Set State

```
POST /state/{key}
```

Stores a state file to S3. The state data should be sent in the request body.

#### Delete State

```
DELETE /state/{key}
```

Removes a state file from S3.

#### Head State

```
HEAD /state/{key}
```

Checks if a state file exists in S3 and returns metadata.

## Configuration

The service is configured using environment variables:

- `S3_BUCKET` (required): The S3 bucket name where state files will be stored
- `AWS_REGION` (optional): AWS region for the S3 bucket (defaults to `us-east-1`)
- `PORT` (optional): HTTP server port (defaults to `8080`)

### AWS Authentication

The service uses the AWS SDK's default credential chain, which supports:

- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
- AWS credentials file
- IAM roles (when running on EC2 or ECS)
- AWS SSO profiles

## Usage with Terraform

To use this backend with Terraform, configure your Terraform backend like this:

```hcl
terraform {
  backend "http" {
    address = "http://your-state-backend:8080/state/terraform.tfstate"
    lock_address = "http://your-state-backend:8080/state/terraform.tfstate"
    unlock_address = "http://your-state-backend:8080/state/terraform.tfstate"
  }
}
```

Or using the `-backend-config` flag:

```bash
terraform init \
  -backend-config="address=http://your-state-backend:8080/state/terraform.tfstate" \
  -backend-config="lock_address=http://your-state-backend:8080/state/terraform.tfstate" \
  -backend-config="unlock_address=http://your-state-backend:8080/state/terraform.tfstate"
```

## Building and Running

### Build

```bash
go build -o digger-state-backend .
```

### Run

```bash
export S3_BUCKET=your-terraform-state-bucket
export AWS_REGION=us-west-2
./digger-state-backend
```

### Docker

```bash
docker build -t digger-state-backend .
docker run -e S3_BUCKET=your-terraform-state-bucket -e AWS_REGION=us-west-2 -p 8080:8080 digger-state-backend
```

## State File Organization

State files are stored in S3 using the key provided in the URL path. For example:

- `/state/project1/terraform.tfstate` → S3 key: `project1/terraform.tfstate`
- `/state/environments/dev/terraform.tfstate` → S3 key: `environments/dev/terraform.tfstate`

## Security Considerations

- Ensure your S3 bucket has appropriate access controls
- Use HTTPS in production environments
- Consider implementing authentication/authorization for the HTTP endpoints
- Use IAM roles with minimal required permissions
- Enable S3 bucket versioning for state file recovery

## Limitations

- No built-in locking mechanism (Terraform handles this)
- No encryption at rest (relies on S3 bucket policies)
- No authentication/authorization (should be added for production use)
- No state file validation (Terraform handles validation)

## Development

### Prerequisites

- Go 1.24.0 or later
- AWS credentials configured

### Running Tests

```bash
go test ./...
```

### Local Development

```bash
# Set up local environment
export S3_BUCKET=your-test-bucket
export AWS_REGION=us-east-1

# Run the server
go run .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is part of the Digger ecosystem and follows the same licensing terms.
