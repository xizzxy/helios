.PHONY: help build test clean docker-build docker-up docker-down proto lint fmt vet security-scan dev-setup install-tools benchmark

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
BINARY_DIR=bin
DOCKER_REGISTRY=helios
VERSION?=dev

# Binary names
GATEWAY_BINARY=helios-gateway
CONTROL_BINARY=helios-control

# Build targets
build: ## Build all binaries
	@echo "Building binaries..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(GATEWAY_BINARY) ./cmd/helios-gateway
	$(GOBUILD) -o $(BINARY_DIR)/$(CONTROL_BINARY) ./cmd/helios-control
	@echo "Build complete!"

build-gateway: ## Build gateway binary only
	@echo "Building gateway..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(GATEWAY_BINARY) ./cmd/helios-gateway

build-control: ## Build control plane binary only
	@echo "Building control plane..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(CONTROL_BINARY) ./cmd/helios-control

build-linux: ## Build Linux binaries
	@echo "Building Linux binaries..."
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/$(GATEWAY_BINARY)-linux ./cmd/helios-gateway
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/$(CONTROL_BINARY)-linux ./cmd/helios-control

# Test targets
test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

test-short: ## Run short tests
	$(GOTEST) -v -short ./...

benchmark: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

coverage: test ## Generate coverage report
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code quality
lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format Go code
	$(GOFMT) -s -w .

vet: ## Run go vet
	$(GOCMD) vet ./...

security-scan: ## Run security scan with gosec
	gosec ./...

# Protobuf
proto: ## Generate protobuf code
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/*.proto
	@echo "Protobuf generation complete!"

# Dependencies
tidy: ## Tidy go modules
	$(GOMOD) tidy
	$(GOMOD) verify

download: ## Download dependencies
	$(GOMOD) download

# Docker targets
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -t $(DOCKER_REGISTRY)/helios-gateway:$(VERSION) -f deploy/Dockerfile.gateway .
	docker build -t $(DOCKER_REGISTRY)/helios-control:$(VERSION) -f deploy/Dockerfile.control .

docker-up: ## Start development environment with Docker Compose
	@echo "Starting development environment..."
	docker-compose -f deploy/docker-compose.yml up -d

docker-down: ## Stop development environment
	@echo "Stopping development environment..."
	docker-compose -f deploy/docker-compose.yml down

docker-logs: ## Show Docker Compose logs
	docker-compose -f deploy/docker-compose.yml logs -f

# Development environment
dev-setup: install-tools ## Setup development environment
	@echo "Setting up development environment..."
	$(GOMOD) tidy
	$(GOMOD) download
	@echo "Installing pre-commit hooks..."
	pre-commit install
	@echo "Development environment ready!"

install-tools: ## Install development tools
	@echo "Installing development tools..."
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Tools installed!"

# Load testing
load-test: ## Run load tests
	@echo "Running load tests..."
	./scripts/loadtest/k6-test.js

load-test-vegeta: ## Run Vegeta load tests
	@echo "Running Vegeta load tests..."
	./scripts/loadtest/vegeta-test.sh

# Configuration
config-example: ## Generate example configuration
	@echo "Generating example configuration..."
	cp configs/config.example.yaml config.yaml
	@echo "Configuration template created: config.yaml"

# Clean targets
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

clean-docker: ## Clean Docker images and containers
	docker-compose -f deploy/docker-compose.yml down --volumes --remove-orphans
	docker system prune -f

# Run targets
run-gateway: build-gateway ## Run gateway server
	@echo "Starting gateway server..."
	./$(BINARY_DIR)/$(GATEWAY_BINARY)

run-control: build-control ## Run control plane server
	@echo "Starting control plane server..."
	./$(BINARY_DIR)/$(CONTROL_BINARY)

# All-in-one targets
all: clean fmt vet lint test build ## Clean, format, vet, lint, test, and build
	@echo "All tasks completed!"

ci: fmt vet lint test security-scan ## Run CI pipeline
	@echo "CI pipeline completed!"

# Development shortcuts
dev-gateway: ## Start gateway in development mode
	@echo "Starting gateway in development mode..."
	HELIOS_LOG_LEVEL=debug HELIOS_METRICS_ENABLED=true $(GOCMD) run ./cmd/helios-gateway

dev-control: ## Start control plane in development mode
	@echo "Starting control plane in development mode..."
	HELIOS_LOG_LEVEL=debug HELIOS_METRICS_ENABLED=true $(GOCMD) run ./cmd/helios-control

# Documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	godoc -http=:6060
	@echo "Documentation server started at http://localhost:6060"

# Release targets
release: clean all docker-build ## Build release artifacts
	@echo "Release build completed!"

# Quick start
quickstart: dev-setup docker-up ## Quick start development environment
	@echo "Helios development environment is ready!"
	@echo "Gateway: http://localhost:8080"
	@echo "Control: http://localhost:8081"
	@echo "Metrics: http://localhost:2112"
	@echo "Redis: localhost:6379"
	@echo "Etcd: localhost:2379"