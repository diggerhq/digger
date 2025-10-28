# Taco Token Service Helm Chart

This Helm chart deploys the Taco Token Service, which provides token management capabilities for OpenTaco.

## Features

- Create tokens for users and organizations
- List tokens by user and organization
- Delete tokens by ID
- Verify tokens
- Support for multiple database backends (SQLite, PostgreSQL, MySQL, MSSQL)
- Persistent storage for SQLite

## Installation

```bash
helm install taco-token-service ./helm-charts/taco-token-service
```

## Configuration

The following table lists the configurable parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `tokenService.image.repository` | Image repository | `ghcr.io/diggerhq/digger/taco-token-service` |
| `tokenService.image.tag` | Image tag | `v0.1.0` |
| `tokenService.replicaCount` | Number of replicas | `1` |
| `tokenService.service.port` | Service port | `8081` |
| `tokenService.database.backend` | Database backend type | `sqlite` |
| `tokenService.persistence.enabled` | Enable persistent storage | `true` |
| `tokenService.persistence.size` | Storage size | `1Gi` |

## Database Configuration

### SQLite (Default)

```yaml
tokenService:
  database:
    backend: sqlite
    sqlite:
      path: /app/data/tokens.db
  persistence:
    enabled: true
```

### PostgreSQL

```yaml
tokenService:
  database:
    backend: postgres
    postgres:
      host: postgres.default.svc.cluster.local
      port: 5432
      user: postgres
      dbname: taco
      secretName: postgres-credentials  # Optional: use secret for password
```

### MySQL

```yaml
tokenService:
  database:
    backend: mysql
    mysql:
      host: mysql.default.svc.cluster.local
      port: 3306
      user: root
      dbname: taco
      secretName: mysql-credentials  # Optional: use secret for password
```

## API Endpoints

- `POST /api/v1/tokens` - Create a new token
- `GET /api/v1/tokens` - List tokens (with query params: user_id, org_id)
- `GET /api/v1/tokens/:id` - Get token by ID
- `DELETE /api/v1/tokens/:id` - Delete token by ID
- `POST /api/v1/tokens/verify` - Verify a token
- `GET /healthz` - Health check endpoint

