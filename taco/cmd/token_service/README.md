# Token Service

A microservice for managing API tokens in the OpenTaco system.

## Overview

The Token Service provides a REST API for creating, listing, verifying, and deleting tokens for users and organizations. It supports multiple database backends including SQLite, PostgreSQL, MySQL, and MSSQL.

## Features

- **Create tokens**: Generate secure tokens for users and organizations
- **List tokens**: Query tokens by user ID and/or organization ID
- **Verify tokens**: Validate token authenticity and status
- **Delete tokens**: Remove tokens by ID
- **Multiple DB backends**: SQLite (default), PostgreSQL, MySQL, MSSQL
- **Auto-migration**: Database tables are created automatically
- **Health checks**: Built-in health endpoint

## Building

```bash
# From the taco directory
make build-token

# Or build directly
cd cmd/token_service
go build -o token_service .
```

## Running Locally

```bash
# Using Make
make token

# Or run directly
./token_service --port 8081
```

## Configuration

The service is configured via environment variables with the `OPENTACO_` prefix:

### General Configuration

- `OPENTACO_PORT`: Service port (default: 8081)
- `OPENTACO_QUERY_BACKEND`: Database backend type: `sqlite`, `postgres`, `mysql`, `mssql` (default: sqlite)

### SQLite Configuration

- `OPENTACO_SQLITE_DB_PATH`: Path to SQLite database file (default: ./data/taco.db)

### PostgreSQL Configuration

- `OPENTACO_POSTGRES_HOST`: PostgreSQL host (default: localhost)
- `OPENTACO_POSTGRES_PORT`: PostgreSQL port (default: 5432)
- `OPENTACO_POSTGRES_USER`: PostgreSQL user (default: postgres)
- `OPENTACO_POSTGRES_PASSWORD`: PostgreSQL password
- `OPENTACO_POSTGRES_DBNAME`: PostgreSQL database name (default: taco)
- `OPENTACO_POSTGRES_SSLMODE`: SSL mode (default: disable)

### MySQL Configuration

- `OPENTACO_MYSQL_HOST`: MySQL host (default: localhost)
- `OPENTACO_MYSQL_PORT`: MySQL port (default: 3306)
- `OPENTACO_MYSQL_USER`: MySQL user (default: root)
- `OPENTACO_MYSQL_PASSWORD`: MySQL password
- `OPENTACO_MYSQL_DBNAME`: MySQL database name (default: taco)

### MSSQL Configuration

- `OPENTACO_MSSQL_HOST`: MSSQL host (default: localhost)
- `OPENTACO_MSSQL_PORT`: MSSQL port (default: 1433)
- `OPENTACO_MSSQL_USER`: MSSQL user
- `OPENTACO_MSSQL_PASSWORD`: MSSQL password
- `OPENTACO_MSSQL_DBNAME`: MSSQL database name (default: taco)

## API Endpoints

### Health Check

```bash
GET /healthz
GET /health
```

### Create Token

```bash
POST /api/v1/tokens
Content-Type: application/json

{
  "user_id": "user-123",
  "org_id": "org-456",
  "name": "My API Token",
  "expires_in": "720h"  // Optional: duration string (e.g., "24h", "7d")
}
```

Response:
```json
{
  "id": "token-uuid",
  "user_id": "user-123",
  "org_id": "org-456",
  "token": "otc_tok_abcd1234...",
  "name": "My API Token",
  "status": "active",
  "created_at": "2025-10-28T12:00:00Z",
  "updated_at": "2025-10-28T12:00:00Z",
  "expires_at": "2025-11-28T12:00:00Z"
}
```

### List Tokens

```bash
GET /api/v1/tokens?user_id=user-123&org_id=org-456
```

Query parameters:
- `user_id` (optional): Filter by user ID
- `org_id` (optional): Filter by organization ID

### Get Token by ID

```bash
GET /api/v1/tokens/:id
```

### Delete Token

```bash
DELETE /api/v1/tokens/:id
```

### Verify Token

```bash
POST /api/v1/tokens/verify
Content-Type: application/json

{
  "token": "otc_tok_abcd1234...",
  "user_id": "user-123",  // Optional
  "org_id": "org-456"     // Optional
}
```

Response:
```json
{
  "valid": true,
  "token": {
    "id": "token-uuid",
    "user_id": "user-123",
    "org_id": "org-456",
    "status": "active",
    ...
  }
}
```

## Docker

Build and run using Docker:

```bash
# Build
make docker-build-token

# Run
make docker-run-token

# Or manually
docker build -f Dockerfile_token_service -t token-service:latest .
docker run -p 8081:8081 \
  -e OPENTACO_QUERY_BACKEND=sqlite \
  -e OPENTACO_SQLITE_DB_PATH=/app/data/tokens.db \
  token-service:latest
```

## Kubernetes/Helm

Deploy using Helm:

```bash
# Install with default values (SQLite)
helm install token-service ./helm-charts/taco-token-service

# Install with PostgreSQL
helm install token-service ./helm-charts/taco-token-service \
  --set tokenService.database.backend=postgres \
  --set tokenService.database.postgres.host=postgres.default.svc.cluster.local \
  --set tokenService.database.postgres.secretName=postgres-credentials

# Uninstall
helm uninstall token-service
```

See [helm-charts/taco-token-service/README.md](../../helm-charts/taco-token-service/README.md) for detailed Helm configuration options.

## Development

### Running Tests

```bash
cd cmd/token_service
go test ./...
```

### Code Structure

- `main.go`: Service entry point and initialization
- `../../internal/token_service/`:
  - `repository.go`: Database operations for tokens
  - `handler.go`: HTTP request handlers
  - `routes.go`: API route registration
- `../../internal/query/types/models.go`: Token database model

## Security Considerations

- Tokens are generated using cryptographically secure random bytes
- Token values are prefixed with `otc_tok_` for easy identification
- Tokens can have expiration times
- Inactive/expired tokens are rejected during verification
- Database credentials should be stored in Kubernetes secrets in production

## License

See [LICENSE](../../LICENSE) file.

