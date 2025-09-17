.PHONY: all build test lint clean svc cli prov help

# Default goal
.DEFAULT_GOAL := help

# Build all components
all: build ## Build all components

build: build-svc build-cli build-prov ## Build service, CLI, and provider

build-svc: ## Build the OpenTaco service
	@echo "Building OpenTaco service..."
	cd cmd/opentacosvc && GOWORK=off go build -o ../../opentacosvc .

build-cli: ## Build the taco CLI
	@echo "Building taco CLI..."
	cd cmd/taco && GOWORK=off go build -o ../../taco .

build-prov: ## Build the Terraform provider
	@echo "Building Terraform provider..."
	cd providers/terraform/opentaco && GOWORK=off go build -o ../../../terraform-provider-opentaco .

install: install-svc install-cli ## Install service and CLI

install-svc: ## Build the OpenTaco service
	@echo "Installing OpenTaco service..."
	cd cmd/opentacosvc && GOWORK=off go install  .

install-cli: ## Build the taco CLI
	@echo "Installing taco CLI..."
	cd cmd/taco && GOWORK=off go install .

# Run components
svc: build-svc ## Run the OpenTaco service
	@echo "Starting OpenTaco service on :8080..."
	./opentacosvc

cli: build-cli ## Build and show CLI help
	@echo "Taco CLI built. Run ./taco --help for usage"
	./taco --help

# Development
test: ## Run tests
	@echo "Running tests..."
	cd cmd/opentacosvc && go test ./...
	cd cmd/taco && go test ./...
	cd pkg/sdk && go test ./...
	cd providers/terraform/opentaco && go test ./...

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f opentacosvc taco terraform-provider-opentaco
	rm -rf .devdata/*
	find . -name "*.test" -delete
	find . -name "*.out" -delete

# Initialize modules
init: ## Initialize Go modules
	@echo "Initializing Go modules..."
	cd cmd/opentacosvc && go mod init github.com/digger/opentaco/cmd/opentacosvc && go mod tidy
	cd cmd/taco && go mod init github.com/digger/opentaco/cmd/taco && go mod tidy  
	cd pkg/sdk && go mod init github.com/digger/opentaco/pkg/sdk && go mod tidy
	cd providers/terraform/opentaco && go mod init github.com/digger/opentaco/providers/terraform/opentaco && go mod tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'