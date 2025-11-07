#!/bin/bash
set -e

# Determine which database backend is being used
BACKEND=${OPENTACO_TOKEN_QUERY_BACKEND:-sqlite}

echo "Starting OpenTaco Token Service with backend: $BACKEND"

# Generate checksums for migration directories (atlas.sum files are gitignored)
echo "Generating migration checksums..."
atlas migrate hash --dir "file:///app/migrations/postgres" 2>/dev/null || true
atlas migrate hash --dir "file:///app/migrations/mysql" 2>/dev/null || true
atlas migrate hash --dir "file:///app/migrations/sqlite" 2>/dev/null || true

# Apply migrations based on backend type
case $BACKEND in
  postgres)
    echo "Applying PostgreSQL migrations..."
    # URL-encode the password to handle special characters
    ENCODED_PASSWORD=$(printf '%s' "$OPENTACO_TOKEN_POSTGRES_PASSWORD" | jq -sRr @uri)
    DB_URL="postgres://${OPENTACO_TOKEN_POSTGRES_USER}:${ENCODED_PASSWORD}@${OPENTACO_TOKEN_POSTGRES_HOST}:${OPENTACO_TOKEN_POSTGRES_PORT}/${OPENTACO_TOKEN_POSTGRES_DBNAME}?sslmode=${OPENTACO_TOKEN_POSTGRES_SSLMODE:-disable}"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/postgres"
    ;;
  mysql)
    echo "Applying MySQL migrations..."
    DB_URL="mysql://${OPENTACO_TOKEN_MYSQL_USER}:${OPENTACO_TOKEN_MYSQL_PASSWORD}@${OPENTACO_TOKEN_MYSQL_HOST}:${OPENTACO_TOKEN_MYSQL_PORT}/${OPENTACO_TOKEN_MYSQL_DBNAME}"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/mysql"
    ;;
  sqlite)
    echo "Applying SQLite migrations..."
    SQLITE_PATH=${OPENTACO_TOKEN_SQLITE_DB_PATH:-/app/data/token_service.db}
    # Ensure directory exists
    mkdir -p "$(dirname "$SQLITE_PATH")"
    DB_URL="sqlite://$SQLITE_PATH"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/sqlite"
    ;;
  mssql)
    echo "Applying MSSQL migrations..."
    DB_URL="sqlserver://${OPENTACO_TOKEN_MSSQL_USER}:${OPENTACO_TOKEN_MSSQL_PASSWORD}@${OPENTACO_TOKEN_MSSQL_HOST}:${OPENTACO_TOKEN_MSSQL_PORT}?database=${OPENTACO_TOKEN_MSSQL_DBNAME}"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/mssql"
    ;;
  *)
    echo "Unknown backend: $BACKEND"
    exit 1
    ;;
esac

echo "Migrations applied successfully. Starting token service..."

# Start the token service binary
exec /app/token_service "$@"

