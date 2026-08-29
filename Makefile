# Makefile for Zotero AI Tagger

BINARY_NAME := zotero-tagger
MODULE := github.com/andreassag/zotero-tagger

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: all build clean test coverage lint vet run help

all: lint test build ## Run linter, tests, and build binary

build: ## Build binary for host architecture
	@echo "==> Building $(BINARY_NAME) $(VERSION)..."
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/$(BINARY_NAME)

test: ## Run unit tests with race detector
	@echo "==> Running unit tests..."
	go test -v -race ./...

coverage: ## Run unit tests with code coverage report
	@echo "==> Running tests with coverage..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt

lint: ## Run golangci-lint
	@echo "==> Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint is not installed. Running go vet instead..."; \
		go vet ./...; \
	fi

vet: ## Run go vet
	@echo "==> Running go vet..."
	go vet ./...

fmt: ## Run gofmt on all Go source files
	@echo "==> Formatting Go files..."
	gofmt -s -w .

clean: ## Remove build artifacts and temporary files
	@echo "==> Cleaning..."
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe coverage.txt

docker-build: ## Build Docker container image
	@echo "==> Building Docker image..."
	docker build -t $(BINARY_NAME):latest -f docker/Dockerfile .

help: ## Display available make targets
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
