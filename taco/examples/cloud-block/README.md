# Cloud Block Example

This example demonstrates using OpenTaco as a Terraform Cloud-compatible backend with the `cloud` configuration block.

## Prerequisites

1. OpenTaco server running (see getting-started guide)
2. Authentication configured (OIDC provider recommended)
3. Terraform CLI 1.1+ (cloud block support)

## Setup

1. **Start OpenTaco server**:
   ```bash
   cd ../../
   OPENTACO_S3_BUCKET=your-bucket \
   OPENTACO_S3_REGION=us-east-1 \
   OPENTACO_S3_PREFIX=opentaco/ \
   ./statesman
   ```

2. **Authenticate with OpenTaco**:
   ```bash
   terraform login localhost:8080
   ```
   
   This will open your browser and walk you through the OAuth2 flow. The credentials are automatically available to all `taco` CLI commands too!

3. **Initialize and apply**:
   ```bash
   cd examples/cloud-block/
   terraform init
   terraform plan
   terraform apply
   ```

## What Happens

1. **Service Discovery**: Terraform queries `/.well-known/terraform.json` to discover OAuth and API endpoints
2. **Authentication**: Uses stored credentials from `terraform login`
3. **Workspace Creation**: OpenTaco automatically creates the `my-app-production` workspace
4. **State Management**: State is stored and versioned in OpenTaco's backend
5. **Locking**: State is automatically locked during plan/apply operations
6. **Dependencies**: The `opentaco_dependency` resource creates a dependency edge in the system graph

## Cloud Block Options

### Single Workspace
```hcl
terraform {
  cloud {
    hostname = "your-opentaco-server.com"
    
    workspaces {
      name = "my-workspace"
    }
  }
}
```

### Multiple Workspaces with Tags
```hcl
terraform {
  cloud {
    hostname = "your-opentaco-server.com"
    organization = "my-org"  # Optional
    
    workspaces {
      tags = ["app:web", "env:production"]
    }
  }
}
```

**NEW**: For a complete tag-based workspace example, see the [cloud-block-tags example](../cloud-block-tags/).

## Authentication Flow

The authentication process follows OAuth2 with PKCE:

1. Terraform generates a code challenge
2. Browser opens to OpenTaco's authorization endpoint
3. User authenticates with OIDC provider (WorkOS, Auth0, etc.)
4. OpenTaco returns authorization code
5. Terraform exchanges code for access/refresh tokens
6. Tokens are stored in `~/.terraform.d/credentials.tfrc.json`

## Security Features

- **OAuth2/PKCE**: Secure authentication without client secrets
- **Token Refresh**: Automatic token renewal by Terraform
- **RBAC Integration**: Fine-grained workspace permissions
- **Lock Validation**: State changes require proper lock ownership
- **Audit Logging**: All operations are logged for security monitoring

## Troubleshooting

### Authentication Issues
```bash
# Re-authenticate
terraform login -force localhost:8080

# Check current user
taco whoami --server http://localhost:8080
```

### Workspace Issues
```bash
# List available workspaces
taco unit ls

# Check workspace status
taco unit status my-app-production
```

### Permission Issues
```bash
# Check RBAC permissions (if enabled)
taco rbac me --server http://localhost:8080

# Test permissions
taco rbac test unit:read my-app-production
```

## Migration from Terraform Cloud

To migrate existing workspaces from Terraform Cloud:

1. Export state from Terraform Cloud
2. Update cloud block to point to OpenTaco
3. Run `terraform init -reconfigure`
4. Import state: `terraform state push terraform.tfstate`
5. Verify: `terraform plan` (should show no changes)

## Advanced Configuration

### Custom Client ID
Set a custom OAuth2 client ID:
```bash
OPENTACO_AUTH_CLIENT_ID=my-terraform-app ./statesman
```

### RBAC Permissions
Grant workspace access to users:
```bash
taco rbac users assign user@example.com workspace-admin my-app-production
```

### Environment Variables
Configure Terraform behavior:
```bash
export TF_CLOUD_HOSTNAME=localhost:8080
export TF_WORKSPACE=my-app-production
terraform init
```
