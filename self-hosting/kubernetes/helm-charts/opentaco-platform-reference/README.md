# OpenTaco Platform Reference Chart

This chart is a reference implementation to get to a working OpenTaco setup quickly.

It is not intended as a production blueprint. Teams should use their own platform approach for ingress, database lifecycle/operations, and object storage.

It installs:
- Traefik ingress controller
- CloudNativePG operator
- A single shared CNPG cluster with three databases and app credentials for orchestrator, statesman, and token-service
- MinIO (StatefulSet) for statesman object storage
- Bucket init job (creates `opentaco` bucket by default)
- Statesman object storage secret (`statesman-object-storage` by default)

CNPG note:
- CNPG can auto-generate the bootstrap app secret (`<cluster>-app`) when no bootstrap secret is provided.
- This chart creates explicit per-service app secrets so the `opentaco` subcharts can reference stable, service-specific credentials.
- Secrets include structured postgres keys (`host`, `port`, `database`, `username`, `password`, `sslmode`) and are intended to be consumed via each service chart's `database.existingSecret` + `database.secretKeys` settings.

MinIO defaults:
- Service: `minio.opentaco.svc.cluster.local:9000`
- Console: `minio.opentaco.svc.cluster.local:9001`
- Bucket: `opentaco`

For statesman S3 backend, configure OpenTaco with:
- `OPENTACO_STORAGE=s3`
- `taco-statesman.taco.storage.s3.secretRef.name=statesman-object-storage`

Install `opentaco` separately after this chart. This chart now owns the CNPG `Cluster` resource and app database credentials.

Use this chart for demos and rapid validation. For production, consume the `opentaco` chart directly and manage ingress, database management, and object storage with your own standards and tooling.
