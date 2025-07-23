# Digger Backend Helm Chart

This Helm chart deploys the Digger backend orchestrator for managing Terraform/OpenTofu runs in your CI/CD pipelines.

## Installation

### Install from GitHub Container Registry

```bash
# Install directly
helm install digger-backend oci://ghcr.io/diggerhq/helm-charts/digger-backend \
  --namespace digger \
  --create-namespace \
  --values values.yaml

# Or pull a specific version
helm pull oci://ghcr.io/diggerhq/helm-charts/digger-backend --version 0.1.12
```

### Installation Steps

The installation is a two-step process:

1. **Initial deployment**: Install the helm chart with basic configuration
2. **GitHub App setup**: Navigate to `https://your-digger-hostname/github/setup` to create and configure the GitHub App
3. **Update configuration**: Add the GitHub App credentials to your values and upgrade the release

## Configuration

### Basic Configuration

Create a `values.yaml` file with your configuration:

```yaml
digger:
  # Docker image
  image:
    repository: registry.digger.dev/diggerhq/digger_backend
    tag: "v0.6.106"  # Check for latest version

  # Service configuration
  service:
    type: ClusterIP
    port: 3000

  # Ingress configuration
  ingress:
    enabled: true
    host: "digger.example.com"  # Your domain
    annotations: {}  # Add your ingress controller annotations
    tls:
      secretName: "digger-tls"  # If using TLS

  # Required secrets
  secrets:
    httpBasicAuthUsername: "admin"
    httpBasicAuthPassword: "<generate-strong-password>"
    bearerAuthToken: "<generate-strong-token>"
    hostname: "digger.example.com"
    
    # GitHub App credentials (filled after setup)
    githubOrg: ""
    githubAppId: ""
    githubAppClientId: ""
    githubAppClientSecret: ""
    githubAppPrivateKey: ""
    githubWebhookSecret: ""
    
    # Database configuration
    databaseURL: ""  # Leave empty if using built-in postgres
    postgresPassword: "<generate-strong-password>"

  # Resource limits (optional)
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

### Database Options

#### Option 1: External PostgreSQL (Recommended for production)
```yaml
digger:
  secrets:
    databaseURL: "postgresql://user:password@host:5432/digger"
```

#### Option 2: Built-in PostgreSQL (Testing only)
```yaml
postgres:
  enabled: true
  secret:
    postgresPassword: "<test-password>"
```

### Using Existing Secrets

Instead of putting secrets in values.yaml, reference an existing Kubernetes secret:

```yaml
digger:
  secret:
    useExistingSecret: true
    existingSecretName: "digger-secrets"
```

## Upgrade After GitHub App Setup

After configuring the GitHub App at `/github/setup`, update your values with the app credentials and upgrade:

```bash
helm upgrade digger-backend oci://ghcr.io/diggerhq/helm-charts/digger-backend \
  --namespace digger \
  --values values.yaml
```
