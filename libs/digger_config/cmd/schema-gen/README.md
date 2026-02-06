# Schema Generator

Automatically generates and validates the JSON Schema for `digger.yml` configuration files.

## Overview

This tool generates [`digger-schema.json`](../../digger-schema.json) from the Go struct definitions in [`yaml.go`](../../yaml.go), ensuring the schema stays in sync with the actual configuration code.

## Usage

### Generate Schema

```bash
cd libs/digger_config
make schema-generate
```

This creates/updates `digger-schema.json` with:
- All fields from `DiggerConfigYaml` and nested structs
- Type information (string, boolean, array, object)
- Enum constraints from code constants
- Field descriptions and default values
- YAML field name mapping (using `yaml:` struct tags)

### Validate Schema

```bash
make schema-validate
```

Validates that the existing schema matches the current Go struct definitions. Fails if:
- Schema is missing properties that exist in Go structs
- Critical enum values don't match code constants

### Run All Checks

```bash
make schema-check
```

Runs both validation and all schema tests. Use this in CI/CD.

## How It Works

1. **Reflection**: Uses [`github.com/invopop/jsonschema`](https://github.com/invopop/jsonschema) to reflect on Go structs
2. **YAML Tag Mapping**: Configured with `FieldNameTag: "yaml"` to use YAML field names
3. **Enhancement**: [`enhanceSchema()`](main.go:75) adds:
   - Enum values from code constants (e.g., `CommentRenderModeBasic`)
   - Field descriptions
   - Default values
4. **Validation**: Compares generated schema against existing file

## Schema ID

The schema is published at:
```
https://docs.opentaco.dev/schemas/digger.yml.json
```

## Development

### Adding New Fields

When you add fields to `DiggerConfigYaml`:

1. Add the field with proper `yaml:` tag:
   ```go
   NewField string `yaml:"new_field"`
   ```

2. Regenerate the schema:
   ```bash
   make schema-generate
   ```

3. If the field has enum values, add them in [`enhanceSchema()`](main.go:75):
   ```go
   if prop, ok := props.Get("new_field"); ok && prop != nil {
       prop.Enum = []interface{}{"value1", "value2"}
       prop.Description = "Description of new field"
   }
   ```

### Testing

Schema tests are in [`schema_test.go`](../../schema_test.go). Add tests for:
- New enum fields
- Required field validation
- Default value behavior

## Dependencies

- `github.com/invopop/jsonschema` v0.13.0 - Schema generation
- `github.com/wk8/go-ordered-map/v2` v2.1.8 - Ordered property maps (transitive)

## Related Files

- [`../../digger-schema.json`](../../digger-schema.json) - Generated schema
- [`../../yaml.go`](../../yaml.go) - Source structs
- [`../../schema_test.go`](../../schema_test.go) - Schema validation tests
- [`../../Makefile`](../../Makefile) - Build targets
