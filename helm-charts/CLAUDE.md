# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This subdirectory contains the official Helm charts for Digger, an open-source CI/CD tool for Terraform. Currently maintains one chart: `digger-backend`.

## Common Development Commands

```bash
# Install Helm unittest plugin (required for testing)
helm plugin install https://github.com/quintush/helm-unittest

# Run unit tests for the chart
helm unittest digger-backend/

# Lint the chart
helm lint digger-backend/

# Test template rendering with specific values
helm template digger-backend ./digger-backend/ -f custom-values.yaml

# Install the chart locally
helm install digger-backend ./digger-backend/

# Upgrade a release
helm upgrade digger-backend ./digger-backend/
```

## Architecture and Structure

### Chart Organization
- `digger-backend/` - Main Helm chart directory
  - `templates/` - Kubernetes resource templates
  - `tests/` - Helm unit tests using helm-unittest framework
  - `values.yaml` - Default configuration values
  - `Chart.yaml` - Chart metadata and dependencies

### Key Templates
- `backend-deployment.yaml` - Main Digger backend deployment
- `digger-secret.yaml` - Manages GitHub App credentials and database configuration
- `postgres-statefulset.yaml` - Optional PostgreSQL for testing (enabled via `postgres.enabled`)
- `backend-ingress.yaml` - Ingress configuration (enabled by default)

### CI/CD Workflows
- **Pull Request Testing**: Automatically runs `helm unittest` on PR changes
- **Release Process**: On merge to main, runs linting, testing, and publishes to GitHub Pages

### Important Configuration Patterns

1. **Secret Management**: The chart supports both direct values and references to existing secrets:
   ```yaml
   # Direct values
   secrets:
     githubAppID: "12345"
   
   # Or reference existing secret
   existingSecret: "my-existing-secret"
   ```

2. **Database Configuration**: Can use external PostgreSQL or deploy test instance:
   ```yaml
   # External database
   postgres:
     enabled: false
   secrets:
     databaseURL: "postgresql://..."
   
   # Test database
   postgres:
     enabled: true
   ```

3. **Resource Configuration**: Supports standard Kubernetes resource patterns (limits, requests, nodeSelector, tolerations, affinity)

### Testing Strategy
- Unit tests in `digger-backend/tests/` verify template rendering
- Tests use helm-unittest framework with snapshot testing
- CI automatically runs tests on PRs and before releases