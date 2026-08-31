# ==============================================================================
# vpsctl Makefile
# ==============================================================================

# Project metadata
APP_NAME := vpsctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Go configuration
GO := go
GOFLAGS := -trimpath
CGO_ENABLED := 0

# Build output
BUILD_DIR := build
BINARY := $(BUILD_DIR)/$(APP_NAME)

# Docker configuration
DOCKER_IMAGE := ghcr.io/vpsctl/vpsctl
DOCKER_TAG := $(VERSION)

# Install paths
INSTALL_DIR := /usr/local/bin
CONFIG_DIR := /etc/vpsctl

# Colors
GREEN := \033[0;32m
CYAN := \033[0;36m
YELLOW := \033[1;33m
NC := \033[0m

# ==============================================================================
# Default target
# ==============================================================================

.DEFAULT_GOAL := help

# ==============================================================================
# Build targets
# ==============================================================================

## Build the binary
build:
	@echo -e "$(GREEN)▸ Building $(APP_NAME) $(VERSION)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/vpsctl
	@echo -e "$(GREEN)  ✓ Built: $(BINARY)$(NC)"

## Build for all platforms
build-all:
	@echo -e "$(GREEN)▸ Building for all platforms...$(NC)"
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 ./cmd/vpsctl
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_linux_arm64 ./cmd/vpsctl
	@echo -e "$(GREEN)  ✓ Built for linux/amd64 and linux/arm64$(NC)"

## Run the application
run: build
	@echo -e "$(GREEN)▸ Running $(APP_NAME)...$(NC)"
	$(BINARY) server

## Run in development mode with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		echo -e "$(CYAN)▸ Starting with hot reload (air)...$(NC)"; \
		air -c .air.toml 2>/dev/null || air; \
	else \
		echo -e "$(YELLOW)  ⚠ 'air' not found. Install it: go install github.com/air-verse/air@latest$(NC)"; \
		echo -e "$(CYAN)▸ Falling back to regular run...$(NC)"; \
		$(MAKE) run; \
	fi

## Run tests
test:
	@echo -e "$(GREEN)▸ Running tests...$(NC)"
	$(GO) test -v -race -cover -coverprofile=coverage.out ./...
	@echo -e "$(GREEN)  ✓ Tests passed$(NC)"

## Run tests with short flag (skip integration tests)
test-short:
	@echo -e "$(GREEN)▸ Running short tests...$(NC)"
	$(GO) test -short -v ./...

## Run tests and generate HTML coverage report
test-cover: test
	@echo -e "$(GREEN)▸ Generating coverage report...$(NC)"
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo -e "$(GREEN)  ✓ Coverage report: coverage.html$(NC)"

## Run linter (requires golangci-lint)
lint:
	@echo -e "$(GREEN)▸ Linting...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo -e "$(YELLOW)  ⚠ golangci-lint not found. Installing...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi

## Format code
fmt:
	@echo -e "$(GREEN)▸ Formatting...$(NC)"
	$(GO) fmt ./...
	@echo -e "$(GREEN)  ✓ Code formatted$(NC)"

## Vet code
vet:
	@echo -e "$(GREEN)▸ Vetting...$(NC)"
	$(GO) vet ./...

## Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo -e "$(GREEN)✓ All checks passed$(NC)"

## Clean build artifacts
clean:
	@echo -e "$(GREEN)▸ Cleaning...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean -cache -testcache
	@echo -e "$(GREEN)  ✓ Cleaned$(NC)"

# ==============================================================================
# Docker targets
# ==============================================================================

## Build Docker image
docker-build:
	@echo -e "$(GREEN)▸ Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		.
	@echo -e "$(GREEN)  ✓ Docker image built$(NC)"

## Run Docker container
docker-run: docker-build
	@echo -e "$(GREEN)▸ Running Docker container...$(NC)"
	docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-v /var/run/docker.sock:/var/run/docker.sock:ro \
		-v vpsctl-data:/var/lib/vpsctl \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## Stop Docker container
docker-stop:
	@echo -e "$(GREEN)▸ Stopping Docker container...$(NC)"
	docker stop $(APP_NAME) 2>/dev/null || true
	docker rm $(APP_NAME) 2>/dev/null || true

## Run with docker-compose
compose-up:
	@echo -e "$(GREEN)▸ Starting with docker-compose...$(NC)"
	docker-compose up -d --build

## Stop docker-compose
compose-down:
	@echo -e "$(GREEN)▸ Stopping docker-compose...$(NC)"
	docker-compose down

## View docker-compose logs
compose-logs:
	docker-compose logs -f

# ==============================================================================
# Install/Uninstall targets
# ==============================================================================

## Install vpsctl to system
install: build
	@echo -e "$(GREEN)▸ Installing $(APP_NAME)...$(NC)"
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(APP_NAME)
	@echo -e "$(GREEN)  ✓ Installed to $(INSTALL_DIR)/$(APP_NAME)$(NC)"
	@echo -e "$(YELLOW)  Run 'sudo bash scripts/install.sh' for full setup$(NC)"

## Uninstall vpsctl from system
uninstall:
	@echo -e "$(GREEN)▸ Uninstalling $(APP_NAME)...$(NC)"
	@sudo bash scripts/install.sh --uninstall

# ==============================================================================
# Development targets
# ==============================================================================

## Generate API documentation
docs:
	@echo -e "$(GREEN)▸ Generating API docs...$(NC)"
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/vpsctl/main.go -o docs/api; \
	else \
		echo -e "$(YELLOW)  ⚠ swag not found. Install: go install github.com/swaggo/swag/cmd/swag@latest$(NC)"; \
	fi

## Update dependencies
deps:
	@echo -e "$(GREEN)▸ Updating dependencies...$(NC)"
	$(GO) mod tidy
	$(GO) mod download
	@echo -e "$(GREEN)  ✓ Dependencies updated$(NC)"

## Verify dependencies
verify:
	@echo -e "$(GREEN)▸ Verifying dependencies...$(NC)"
	$(GO) mod verify
	@echo -e "$(GREEN)  ✓ Dependencies verified$(NC)"

# ==============================================================================
# Release targets
# ==============================================================================

## Run GoReleaser (dry run)
release-snapshot:
	@echo -e "$(GREEN)▸ Running GoReleaser (snapshot)...$(NC)"
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo -e "$(YELLOW)  ⚠ goreleaser not found. Install: go install github.com/goreleaser/goreleaser@latest$(NC)"; \
	fi

## Run GoReleaser (full release)
release:
	@echo -e "$(GREEN)▸ Running GoReleaser...$(NC)"
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --clean; \
	else \
		echo -e "$(YELLOW)  ⚠ goreleaser not found. Install: go install github.com/goreleaser/goreleaser@latest$(NC)"; \
	fi

# ==============================================================================
# Help
# ==============================================================================

## Show this help message
help:
	@echo ""
	@echo -e "$(GREEN)$(APP_NAME) Makefile$(NC)"
	@echo ""
	@echo -e "$(CYAN)Usage:$(NC) make [target]"
	@echo ""
	@echo -e "$(CYAN)Build & Run:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /' | sort | while read -r line; do \
		target=$$(echo "$$line" | sed 's/^  //'); \
		grep -E "^[a-zA-Z_-]+:" $(MAKEFILE_LIST) | grep -B1 "## $$target" | head -1 | cut -d: -f1 | xargs -I{} echo -e "  $(YELLOW){}$(NC)  $$target"; \
	done 2>/dev/null || true
	@echo ""
	@echo -e "$(CYAN)Development:$(NC)"
	@echo "  $(YELLOW)make dev$(NC)             Run with hot reload (requires air)"
	@echo "  $(YELLOW)make test$(NC)            Run tests with coverage"
	@echo "  $(YELLOW)make lint$(NC)            Run linter"
	@echo "  $(YELLOW)make check$(NC)           Run all checks"
	@echo "  $(YELLOW)make clean$(NC)           Clean build artifacts"
	@echo ""
	@echo -e "$(CYAN)Docker:$(NC)"
	@echo "  $(YELLOW)make docker-build$(NC)    Build Docker image"
	@echo "  $(YELLOW)make docker-run$(NC)      Run Docker container"
	@echo "  $(YELLOW)make compose-up$(NC)      Start with docker-compose"
	@echo "  $(YELLOW)make compose-down$(NC)    Stop docker-compose"
	@echo ""
	@echo -e "$(CYAN)Install:$(NC)"
	@echo "  $(YELLOW)make install$(NC)         Install binary to system"
	@echo "  $(YELLOW)make uninstall$(NC)       Uninstall from system"
	@echo ""

.PHONY: build build-all run dev test test-short test-cover lint fmt vet check clean \
        docker-build docker-run docker-stop compose-up compose-down compose-logs \
        install uninstall docs deps verify release-snapshot release help
