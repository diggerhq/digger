.PHONY: build push run dev test clean

# Variables
IMAGE_NAME ?= digger/sandbox-sidecar
VERSION ?= latest
REGISTRY ?= ghcr.io/diggerhq

# Build the Docker image
build:
	docker build -f Dockerfile_sidecar -t $(IMAGE_NAME):$(VERSION) .

# Tag for registry
tag:
	docker tag $(IMAGE_NAME):$(VERSION) $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Push to registry
push: tag
	docker push $(REGISTRY)/$(IMAGE_NAME):$(VERSION)

# Build and push
release: build push

# Run container locally
run:
	docker run -p 9100:9100 \
		--env-file .env \
		$(IMAGE_NAME):$(VERSION)

# Development mode (with hot reload)
dev:
	npm run dev

# Run tests
test:
	npm test

# Clean up
clean:
	rm -rf dist node_modules
	docker rmi $(IMAGE_NAME):$(VERSION) 2>/dev/null || true

# Install dependencies
install:
	npm install

# Build TypeScript
compile:
	npm run build

