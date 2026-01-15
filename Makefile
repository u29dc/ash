.PHONY: all build test lint fmt check clean install dev

# Build variables
BINARY := ash
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Go tools
GOFUMPT := gofumpt
GOLANGCI := golangci-lint

all: check build

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/ash

build-release:
	CGO_ENABLED=0 go build $(LDFLAGS) -trimpath -o bin/$(BINARY) ./cmd/ash

test:
	go test -race -coverprofile=coverage.out ./...

test-short:
	go test -short ./...

test-verbose:
	go test -v -race ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	$(GOFUMPT) -w .
	go fmt ./...

vet:
	go vet ./...

# Quality gate
check: fmt vet lint test
	@echo "All checks passed"

clean:
	rm -rf bin/ coverage.out

install: build
	cp bin/$(BINARY) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(BINARY)

# Development
dev:
	go run ./cmd/ash

# Download dependencies
deps:
	go mod download
	go mod tidy

# Coverage report
coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-release- Build release binary (static)"
	@echo "  test         - Run tests with race detector"
	@echo "  test-short   - Run short tests"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  check        - Run all quality checks"
	@echo "  clean        - Remove build artifacts"
	@echo "  install      - Install to /usr/local/bin"
	@echo "  dev          - Run in development mode"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  coverage     - Generate coverage report"
