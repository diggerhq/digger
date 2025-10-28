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

# Migration name variable (use with: make atlas-diff-all-clean name=add_new_feature)
NAME ?=

.PHONY: atlas-install atlas-init-all atlas-init-dirs atlas-hash-all atlas-diff-all-clean atlas-validate-all atlas-status-all

# Install Atlas CLI (provider not required with AutoMigrate loader)
atlas-install: ## Install Atlas CLI
	@echo "Installing Atlas CLI..."
	@go install ariga.io/atlas/cmd/atlas@latest
	@echo "✅ Atlas installed successfully"

# Initialize migration directories and hash all files
atlas-init-dirs: ## Initialize migration directories if missing
	@mkdir -p migrations/postgres migrations/mysql migrations/sqlite
	@echo "Ensuring atlas.sum files exist by hashing..."
	@atlas migrate hash --dir file://migrations/postgres 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/mysql 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/sqlite 2>/dev/null || true
	@echo "✅ Migration directories initialized (if needed)"

atlas-hash-all: ## Recompute atlas.sum for all dirs
	@atlas migrate hash --dir file://migrations/postgres 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/mysql 2>/dev/null || true
	@atlas migrate hash --dir file://migrations/sqlite 2>/dev/null || true

atlas-init-all: ## Initialize dirs and hash all
	@$(MAKE) atlas-init-dirs
	@$(MAKE) atlas-hash-all
	@echo "✅ Atlas directories initialized and hashed"

# Generate migrations using env dev DBs (Atlas replays migrations internally)
atlas-diff-all: ## Generate migrations using env dev DBs only (no overrides)
	@echo "Generating migrations using env dev DBs (no URL overrides)..."
	@$(MAKE) atlas-init-all
	@echo "\n📊 PostgreSQL (env)..." && if [ -n "$(strip $(NAME))" ]; then atlas migrate diff "$(NAME)" --env postgres; else atlas migrate diff --env postgres; fi
	@echo "\n📊 MySQL (env)..." && if [ -n "$(strip $(NAME))" ]; then atlas migrate diff "$(NAME)" --env mysql; else atlas migrate diff --env mysql; fi
	@echo "\n📊 SQLite (env)..." && if [ -n "$(strip $(NAME))" ]; then atlas migrate diff "$(NAME)" --env sqlite; else atlas migrate diff --env sqlite; fi
	@echo "\n✅ Env-driven diffs generated successfully"

atlas-validate-all: ## Validate migrations in all envs
	@echo "Validating migrations via envs..."
	@echo "PostgreSQL..." && atlas migrate validate --env postgres
	@echo "MySQL..." && atlas migrate validate --env mysql
	@echo "SQLite..." && atlas migrate validate --env sqlite
	@echo "✅ Validation successful"

atlas-status-all: ## Show migration status for all envs
	@echo "Migration status (envs)..."
	@echo "PostgreSQL:" && atlas migrate status --env postgres || true
	@echo "MySQL:" && atlas migrate status --env mysql || true
	@echo "SQLite:" && atlas migrate status --env sqlite || true
	@echo "✅ Status fetched"

