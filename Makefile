# Makefile for Go module

# Variables
PKG := ./...
GOFILES := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Default target
.PHONY: all
all: test

# Run tests with race detector and verbose output
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v -cover -race $(PKG)

# Format the code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt $(PKG)

# Lint the code using golangci-lint
.PHONY: lint
lint:
	@echo "Linting code..."
	@golangci-lint run

# Clean build artifacts and coverage reports
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@go clean

# Display help
.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Run tests (default)"
	@echo "  test          Run tests with race detector and verbose output"
	@echo "  fmt           Format the code"
	@echo "  lint          Lint the code using golangci-lint"
	@echo "  clean         Remove build artifacts and coverage reports"
	@echo "  help          Display this help message"
