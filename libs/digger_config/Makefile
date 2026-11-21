.PHONY: schema-generate schema-validate test-schema

# Generate JSON schema from Go structs
schema-generate:
	@echo "Generating JSON schema from Go structs..."
	@cd cmd/schema-gen && GOWORK=off go mod download && GOWORK=off go run main.go -output ../../digger-schema.json
	@echo "✓ Schema generated successfully"

# Validate existing schema matches Go structs
schema-validate:
	@echo "Validating JSON schema against Go structs..."
	@cd cmd/schema-gen && GOWORK=off go mod download && GOWORK=off go run main.go -validate -output ../../digger-schema.json
	@echo "✓ Schema validation passed"

# Run schema tests
test-schema:
	@echo "Running schema tests..."
	@go test -v -run "^Test.*" schema_test.go config.go converters.go digger_config.go utils.go validators.go yaml.go
	@echo "✓ All schema tests passed"

# Run all schema checks (for CI)
schema-check: schema-validate test-schema
	@echo "✓ All schema checks passed"
