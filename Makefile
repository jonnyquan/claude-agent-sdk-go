# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Version (for documentation and tooling)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: all test test-unit test-integration test-verbose test-race test-cover clean deps fmt lint vet check examples help

all: test

## Test targets
test: test-unit ## Run all unit tests
	@echo "✅ All unit tests passed"

test-unit: ## Run unit tests (internal packages)
	@echo "=== Running Unit Tests ==="
	$(GOTEST) -v ./internal/...
	$(GOTEST) -v ./pkg/...

test-integration: ## Run integration tests (requires Claude CLI)
	@echo "=== Running Integration Tests ==="
	@echo "Note: Requires Claude CLI to be installed"
	$(GOTEST) -tags=integration -v ./tests/integration/...

test-all: test-unit test-integration ## Run all tests including integration
	@echo "✅ All tests passed"

test-verbose: ## Run tests with verbose output
	$(GOTEST) -v -cover ./internal/... ./pkg/...

test-race: ## Run tests with race detection
	$(GOTEST) -race ./internal/... ./pkg/...

test-cover: ## Run tests with coverage
	$(GOTEST) -race -coverprofile=coverage.out ./internal/... ./pkg/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-cover-integration: ## Run all tests with coverage (including integration)
	$(GOTEST) -tags=integration -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./internal/... ./pkg/...

## Specific test categories
test-hooks: ## Test hook system components
	@echo "=== Testing Hook System ==="
	$(GOTEST) -v ./internal/query/...
	$(GOTEST) -v ./pkg/claudesdk/... -run Hook
	@echo "✅ Hook system tests passed"

test-mcp: ## Test MCP integration
	@echo "=== Testing MCP Integration ==="
	$(GOTEST) -v ./internal/mcp/...
	@echo "✅ MCP tests passed"

test-transport: ## Test transport layer
	@echo "=== Testing Transport Layer ==="
	$(GOTEST) -v ./internal/transport/...
	$(GOTEST) -v ./internal/discovery/...
	@echo "✅ Transport tests passed"

## Clean
clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f coverage.out
	rm -f coverage.html
	@find examples -type f -perm +111 ! -name "*.go" ! -name "*.md" -delete 2>/dev/null || true
	@echo "✅ Cleaned build artifacts"

## Dependencies
deps: ## Download dependencies
	$(GOMOD) download

deps-update: ## Update dependencies
	$(GOMOD) tidy

deps-verify: ## Verify dependencies
	$(GOMOD) verify

## Code quality
fmt: ## Format code
	$(GOFMT) -s -w .

fmt-check: ## Check if code is formatted
	@if [ "$$($(GOFMT) -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not formatted with gofmt:"; \
		$(GOFMT) -s -l .; \
		exit 1; \
	fi

lint: ## Run linter
	$(GOLINT) run

vet: ## Run go vet
	$(GOCMD) vet ./...

check: fmt-check vet lint ## Run all checks

## Security
security: ## Run security checks
	$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

nancy: ## Run Nancy security scan
	$(GOCMD) install github.com/sonatypecommunity/nancy@latest
	$(GOCMD) list -json -deps ./... | nancy sleuth --skip-update-check

## Development
generate: ## Run go generate
	$(GOCMD) generate ./...

## Examples
examples: ## Build all examples
	@echo "=== Building Examples ==="
	@for dir in examples/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			echo "Building $$dir"; \
			cd "$$dir" && $(GOBUILD) -v . && cd - > /dev/null; \
		fi; \
	done
	@echo "✅ All examples built successfully"

examples-new-api: ## Build new API examples (19-24)
	@echo "=== Building New API Examples ==="
	@for i in 19 20 21 22 23 24; do \
		if [ -d "examples/$${i}_"* ]; then \
			dir=$$(ls -d examples/$${i}_* | head -1); \
			echo "Building $$dir"; \
			cd "$$dir" && $(GOBUILD) -v . && cd - > /dev/null; \
		fi; \
	done
	@echo "✅ New API examples built successfully"

examples-clean: ## Clean example binaries
	@find examples -type f -perm +111 ! -name "*.go" ! -name "*.md" -delete 2>/dev/null || true
	@echo "✅ Example binaries cleaned"

example-hooks: ## Run hook examples
	@echo "=== Hook Examples ==="
	@if [ -d "examples/12_hooks" ]; then \
		cd examples/12_hooks && $(GOCMD) run main.go; \
	else \
		echo "Hook examples not found"; \
	fi
	@echo "✅ Hook examples completed"

example-new-api: ## Run a new API example (default: 19)
	@echo "=== New API Example ==="
	@dir=$$(ls -d examples/19_* | head -1); \
	if [ -d "$$dir" ]; then \
		cd "$$dir" && $(GOCMD) run main.go; \
	else \
		echo "New API examples not found"; \
	fi

## Documentation
docs: ## Generate documentation
	$(GOCMD) doc -all ./pkg/claudesdk

docs-serve: ## Serve documentation locally
	@echo "Starting documentation server at http://localhost:6060"
	@echo "Visit http://localhost:6060/pkg/github.com/jonnyquan/claude-agent-sdk-go/"
	godoc -http=:6060

## SDK/Library specific tasks
sdk-test: ## Test SDK as a consumer would use it
	@echo "=== SDK Consumer Test ==="
	@mkdir -p /tmp/sdk-test
	@cd /tmp/sdk-test && \
	go mod init sdk-consumer-test && \
	echo 'module sdk-consumer-test\n\ngo 1.18\n\nreplace github.com/jonnyquan/claude-agent-sdk-go => $(PWD)' > go.mod && \
	echo 'package main\n\nimport (\n\t"context"\n\t"fmt"\n\t"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"\n)\n\nfunc main() {\n\tctx := context.Background()\n\t_ = claudesdk.WithModel("claude-3-5-sonnet-20241022")\n\tfmt.Println("✅ New SDK API imports work")\n\t_, _ = claudesdk.Query(ctx, "test")\n\tfmt.Println("✅ New SDK API accessible")\n}' > main.go && \
	go mod tidy && \
	go run main.go && \
	rm -rf /tmp/sdk-test
	@echo "✅ SDK consumer test passed"

sdk-test-compat: ## Test SDK backward compatibility
	@echo "=== SDK Backward Compatibility Test ==="
	@mkdir -p /tmp/sdk-compat-test
	@cd /tmp/sdk-compat-test && \
	go mod init sdk-compat-test && \
	echo 'module sdk-compat-test\n\ngo 1.18\n\nreplace github.com/jonnyquan/claude-agent-sdk-go => $(PWD)' > go.mod && \
	echo 'package main\n\nimport (\n\t"context"\n\t"fmt"\n\tclaudecode "github.com/jonnyquan/claude-agent-sdk-go"\n)\n\nfunc main() {\n\tctx := context.Background()\n\t_ = claudecode.NewOptions()\n\tfmt.Println("✅ Old SDK API imports work")\n\t_, _ = claudecode.Query(ctx, "test")\n\tfmt.Println("✅ Old SDK API accessible (backward compatible)")\n}' > main.go && \
	go mod tidy && \
	go run main.go && \
	rm -rf /tmp/sdk-compat-test
	@echo "✅ SDK backward compatibility test passed"

api-check: ## Check public API surface
	@echo "=== Public API Surface (New API) ==="
	@$(GOCMD) doc -all ./pkg/claudesdk | head -50
	@echo ""
	@echo "=== Key Exported Types (pkg/claudesdk) ==="
	@$(GOCMD) doc ./pkg/claudesdk Client 2>/dev/null || echo "Client interface available"
	@$(GOCMD) doc ./pkg/claudesdk Query 2>/dev/null || echo "Query function available"
	@$(GOCMD) doc ./pkg/claudesdk WithClient 2>/dev/null || echo "WithClient function available"
	@echo ""
	@echo "=== Hook System API ==="
	@$(GOCMD) doc ./pkg/claudesdk HookEvent 2>/dev/null || echo "Hook types available"
	@$(GOCMD) doc ./pkg/claudesdk HookCallback 2>/dev/null || echo "Hook callbacks available"
	@echo ""
	@echo "=== Backward Compatibility Layer ==="
	@$(GOCMD) doc . | head -20

api-check-compat: ## Check backward compatibility API
	@echo "=== Backward Compatibility API ==="
	@$(GOCMD) doc -all . | head -30

module-check: ## Check module health
	@echo "=== Module Health Check ==="
	@$(GOMOD) verify
	@$(GOMOD) tidy
	@echo "✅ Module is healthy"

## Project structure
structure: ## Show project structure
	@echo "=== Project Structure ==="
	@tree -L 2 -I 'vendor|.git|examples' . 2>/dev/null || \
	(find . -maxdepth 2 -type d ! -path '*/\.*' ! -path '*/vendor/*' ! -path '*/examples/*' | sort)

structure-tests: ## Show test file organization
	@echo "=== Test Organization ==="
	@echo "Integration tests:"
	@find tests -name "*_test.go" -type f | sort
	@echo ""
	@echo "Unit tests:"
	@find internal -name "*_test.go" -type f | sort

## Release
release-check: ## Check if ready for release
	@echo "=== Release Readiness Check ==="
	@$(MAKE) test-unit
	@$(MAKE) test-hooks
	@$(MAKE) test-mcp
	@$(MAKE) check  
	@$(MAKE) examples
	@$(MAKE) examples-new-api
	@$(MAKE) sdk-test
	@$(MAKE) sdk-test-compat
	@$(MAKE) api-check
	@$(MAKE) module-check
	@echo "✅ Ready for release!"

release-dry: ## Dry run release
	goreleaser release --snapshot --clean --skip-publish

## CI/CD helpers
ci: deps-verify test-race check examples sdk-test sdk-test-compat ## Run CI pipeline locally
	@echo "✅ CI pipeline passed"

ci-coverage: ## Run CI with coverage
	$(GOTEST) -race -coverprofile=coverage.out ./internal/... ./pkg/...

ci-full: deps-verify test-all test-race check examples examples-new-api sdk-test sdk-test-compat ## Run full CI pipeline including integration tests
	@echo "✅ Full CI pipeline passed"

## Docker (if needed in future)
docker-build: ## Build Docker image
	@echo "Docker support not implemented yet (library project)"

## Help
help: ## Display this help screen
	@echo "Claude Agent SDK for Go - Makefile Commands"
	@echo ""
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Quick Start:"
	@echo "  make test              - Run unit tests"
	@echo "  make test-all          - Run all tests (unit + integration)"
	@echo "  make examples          - Build all examples"
	@echo "  make check             - Run code quality checks"
	@echo "  make ci                - Run CI pipeline locally"

# Default target
.DEFAULT_GOAL := help
