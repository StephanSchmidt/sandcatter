.PHONY: all build test test-verbose clean install lint sec secrets check release help

# Binary name
BINARY=sandcatter

# Version info
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

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
	@$(GOBUILD) -ldflags "$(LDFLAGS)" $(BUILD_FLAGS) -o $(BINARY) .
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

## lint: Run static analysis
lint:
	go vet ./...
	go tool staticcheck ./...
	go tool golangci-lint run ./...
	go tool nilaway ./...
	go tool gocyclo -over 15 .

## sec: Run security checks
sec:
	go tool gosec ./...
	go tool govulncheck ./...

## secrets: Scan for leaked secrets
secrets:
	go tool gitleaks git -v

## integration-test: Run integration tests with fresh sandcat
integration-test: build
	@echo "Running integration tests..."
	@./test.sh
	@echo "✓ Integration tests passed"

## release: Tag and release with goreleaser
release:
	@test -z "$$(git status --porcelain)" || (echo "error: working tree is dirty" && exit 1)
	@echo "Tagging $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	go tool goreleaser release --clean

## check: Run all checks (lint, sec, secrets)
check: lint sec secrets

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^##/ { sub(/^## */, ""); split($$0, a, ": *"); printf "  %-20s %s\n", a[1], a[2] }' $(MAKEFILE_LIST)

# Default target
.DEFAULT_GOAL := help
