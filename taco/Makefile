.PHONY: all build test lint clean svc cli prov help docker-build-svc docker-run-svc docker-clean

# Default goal
.DEFAULT_GOAL := help

# Build all components
all: build ## Build all components

build: build-svc build-cli build-prov build-token ## Build service, CLI, provider, and token service

build-svc: ## Build the OpenTaco service
	@echo "Building OpenTaco service..."
	cd cmd/statesman && GOWORK=off go build -o ../../statesman .

build-cli: ## Build the taco CLI
	@echo "Building taco CLI..."
	cd cmd/taco && GOWORK=off go build -o ../../taco .

build-prov: ## Build the Terraform provider
	@echo "Building Terraform provider..."
	cd providers/terraform/opentaco && GOWORK=off go build -o ../../../terraform-provider-opentaco .

build-token: ## Build the Token Service
	@echo "Building Token Service..."
	cd cmd/token_service && GOWORK=off go build -o ../../token_service .

install: install-svc install-cli install-token ## Install service, CLI, and token service

install-svc: ## Build the OpenTaco service
	@echo "Installing OpenTaco service..."
	cd cmd/statesman && GOWORK=off go install  .

install-cli: ## Build the taco CLI
	@echo "Installing taco CLI..."
	cd cmd/taco && GOWORK=off go install .

install-token: ## Install the Token Service
	@echo "Installing Token Service..."
	cd cmd/token_service && GOWORK=off go install .

# Run components
svc: build-svc ## Run the OpenTaco service
	@echo "Starting OpenTaco service on :8080..."
	./statesman

cli: build-cli ## Build and show CLI help
	@echo "Taco CLI built. Run ./taco --help for usage"
	./taco --help

token: build-token ## Run the Token Service
	@echo "Starting Token Service on :8081..."
	./token_service

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
	rm -f statesman taco terraform-provider-opentaco token_service
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

docker-build-token: ## Build token service Docker image
	@echo "Building token service Docker image..."
	docker build -f Dockerfile_token_service -t token-service:latest --build-arg COMMIT_SHA=$(shell git rev-parse HEAD) .

docker-run-token: ## Run token service in Docker container
	@echo "Running token service in Docker container..."
	docker run -p 8081:8081 \
		-e OPENTACO_QUERY_BACKEND=sqlite \
		-e OPENTACO_SQLITE_DB_PATH=/app/data/tokens.db \
		--name token-service-container \
		token-service:latest

docker-clean: ## Clean Docker images and containers
	@echo "Cleaning Docker artifacts..."
	docker stop statesman-container 2>/dev/null || true
	docker rm statesman-container 2>/dev/null || true
	docker rmi statesman:latest 2>/dev/null || true
	docker stop token-service-container 2>/dev/null || true
	docker rm token-service-container 2>/dev/null || true
	docker rmi token-service:latest 2>/dev/null || true

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'