#!/bin/bash
set -e

# Determine which database backend is being used
BACKEND=${OPENTACO_BACKEND:-sqlite}

echo "Starting OpenTaco Statesman with backend: $BACKEND"

# Apply migrations based on backend type
case $BACKEND in
  postgres)
    echo "Applying PostgreSQL migrations..."
    DB_URL="postgres://${OPENTACO_POSTGRES_USER}:${OPENTACO_POSTGRES_PASSWORD}@${OPENTACO_POSTGRES_HOST}:${OPENTACO_POSTGRES_PORT}/${OPENTACO_POSTGRES_DATABASE}?sslmode=${OPENTACO_POSTGRES_SSLMODE:-disable}"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/postgres"
    ;;
  mysql)
    echo "Applying MySQL migrations..."
    DB_URL="mysql://${OPENTACO_MYSQL_USER}:${OPENTACO_MYSQL_PASSWORD}@${OPENTACO_MYSQL_HOST}:${OPENTACO_MYSQL_PORT}/${OPENTACO_MYSQL_DATABASE}"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/mysql"
    ;;
  sqlite)
    echo "Applying SQLite migrations..."
    SQLITE_PATH=${OPENTACO_SQLITE_PATH:-/app/data/taco.db}
    # Ensure directory exists
    mkdir -p "$(dirname "$SQLITE_PATH")"
    DB_URL="sqlite://$SQLITE_PATH"
    atlas migrate apply --url "$DB_URL" --dir "file:///app/migrations/sqlite"
    ;;
  *)
    echo "Unknown backend: $BACKEND"
    exit 1
    ;;
esac

echo "Migrations applied successfully. Starting statesman..."

# Start the statesman binary
exec /app/statesman "$@"

