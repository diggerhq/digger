.PHONY: build clean test schema schema-docs

# Binary name
BINARY_NAME=dgctl

# Build the dgctl binary
build:
	go build -o $(BINARY_NAME) ./main.go

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f schema.json

# Run tests
test:
	go test -v ./...

# Generate JSON Schema for digger.yml and output to stdout
schema:
	go run . schema

# Generate JSON Schema and save to docs folder
schema-docs:
	go run . schema -o ./schema.json
	@echo "Schema generated at ./schema.json"

# Install dependencies
deps:
	go mod tidy

# Run the schema command and verify it works
verify-schema:
	@echo "Generating schema..."
	@go run . schema > /dev/null && echo "Schema generation successful" || echo "Schema generation failed"
