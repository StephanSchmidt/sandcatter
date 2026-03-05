.PHONY: all build test test-verbose clean install help

# Binary name
BINARY=sandcutter

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get
GOCLEAN=$(GOCMD) clean

# Build flags
BUILD_FLAGS=-v

all: test build

## build: Build the binary
build:
	@echo "Building $(BINARY)..."
	@$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY) .
	@echo "✓ Build complete: ./$(BINARY)"

## test: Run all tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...
	@echo "✓ All tests passed"

## test-verbose: Run tests with verbose output
test-verbose:
	@$(GOTEST) -v -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

## test-short: Run tests without integration tests
test-short:
	@$(GOTEST) -short ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -f $(BINARY)
	@rm -f coverage.out coverage.html
	@rm -rf test-sandcat
	@echo "✓ Clean complete"

## install: Install the binary to $GOPATH/bin
install: build
	@echo "Installing $(BINARY)..."
	@cp $(BINARY) $(GOPATH)/bin/
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY)"

## deps: Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "✓ Dependencies updated"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...
	@echo "✓ Code formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@$(GOCMD) vet ./...
	@echo "✓ Vet complete"

## lint: Run static analysis (requires golangci-lint)
lint:
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
		echo "✓ Lint complete"; \
	else \
		echo "⚠ golangci-lint not installed, skipping"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## integration-test: Run integration tests with fresh sandcat
integration-test: build
	@echo "Running integration tests..."
	@./test.sh
	@echo "✓ Integration tests passed"

## check: Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "✓ All checks passed"

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

# Default target
.DEFAULT_GOAL := help
