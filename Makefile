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
	mkdir -p "$${ASH_HOME:-$${TOOLS_HOME:-$$HOME/.tools}/ash}"
	cp bin/$(BINARY) "$${ASH_HOME:-$${TOOLS_HOME:-$$HOME/.tools}/ash}/$(BINARY)"

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

# Coverage threshold check (fails if < 70%)
COVERAGE_THRESHOLD := 70
coverage-check: test
	@echo "Checking coverage threshold ($(COVERAGE_THRESHOLD)%)..."
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	COVERAGE_INT=$$(echo "$$COVERAGE" | cut -d. -f1); \
	if [ "$$COVERAGE_INT" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "Coverage $$COVERAGE% is below threshold $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "Coverage $$COVERAGE% meets threshold $(COVERAGE_THRESHOLD)%"; \
	fi

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-release  - Build release binary (static)"
	@echo "  test           - Run tests with race detector"
	@echo "  test-short     - Run short tests"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
	@echo "  check          - Run all quality checks"
	@echo "  clean          - Remove build artifacts"
	@echo "  install        - Install to /usr/local/bin"
	@echo "  dev            - Run in development mode"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  coverage       - Generate coverage report"
	@echo "  coverage-check - Fail if coverage < 70%"
