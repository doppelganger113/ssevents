# Define the name of the binary/output file
BINARY_NAME := server
BINARY_CLIENT_NAME := client

# Setup Go tools
TOOLS_DIR := $(shell go env GOPATH)/bin
STRINGER := $(TOOLS_DIR)/stringer

# Define the Go module name (replace with your module name)
MODULE_NAME := github.com/doppelganger113/ssevents

# Define the Go version to use (optional)
GO_VERSION := 1.23

# Define the default target (run when you just type `make`)
.DEFAULT_GOAL := help

# Define the help target to display available commands
help:
	@echo "Available commands:"
	@echo "  make build       	- Build the server application"
	@echo "  make build-client  - Build the client application"
	@echo "  make run         	- Run the application"
	@echo "  make run-client	- Run the client application"
	@echo "  make test        	- Run all tests"
	@echo "  make lint        	- Run linter (golangci-lint)"
	@echo "  make fmt         	- Format the code"
	@echo "  make clean       	- Clean the build artifacts"
	@echo "  make help        	- Show this help message"

# Build the application
build: generate
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) ./examples/$(BINARY_NAME)

# Build the client application
build-client:
	@echo "Building $(BINARY_CLIENT_NAME)..."
	go build -o bin/$(BINARY_CLIENT_NAME) ./examples/$(BINARY_CLIENT_NAME)

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME) $(ARGS)

# Run the application
run-client: build-client
	@echo "Running $(BINARY_CLIENT_NAME)..."
	./bin/$(BINARY_CLIENT_NAME) $(ARGS)

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run linter (requires golangci-lint to be installed)
lint:
	@echo "Running linter..."
	go vet ./...
	golangci-lint run

# Format the code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/$(BINARY_NAME)

# Install dependencies (e.g., golangci-lint)
install-deps:
	@echo "Installing dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Ensure the correct Go version is installed
ensure-go-version:
	@if ! go version | grep -q "go$(GO_VERSION)"; then \
		echo "Error: Go version $(GO_VERSION) is required"; \
		exit 1; \
	fi

# Run all checks (lint, test, etc.)
check: ensure-go-version lint test

tools: $(STRINGER)

$(STRINGER):
	go install golang.org/x/tools/cmd/stringer@latest

# Execute all tools like stringer to generate code
generate: tools
	go generate ./...

# Send data to the server to omit to all connected clients
emit:
	@echo "Sending hello to the server API..."
	curl -X POST -H "Content-Type: application/json" -d '{"data": "{\"message\": \"Hello\"}"}' localhost:3000/emit

# Phony targets (targets that are not files)
.PHONY: help build build-client run run-client test lint fmt clean docker-build docker-run install-deps ensure-go-version check emit tools generate