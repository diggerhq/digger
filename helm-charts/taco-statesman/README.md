# Taco Statesman Helm Chart

A minimalist Helm chart for deploying the Taco Statesman service.

## Quick Start

```bash
# Install with default settings (memory storage)
helm install taco-statesman ./helm-charts/taco-statesman

# Install with S3 storage
helm install taco-statesman ./helm-charts/taco-statesman \
  --set taco.storage.type=s3 \
  --set taco.storage.s3.bucket=my-bucket \
  --set taco.storage.s3.region=us-east-1

# Disable authentication (development)
helm install taco-statesman ./helm-charts/taco-statesman \
  --set taco.auth.disable=true
```

## Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `taco.image.repository` | Image repository | `ghcr.io/diggerhq/digger/taco-statesman` |
| `taco.image.tag` | Image tag | `v0.1.0` |
| `taco.replicaCount` | Number of replicas | `1` |
| `taco.service.port` | Service port | `8080` |
| `taco.storage.type` | Storage type (`memory` or `s3`) | `memory` |
| `taco.auth.disable` | Disable authentication | `false` |

## Storage

- **Memory**: Default, no configuration needed
- **S3**: Set `taco.storage.type=s3` and provide S3 credentials
