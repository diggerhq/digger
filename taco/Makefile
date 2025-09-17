.PHONY: all build test lint clean svc cli prov help docker-build-svc docker-run-svc docker-clean release-build

# Default goal
.DEFAULT_GOAL := help

# Version variables
VERSION ?= $(shell cat cmd/statesman/version.txt 2>/dev/null || echo "0.0.0")
CLI_VERSION ?= $(shell cat cmd/taco/version.txt 2>/dev/null || echo "0.0.0")
COMMIT_SHA ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")

# Build all components
all: build ## Build all components

build: build-svc build-cli build-prov ## Build service, CLI, and provider

build-svc: ## Build the OpenTaco service
	@echo "Building OpenTaco service..."
	cd cmd/statesman && GOWORK=off go build -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT_SHA)'" -o ../../statesman .

build-cli: ## Build the taco CLI
	@echo "Building taco CLI..."
	cd cmd/taco && GOWORK=off go build -ldflags="-X 'main.Version=$(CLI_VERSION)' -X 'main.Commit=$(COMMIT_SHA)'" -o ../../taco .

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

# Release build targets
release-build: release-build-cli release-build-svc ## Build release binaries for all platforms

release-build-cli: ## Build CLI release binaries for all platforms
	@echo "Building CLI release binaries..."
	@mkdir -p dist/cli
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then ext=".exe"; else ext=""; fi; \
			echo "Building CLI for $$os/$$arch..."; \
			(cd cmd/taco && \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build \
				-ldflags="-X 'main.Version=$(CLI_VERSION)' -X 'main.Commit=$(COMMIT_SHA)' -s -w" \
				-o ../../dist/cli/taco-$$os-$$arch$$ext .); \
		done; \
	done

release-build-svc: ## Build Statesman release binaries for all platforms
	@echo "Building Statesman release binaries..."
	@mkdir -p dist/statesman
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then ext=".exe"; else ext=""; fi; \
			echo "Building Statesman for $$os/$$arch..."; \
			(cd cmd/statesman && \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build \
				-ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT_SHA)' -s -w" \
				-o ../../dist/statesman/statesman-$$os-$$arch$$ext .); \
		done; \
	done

# Docker targets
docker-build-svc: ## Build statesman Docker image
	@echo "Building statesman Docker image..."
	docker build -f Dockerfile_statesman -t statesman:latest --build-arg COMMIT_SHA=$(COMMIT_SHA) --build-arg VERSION=$(VERSION) .

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