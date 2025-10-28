# Database Migrations

Atlas-based schema migrations for PostgreSQL, MySQL, and SQLite.

## Quick Reference

```bash
# Preview changes (dry-run)
make atlas-plan name=my_change

# Generate migrations
make atlas-diff-all name=my_change

# Validate migrations
make atlas-lint-all

# Apply migrations locally
atlas migrate apply --env postgres
atlas migrate apply --env mysql  
atlas migrate apply --env sqlite
```

## Local Development

### First-time setup
```bash
# Install Atlas CLI
make atlas-install

# Ensure Docker is running (required for Postgres/MySQL dev databases)
docker info
```

### Modifying schema

1. Edit GORM models in `taco/models/models.go`
2. Preview changes: `make atlas-plan name=add_user_field`
3. Generate migrations: `make atlas-diff-all name=add_user_field`
4. Review generated SQL in `migrations/{postgres,mysql,sqlite}/`
5. Apply locally: `atlas migrate apply --env [postgres|mysql|sqlite]`
6. Commit: `git add migrations/ && git commit -m "Add user field migration"`

## Docker/Production

Migrations are **automatically applied** via `scripts/entrypoint.sh` before the app starts.


