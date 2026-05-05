# --- header.mk ---
# Project identification and help system

PROJECT_NAME := $(shell basename $(CURDIR))

.PHONY: help

# Default target (must be first)
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- ports.mk ---
# Deterministic port assignment via cksum hash

# Base port calculated from project name (range 3000-9999)
BASE_PORT := $(shell echo $(PROJECT_NAME) | cksum | awk '{print 3000 + ($$1 % 7000)}')

# Allow PORT override: make dev PORT=9999
PORT ?= $(BASE_PORT)

.PHONY: port

port: ## Show assigned port for this project
	@echo "$(PROJECT_NAME) → http://localhost:$(PORT)"

# --- go.mk ---
# Go development targets

.PHONY: install dev build test lint clean fresh

BINARY_NAME := $(PROJECT_NAME)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/CosmoLabs-org/SmokeSig/cmd.Version=$(VERSION)

install: ## Download Go module dependencies
	go mod download

dev: ## Run in development mode (uses air if available, otherwise go run)
	@if command -v air >/dev/null 2>&1; then \
		echo "Starting $(PROJECT_NAME) with air (hot reload)"; \
		air; \
	else \
		echo "Starting $(PROJECT_NAME) → http://localhost:$(PORT)"; \
		go run .; \
	fi

build: ## Build production binary with version injection
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) .

test: ## Run all Go tests
	go test -v ./...

lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

clean: ## Clean build artifacts
	rm -rf bin/

fresh: clean install ## Clean build from scratch
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) .

# --- quality.mk ---
# Quality gates: lint, type-check, test, build (sequential)

.PHONY: check lint type-check test build

check: lint type-check test build ## Run all quality checks (lint + types + tests + build)
	@echo "✅ All quality checks passed"

