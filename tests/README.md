# Tests Organization

This directory contains organized test suites for the Claude Agent SDK for Go.

## 📁 Directory Structure

```
tests/
├── integration/    # Integration tests
└── README.md       # This file

<root>/             # Backward compatibility tests (old API)
internal/*/         # Unit tests (co-located with source code)
```

## 🧪 Test Categories

### 1. Unit Tests (`internal/*/`)

Unit tests are co-located with their source code in the `internal/` directory following Go best practices:

- `internal/cli/discovery_test.go` - CLI discovery tests
- `internal/mcp/server_test.go` - MCP server implementation tests
- `internal/mcp/types_test.go` - MCP type definition tests
- `internal/parser/json_test.go` - JSON message parsing tests
- `internal/query/hook_processor_test.go` - Hook processing tests
- `internal/shared/errors_test.go` - Error type tests
- `internal/shared/message_test.go` - Message type tests
- `internal/shared/options_test.go` - Option type tests
- `internal/shared/stream_test.go` - Stream handling tests
- `internal/subprocess/transport_test.go` - Transport layer tests

**Package**: Matches the source package (e.g., `package discovery`)

**Run**:
```bash
go test ./internal/...
```

### 2. Integration Tests (`tests/integration/`)

End-to-end tests that verify the entire SDK works correctly with actual Claude CLI integration (requires build tag `integration`):

- **integration_test.go** - Core integration scenarios
- **integration_validation_test.go** - Validation and edge cases
- **integration_helpers_test.go** - Shared test utilities

**Package**: `package claudecode`

**Run**:
```bash
# Run with integration tag
go test -tags=integration ./tests/integration/...

# Or use build constraints
go test ./tests/integration/... -run TestIntegration
```

## 🚀 Running Tests

### Run all tests
```bash
go test ./...
```

### Run specific test category
```bash
# Unit tests only
go test ./internal/...

# Integration tests only  
go test -tags=integration ./tests/integration/...
```

### Run with coverage
```bash
go test -cover ./...
```

### Run with verbose output
```bash
go test -v ./...
```

### Run specific test
```bash
go test -run TestMessageParsing ./internal/shared/
go test -tags=integration -run TestIntegrationQuery ./tests/integration/
```

## 📝 Test Naming Conventions

- **Unit tests**: `Test<FunctionName>` - Test specific functions  
- **Integration tests**: `TestIntegration<Feature>` - Test end-to-end scenarios

## 🏗️ Test Organization Principles

1. **Co-location**: Unit tests live next to the code they test (`internal/`)
2. **Separation**: Integration and compatibility tests are separated into `tests/`
3. **Clarity**: Clear naming shows what each test suite covers
4. **Isolation**: Tests don't depend on each other
5. **Speed**: Unit tests are fast, integration tests may be slower

## ✅ Coverage Goals

- **Unit tests**: >80% coverage for business logic
- **Integration tests**: Cover all major user workflows

## 🔧 CI/CD Integration

Test categories are run in CI pipeline:

```yaml
- Unit tests (fast) - Always run
- Integration tests (slower) - Require Claude CLI and build tag
```

## 📚 Writing New Tests

### Adding a unit test
Place it next to the source file in `internal/`:
```
internal/mypackage/
├── mycode.go
└── mycode_test.go
```

### Adding an integration test
Add to `tests/integration/` with build tag:
```go
//go:build integration

package claudecode_test

import (
    "testing"
    "github.com/jonnyquan/claude-agent-sdk-go"
)

func TestIntegrationNewFeature(t *testing.T) {
    // End-to-end test
}
```

## 🐛 Debugging Tests

### Enable verbose logging
```bash
go test -v ./tests/integration/... 2>&1 | tee test.log
```

### Run single test with trace
```bash
go test -v -run TestSpecificFunction ./tests/compat/
```

### Check test coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 📊 Test Statistics

Run this to see test distribution:
```bash
find . -name "*_test.go" ! -path "./vendor/*" ! -path "./examples/*" -exec wc -l {} + | sort -n
```

---

**Maintained by**: Claude Agent SDK Go team
**Last updated**: 2025-01-23
