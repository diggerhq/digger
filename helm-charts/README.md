# OpenTaco Helm Charts

Production-ready Kubernetes deployment for the OpenTaco infrastructure management platform.

## Quick Start

```bash
# 1. Configure values file (see Configuration Checklist below)
cp opentaco/values-test.yaml.example opentaco/values-test.yaml
# Edit values-test.yaml with your GCP project ID and settings

# 2. Create namespace
kubectl create namespace opentaco

# 3. Create secrets (see Secret Management below)
kubectl create secret generic ui-secrets \
  --from-env-file=.secrets/ui.env -n opentaco

kubectl create secret generic backend-secrets \
  --from-env-file=.secrets/digger-backend.env -n opentaco

kubectl create secret generic statesman-secrets \
  --from-env-file=.secrets/statesman.env -n opentaco

kubectl create secret generic drift-secrets \
  --from-env-file=.secrets/drift.env -n opentaco

# 4. Deploy
cd opentaco
helm install opentaco . -f values-test.yaml -n opentaco
```

## Architecture

The umbrella chart deploys 4 services:

- **digger-backend** (port 3000) - Terraform orchestration
- **drift** (port 3004) - Infrastructure drift detection  
- **statesman** (port 8080) - IaC state management with Cloud SQL
- **ui** (port 3030) - Web frontend

## Configuration Checklist

Before deploying, you need to configure placeholder values in your values file.

### Required Placeholders in `values-test.yaml` or `values-production.yaml`

#### 1. **Cloud SQL Configuration** (if using Cloud SQL for statesman)

```yaml
cloudSql:
  enabled: true
  instanceConnectionName: "YOUR-PROJECT-ID:YOUR-REGION:YOUR-INSTANCE"  # ❌ CHANGE THIS
  credentialsSecret: "cloudsql-credentials"
  serviceAccount: "cloudsql-sa"
```

**What to do:**
- Replace `YOUR-PROJECT-ID` with your GCP project ID (e.g., `my-prod-project`)
- Replace `YOUR-REGION` with your Cloud SQL region (e.g., `us-central1`)
- Replace `YOUR-INSTANCE` with your Cloud SQL instance name (e.g., `opentaco-postgres`)

Example: `my-prod-project:us-central1:opentaco-postgres`

#### 2. **Image Registry** (optional - defaults to public GHCR)

```yaml
global:
  imageRegistry: ghcr.io/diggerhq/digger  # ✅ Public registry (no auth needed)
  # Or use your private registry:
  # imageRegistry: us-central1-docker.pkg.dev/YOUR-PROJECT/YOUR-REPO
```

**What to do:**
- Keep default for public images (recommended)
- OR replace with your private registry path if using custom builds

#### 3. **UI Ingress Configuration** (for production with custom domain)

```yaml
taco-ui:
  ui:
    ingress:
      enabled: false  # Set to true for production
      hosts:
        - host: app.opentaco.example.com  # ❌ CHANGE THIS
          paths:
            - path: /
              pathType: Prefix
      tls:
        - secretName: opentaco-ui-tls
          hosts:
            - app.opentaco.example.com  # ❌ CHANGE THIS
```

**What to do:**
- Replace `app.opentaco.example.com` with your actual domain
- Set `enabled: true` when ready to expose publicly
- Ensure you have an Ingress Controller installed (see Ingress Setup below)

#### 4. **Service Replica Counts** (optional - defaults to 1)

```yaml
digger-backend:
  digger:
    replicaCount: 1  # Increase for high availability

taco-statesman:
  taco:
    replicaCount: 1  # Increase for high availability
```

**What to do:**
- Keep `1` for test/dev environments
- Increase to `2+` for production high availability

### Quick Validation

Before deploying, check your values file for these patterns:

```bash
# In your values-test.yaml or values-production.yaml
grep -E "YOUR-|example\.com|CHANGE THIS" opentaco/values-test.yaml
```

If this returns any results, you have placeholders that need to be filled in!

## Secret Management

### 1. Copy Example Files

```bash
cp -r secrets-example/ .secrets/
```

### 2. Fill In Values

Edit each file in `.secrets/` with your actual credentials:

```bash
.secrets/
├── digger-backend.env  # GitHub App, database, Sentry, etc.
├── drift.env           # GitHub App, database connection
├── statesman.env       # Auth0, S3, PostgreSQL (Cloud SQL)
└── ui.env             # WorkOS, backend service URLs
```

**Key values to configure:**
- GitHub App credentials (create at https://github.com/settings/apps)
- Database connection strings
- S3/storage credentials
- Authentication providers (WorkOS, Auth0)

### 3. Create Kubernetes Secrets

```bash
kubectl create secret generic ui-secrets \
  --from-env-file=.secrets/ui.env -n opentaco

kubectl create secret generic backend-secrets \
  --from-env-file=.secrets/digger-backend.env -n opentaco

kubectl create secret generic statesman-secrets \
  --from-env-file=.secrets/statesman.env -n opentaco

kubectl create secret generic drift-secrets \
  --from-env-file=.secrets/drift.env -n opentaco
```

### 4. Update Secrets

```bash
# Delete old secret
kubectl delete secret statesman-secrets -n opentaco

# Recreate with new values
kubectl create secret generic statesman-secrets \
  --from-env-file=.secrets/statesman.env -n opentaco

# Restart pods to pick up changes
kubectl delete pods -l app.kubernetes.io/name=statesman -n opentaco
```

## Cloud SQL Setup

Statesman uses Google Cloud SQL for database. Backend and Drift can use external databases (Supabase, etc.) or Cloud SQL.

### 1. Create Cloud SQL Instance

```bash
gcloud sql instances create taco-postgres \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=us-central1 \
  --database-flags=max_connections=100
```

### 2. Create Database

```bash
gcloud sql databases create taco \
  --instance=taco-postgres
```

### 3. Create Service Account

```bash
# Create service account for Cloud SQL proxy
gcloud iam service-accounts create cloudsql-sa \
  --display-name="Cloud SQL Proxy Service Account"

# Grant Cloud SQL Client role
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:cloudsql-sa@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"

# Create and download key
gcloud iam service-accounts keys create cloudsql-key.json \
  --iam-account=cloudsql-sa@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

### 4. Create Kubernetes Secret for Cloud SQL

```bash
kubectl create secret generic cloudsql-credentials \
  --from-file=credentials.json=cloudsql-key.json \
  -n opentaco
```

### 5. Configure in values-test.yaml

```yaml
statesman:
  enabled: true
  taco:
    cloudSql:
      enabled: true
      instanceConnectionName: "PROJECT_ID:REGION:INSTANCE_NAME"  # e.g., "dev-XXXXXXX:us-west2:taco-postgres"
      credentialsSecret: "cloudsql-credentials"
```

### 6. Set Database Connection in statesman.env

```bash
# Cloud SQL uses localhost via proxy sidecar
OPENTACO_POSTGRES_HOST=localhost
OPENTACO_POSTGRES_PORT=5432
OPENTACO_POSTGRES_USER=postgres
OPENTACO_POSTGRES_PASSWORD=YOUR_DB_PASSWORD
OPENTACO_POSTGRES_DBNAME=taco
OPENTACO_QUERY_BACKEND=postgres
```

The Cloud SQL proxy runs as a sidecar container, connecting to your Cloud SQL instance and exposing it on localhost:5432.

## Deployment

### Test Environment

```bash
cd opentaco
helm install opentaco . -f values-test.yaml -n opentaco
```

### Production Environment

```bash
# Review and customize production values
vim opentaco/values-production.yaml

# Deploy
helm install opentaco . -f values-production.yaml -n opentaco
```

### Verify Deployment

```bash
# Check pods
kubectl get pods -n opentaco

# Check logs
kubectl logs -f deployment/opentaco-statesman -n opentaco -c statesman

# Access UI locally
kubectl port-forward svc/opentaco-ui 3030:3030 -n opentaco
open http://localhost:3030
```

## Service Communication

Services communicate via Kubernetes DNS:

```bash
# From within the cluster:
http://opentaco-digger-backend-web:3000
http://opentaco-drift:3004
http://opentaco-statesman:8080
http://opentaco-ui:3030
```

These URLs are configured in `ui.env`:
```bash
ORCHESTRATOR_BACKEND_URL="http://opentaco-digger-backend-web:3000"
DRIFT_REPORTING_BACKEND_URL="http://opentaco-drift:3004"
STATESMAN_BACKEND_URL="http://opentaco-statesman:8080"
```

## Upgrading

```bash
# Update dependencies
cd opentaco
helm dependency update

# Upgrade deployment
helm upgrade opentaco . -f values-test.yaml -n opentaco

# Force pod recreation if needed
kubectl delete pods --all -n opentaco
```

## Troubleshooting

### Pods not starting

```bash
# Check pod status
kubectl get pods -n opentaco

# Check events
kubectl describe pod POD_NAME -n opentaco

# Check logs
kubectl logs POD_NAME -n opentaco
```

### Secret issues

```bash
# List secrets
kubectl get secrets -n opentaco

# Verify secret contents
kubectl get secret backend-secrets -n opentaco -o jsonpath='{.data}' | jq 'keys'
```

### Cloud SQL connection issues

```bash
# Check Cloud SQL proxy sidecar logs
kubectl logs POD_NAME -n opentaco -c cloud-sql-proxy

# Verify instance connection name
gcloud sql instances describe INSTANCE_NAME --format="value(connectionName)"
```

## Chart Structure

```
helm-charts/
├── opentaco/              # Umbrella chart
│   ├── Chart.yaml
│   ├── values.yaml        # Default values
│   ├── values-test.yaml   # Test environment
│   └── values-production.yaml
├── digger-backend/        # Terraform orchestration
├── digger-drift/          # Drift detection
├── taco-statesman/        # State management
├── taco-ui/              # Web frontend
└── secrets-example/       # Example secret files
```

## Required External Services

- **GitHub App** - Repository access and webhooks
- **WorkOS** - UI authentication (or configure alternative)
- **Auth0** - Statesman authentication (or configure alternative)
- **S3-compatible storage** - State and artifact storage
- **Cloud SQL or PostgreSQL** - Database

## Configuration Files

| File | Purpose |
|------|---------|
| `values.yaml` | Default configuration for all services |
| `values-test.yaml` | Minimal config for testing |
| `values-production.yaml` | Production-ready settings |
| `.secrets/*.env` | Environment-specific secrets (not committed) |

