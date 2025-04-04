# Go parameters
GO           := go
GOFMT        := gofmt
GOBUILD      := $(GO) build
GOTEST       := $(GO) test
GOVET        := $(GO) vet
GOGET        := $(GO) get
GOMOD        := $(GO) mod
BINARY_NAME  := mindnest
BUILD_DIR    := bin
COVERAGE_DIR := coverage
AUTHOR       := Ahmed ElSebaei
EMAIL        := tildaslashalef@gmail.com

# Version information
VERSION      ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.1")
BUILD_TIME   := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH  := $(shell git rev-parse HEAD)

# Build flags
LDFLAGS      := -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH) -X 'main.Author=$(AUTHOR)' -X 'main.Email=$(EMAIL)'"

# Supported platforms for cross-compilation
PLATFORMS    := linux/amd64 darwin/amd64 darwin/arm64

.PHONY: all build test clean deps lint format help run install migrate

all: test build ## Run tests and build

build: ## Build the binary for current platform
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/mindnest
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

run: build ## Build and run the application
	./$(BUILD_DIR)/$(BINARY_NAME)

install: build ## Install the application to GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

test: ## Run tests
	$(GOTEST) -race -v ./...

test-short: ## Run tests in short mode
	$(GOTEST) -race -v -short ./...

coverage: ## Generate test coverage report
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report available at $(COVERAGE_DIR)/coverage.html"

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	$(GO) clean -testcache

deps: ## Download and verify dependencies
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy

lint-deps: ## Ensure linting tools are installed
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@which revive > /dev/null || (echo "Installing revive..." && go install github.com/mgechev/revive@latest)
	@echo "Linting dependencies are installed"

lint: lint-deps ## Run linters
	golangci-lint run --fix --verbose

lint-check: lint-deps ## Check linting without fixing (useful for CI)
	golangci-lint run --verbose

format: ## Format code
	$(GOFMT) -w -s .

migrate: ## Run database migrations
	$(GO) run ./cmd/mindnest migrate up

# Cross compilation
cross-build: ## Build for multiple platforms
	@mkdir -p $(BUILD_DIR)
	@echo "Building version: $(VERSION)"
	$(foreach platform,$(PLATFORMS),\
		$(eval GOOS = $(word 1,$(subst /, ,$(platform))))\
		$(eval GOARCH = $(word 2,$(subst /, ,$(platform))))\
		$(eval EXTENSION = $(if $(filter windows,$(GOOS)),.exe,))\
		$(eval OUTFILE = $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)$(EXTENSION))\
		$(eval HOST_OS = $(shell uname -s | tr '[:upper:]' '[:lower:]'))\
		$(eval HOST_OS = $(if $(filter Darwin,$(shell uname -s)),darwin,$(HOST_OS)))\
		$(eval CGO_FLAG = $(if $(filter $(HOST_OS),$(GOOS)),1,0))\
		CGO_ENABLED=$(CGO_FLAG) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(LDFLAGS) -o $(OUTFILE) ./cmd/mindnest && \
		echo "Built $(OUTFILE) with CGO_ENABLED=$(CGO_FLAG)" ; \
	)

help: ## Display this help
	@echo "Mindnest Makefile"
	@echo "================="
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Development shortcuts
.PHONY: dev quick
dev: format lint test build ## Development build with all checks
quick: build ## Quick build without tests 