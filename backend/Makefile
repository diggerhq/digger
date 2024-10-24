# Include only if exist
-include .env
export

start:
	go run main.go

# Add or modify a build target
build:
	go build -ldflags "-X main.version=$(shell git describe --tags) -X main.commitID=#$(shell git rev-parse --short HEAD)" -o digger cli/cmd/digger/main.go
