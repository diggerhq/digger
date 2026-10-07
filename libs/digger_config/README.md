# Digger Config Library

Go library for parsing and validating Digger configuration files (`digger.yml`).

## Components

### Configuration Parsing
- **[`digger_config.go`](digger_config.go)** - Main configuration loading and parsing
- **[`yaml.go`](yaml.go)** - YAML struct definitions
- **[`converters.go`](converters.go)** - Convert YAML structs to internal types
- **[`validators.go`](validators.go)** - Configuration validation logic

### JSON Schema
- **[`digger-schema.json`](digger-schema.json)** - JSON Schema for `digger.yml`
- **[`cmd/schema-gen/`](cmd/schema-gen/)** - Schema generation tool
- **[`schema_test.go`](schema_test.go)** - Schema validation tests

See [`cmd/schema-gen/README.md`](cmd/schema-gen/README.md) for schema generation documentation.

### Terragrunt Support
- **[`terragrunt/`](terragrunt/)** - Terragrunt configuration parsing

## Usage

```go
import "github.com/diggerhq/digger/libs/digger_config"

// Load configuration
config, _, _, _, err := digger_config.LoadDiggerConfig(".", true, nil, nil)
if err != nil {
    log.Fatal(err)
}

// Validate
if err := digger_config.ValidateDiggerConfig(config); err != nil {
    log.Fatal(err)
}
```

## Development

### Running Tests

```bash
# All tests
go test ./...

# Schema tests only
make test-schema
```

### Maintaining the Schema

When modifying configuration structs:

```bash
# Regenerate schema
make schema-generate

# Validate it matches
make schema-validate

# Run all checks
make schema-check
```

See [`cmd/schema-gen/README.md`](cmd/schema-gen/README.md) for details.

## Configuration Reference

See the [digger.yml reference documentation](../../docs/ce/reference/digger.yml.mdx) for user-facing configuration documentation.
