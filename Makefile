# Define the name of the binary/output file
BINARY_NAME := sse_server

# Define the Go module name (replace with your module name)
MODULE_NAME := github.com/doppelganger113/sse-server

# Define the Go version to use (optional)
GO_VERSION := 1.23

# Define the default target (run when you just type `make`)
.DEFAULT_GOAL := help

# Define the help target to display available commands
help:
	@echo "Available commands:"
	@echo "  make build       - Build the application"
	@echo "  make run         - Run the application"
	@echo "  make test        - Run all tests"
	@echo "  make lint        - Run linter (golangci-lint)"
	@echo "  make fmt         - Format the code"
	@echo "  make clean       - Clean the build artifacts"
	@echo "  make help        - Show this help message"

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME)

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

# Phony targets (targets that are not files)
.PHONY: help build run test lint fmt clean docker-build docker-run install-deps ensure-go-version check