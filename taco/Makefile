.PHONY: all build test lint clean svc cli prov help docker-build-svc docker-run-svc docker-clean

# Default goal
.DEFAULT_GOAL := help

# Build all components
all: build ## Build all components

build: build-svc build-cli build-prov ## Build service, CLI, and provider

build-svc: ## Build the OpenTaco service
	@echo "Building OpenTaco service..."
	cd cmd/statesman && GOWORK=off go build -o ../../statesman .

build-cli: ## Build the taco CLI
	@echo "Building taco CLI..."
	cd cmd/taco && GOWORK=off go build -o ../../taco .

build-prov: ## Build the Terraform provider
	@echo "Building Terraform provider..."
	cd providers/terraform/opentaco && GOWORK=off go build -o ../../../terraform-provider-opentaco .

install: install-svc install-cli ## Install service and CLI

install-svc: ## Build the OpenTaco service
	@echo "Installing OpenTaco service..."
	cd cmd/statesman && GOWORK=off go install  .

install-cli: ## Build the taco CLI
	@echo "Installing taco CLI..."
	cd cmd/taco && GOWORK=off go install .

# Run components
svc: build-svc ## Run the OpenTaco service
	@echo "Starting OpenTaco service on :8080..."
	./statesman

cli: build-cli ## Build and show CLI help
	@echo "Taco CLI built. Run ./taco --help for usage"
	./taco --help

# Development
test: ## Run tests
	@echo "Running tests..."
	cd cmd/statesman && go test ./...
	cd cmd/taco && go test ./...
	cd pkg/sdk && go test ./...
	cd providers/terraform/opentaco && go test ./...

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f statesman taco terraform-provider-opentaco
	rm -rf .devdata/*
	find . -name "*.test" -delete
	find . -name "*.out" -delete

# Initialize modules
init: ## Initialize Go modules
	@echo "Initializing Go modules..."
	cd cmd/statesman && go mod init github.com/digger/opentaco/cmd/statesman && go mod tidy
	cd cmd/taco && go mod init github.com/digger/opentaco/cmd/taco && go mod tidy  
	cd pkg/sdk && go mod init github.com/digger/opentaco/pkg/sdk && go mod tidy
	cd providers/terraform/opentaco && go mod init github.com/digger/opentaco/providers/terraform/opentaco && go mod tidy

# Docker targets
docker-build-svc: ## Build statesman Docker image
	@echo "Building statesman Docker image..."
	docker build -f Dockerfile_statesman -t statesman:latest --build-arg COMMIT_SHA=$(shell git rev-parse HEAD) .

docker-run-svc: ## Run statesman in Docker container
	@echo "Running statesman in Docker container..."
	docker run -p 8080:8080 \
		-e OPENTACO_STORAGE=memory \
		-e OPENTACO_AUTH_DISABLE=true \
		--name statesman-container \
		statesman:latest

docker-clean: ## Clean Docker images and containers
	@echo "Cleaning Docker artifacts..."
	docker stop statesman-container 2>/dev/null || true
	docker rm statesman-container 2>/dev/null || true
	docker rmi statesman:latest 2>/dev/null || true

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ============================================================================
# Atlas Migration Targets
# ============================================================================

# Migration name variable (use with: make atlas-diff-all name=add_new_feature)
NAME ?= $(shell date +%Y%m%d%H%M%S)

.PHONY: atlas-install atlas-diff-all atlas-apply-all atlas-lint-all atlas-hash-all

# Install Atlas CLI and GORM provider
atlas-install: ## Install Atlas CLI and GORM provider
	@echo "Installing Atlas CLI and GORM provider..."
	@go install ariga.io/atlas/cmd/atlas@latest
	@go install ariga.io/atlas-provider-gorm@latest
	@echo "✅ Atlas tools installed successfully"

# Generate migrations for all database dialects (Postgres, MySQL, SQLite)
atlas-diff-all: ## Generate migrations for all databases (use: make atlas-diff-all name=my_change)
	@echo "Generating migrations for all databases..."
	@if [ -z "$(filter-out $(NAME),$(shell date +%Y%m%d%H%M%S))" ]; then \
		echo "⚠️  Using auto-generated name: $(NAME)"; \
		echo "💡 Tip: Use 'make atlas-diff-all name=descriptive_name' for better naming"; \
	fi
	@mkdir -p migrations/postgres migrations/mysql migrations/sqlite
	@echo "\n🔐 Generating checksums for existing migrations..."
	@$(MAKE) atlas-hash-all
	@echo "\n📊 PostgreSQL..." && atlas migrate diff $(NAME) --env postgres
	@echo "\n📊 MySQL..." && atlas migrate diff $(NAME) --env mysql
	@echo "\n📊 SQLite..." && atlas migrate diff $(NAME) --env sqlite
	@echo "\n✅ All migrations generated successfully!"


# Validate and lint all migrations
atlas-lint-all: ## Validate and lint all migration files
	@echo "Validating and linting migrations..."
	@echo "PostgreSQL..." && atlas migrate validate --env postgres
	@echo "MySQL..." && atlas migrate validate --env mysql
	@echo "SQLite..." && atlas migrate validate --env sqlite
	@echo "✅ All migrations validated successfully"

# Update migration directory hashes (called automatically by atlas-diff-all)
atlas-hash-all:
	@atlas migrate hash --dir file://migrations/postgres 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/mysql 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/sqlite 2>/dev/null || true