# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

This subdirectory contains the official Helm charts for Digger, an open-source CI/CD tool for Terraform. Currently maintains one chart: `digger-backend`.

**Important**: This is part of the main digger monorepo. Helm charts are developed here but published to OCI registry for distribution.

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
- **Pull Request Testing** (`.github/workflows/helm-test.yml`): Runs `helm unittest` and linting on PR changes to helm-charts/
- **Release Process** (`.github/workflows/helm-release.yml`): On merge to **develop** branch (not main!), publishes to GitHub Container Registry at `oci://ghcr.io/diggerhq/helm-charts/digger-backend`
- **Installation**: Users install directly from OCI registry, not GitHub Pages
- **Important**: This repo uses `develop` as the default branch, not `main`

### Important Configuration Patterns

1. **Secret Management**: The chart supports both direct values and references to existing secrets:
   ```yaml
   # Direct values
   digger:
     secret:
       githubAppID: "12345"  # Note: uppercase ID
       githubAppKeyFile: "<base64-encoded>"  # Not githubAppPrivateKey
   
   # Or reference existing secret
   digger:
     secret:
       useExistingSecret: true
       existingSecretName: "my-existing-secret"
   ```
   
   **Critical**: Configuration uses `secret` (singular), not `secrets`. Field names are case-sensitive (e.g., `githubAppID` not `githubAppId`)

2. **Database Configuration**: PostgreSQL configuration is under `digger.postgres`:
   ```yaml
   # External database
   digger:
     postgres:
       user: "digger"
       database: "digger"
       host: "postgresql.example.com"
       password: "secure-password"
       sslmode: "require"
   
   # Test database (separate top-level key)
   postgres:
     enabled: true
     secret:
       postgresPassword: "test-password"
   ```

3. **Resource Configuration**: Supports standard Kubernetes resource patterns (limits, requests, nodeSelector, tolerations, affinity)

### Testing Strategy
- Unit tests in `digger-backend/tests/` verify template rendering
- Tests use helm-unittest framework with snapshot testing
- CI automatically runs tests on PRs and before releases

## Monorepo Integration Notes

1. **Directory Structure**: Helm charts live in `/helm-charts/` subdirectory of main digger repo, with charts directly under it (not nested in `/charts/`)

2. **Publishing Strategy**: 
   - Charts are developed in the main repo but published to GitHub Container Registry (OCI)
   - Users install from `oci://ghcr.io/diggerhq/helm-charts/digger-backend`
   - No longer using separate helm-charts repository or GitHub Pages

3. **Version Management**:
   - Chart version is in `Chart.yaml` (e.g., `version: 0.1.12`)
   - App version should match the digger backend version (e.g., `appVersion: "v0.6.106"`)
   - Default image tag in `values.yaml` should typically match appVersion

4. **Common Issues to Watch For**:
   - Configuration key names are case-sensitive (`githubAppID` not `githubAppId`)
   - Use `secret` (singular) not `secrets` in configuration
   - GitHub App private key field is `githubAppKeyFile` (base64 encoded), not `githubAppPrivateKey`
   - PostgreSQL config is under `digger.postgres`, not in a `databaseURL` field