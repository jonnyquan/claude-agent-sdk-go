# Claude Code SDK for Go

<div align="center">
  <img src="gopher.png" alt="Go Gopher" width="200"/>
</div>

<div align="center">

[![CI](https://github.com/jonnyquan/claude-agent-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/jonnyquan/claude-agent-sdk-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jonnyquan/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/jonnyquan/claude-agent-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonnyquan/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/jonnyquan/claude-agent-sdk-go)
[![codecov](https://codecov.io/gh/severity1/claude-agent-sdk-go/branch/main/graph/badge.svg)](https://codecov.io/gh/severity1/claude-agent-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

Unofficial Go SDK for Claude Code CLI integration. Build production-ready applications that leverage Claude's advanced code understanding, secure file operations, and external tool integrations through a clean, idiomatic Go API with comprehensive error handling and automatic resource management.

**🚀 Two powerful APIs for different use cases:**
- **Query API**: One-shot operations, automation, CI/CD integration  
- **Client API**: Interactive conversations, multi-turn workflows, streaming responses
- **WithClient**: Go-idiomatic context manager for automatic resource management

![Claude Code SDK in Action](cc-sdk-go-in-action-v2.gif)

## Installation

```bash
go get github.com/jonnyquan/claude-agent-sdk-go
```

**Prerequisites:** Go 1.18+

**Note:** The Claude Code CLI can be optionally bundled with the SDK - no separate installation required! The SDK will automatically use a bundled CLI if available. For manual installation or custom versions:
- Node.js (for Claude Code)
- Claude Code 2.0.0+: `curl -fsSL https://claude.ai/install.sh | bash`
- Or specify custom path: `claudecode.WithCLIPath("/path/to/claude")`

## Key Features

**Two APIs for different needs** - Query for automation, Client for interaction
**Structured Outputs** - JSON schema validation for guaranteed response formats
**CLI Auto-Bundling** - Optional bundled Claude CLI for zero-dependency deployment
**100% Python SDK compatibility** - Same functionality, Go-native design (including Hook system)
**Enhanced Hook system** - Intercept and control tool execution with custom callbacks and timeouts
**Automatic resource management** - WithClient provides Go-idiomatic context manager pattern
**Session management** - Isolated conversation contexts with `Query()` and `QueryWithSession()`
**Built-in tool integration** - File operations, AWS, GitHub, databases, and more
**Production ready** - Comprehensive error handling, timeouts, resource cleanup, fallback models
**Security focused** - Granular tool permissions, access controls, and runtime hooks
**Context-aware** - Maintain conversation state across multiple interactions  

## Usage

### Query API - One-Shot Operations
Best for automation, scripting, and tasks with clear completion criteria:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
    fmt.Println("Claude Code SDK - Query API Example")
    fmt.Println("Asking: What is 2+2?")

    ctx := context.Background()

    // Create and execute query
    iterator, err := claudecode.Query(ctx, "What is 2+2?")
    if err != nil {
        log.Fatalf("Query failed: %v", err)
    }
    defer iterator.Close()

    fmt.Println("\nResponse:")

    // Iterate through messages
    for {
        message, err := iterator.Next(ctx)
        if err != nil {
            if errors.Is(err, claudecode.ErrNoMoreMessages) {
                break
            }
            log.Fatalf("Failed to get message: %v", err)
        }

        if message == nil {
            break
        }

        // Handle different message types
        switch msg := message.(type) {
        case *claudecode.AssistantMessage:
            for _, block := range msg.Content {
                if textBlock, ok := block.(*claudecode.TextBlock); ok {
                    fmt.Print(textBlock.Text)
                }
            }
        case *claudecode.ResultMessage:
            if msg.IsError {
                log.Printf("Error: %s", msg.Result)
            }
        }
    }

    fmt.Println("\nQuery completed!")
}
```

### Client API - Interactive & Multi-Turn
**WithClient provides automatic resource management (equivalent to Python's `async with`):**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
    fmt.Println("Claude Code SDK - Client Streaming Example")
    fmt.Println("Asking: Explain Go goroutines with a simple example")

    ctx := context.Background()
    question := "Explain what Go goroutines are and show a simple example"

    // WithClient handles connection lifecycle automatically
    err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
        fmt.Println("\nConnected! Streaming response:")

        // Simple query uses default session
        if err := client.Query(ctx, question); err != nil {
            return fmt.Errorf("query failed: %w", err)
        }

        // Stream messages in real-time
        msgChan := client.ReceiveMessages(ctx)
        for {
            select {
            case message := <-msgChan:
                if message == nil {
                    return nil // Stream ended
                }

                switch msg := message.(type) {
                case *claudecode.AssistantMessage:
                    // Print streaming text as it arrives
                    for _, block := range msg.Content {
                        if textBlock, ok := block.(*claudecode.TextBlock); ok {
                            fmt.Print(textBlock.Text)
                        }
                    }
                case *claudecode.ResultMessage:
                    if msg.IsError {
                        return fmt.Errorf("error: %s", msg.Result)
                    }
                    return nil // Success, stream complete
                }
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    })

    if err != nil {
        log.Fatalf("Streaming failed: %v", err)
    }

    fmt.Println("\n\nStreaming completed!")
}
```

### Session Management

**Maintain conversation context across multiple queries with session management:**

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
    fmt.Println("Claude Code SDK - Session Management Example")

    ctx := context.Background()

    err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
        fmt.Println("\nDemonstrating isolated sessions:")

        // Session A: Math conversation
        sessionA := "math-session"
        if err := client.QueryWithSession(ctx, "Remember this: x = 5", sessionA); err != nil {
            return err
        }

        // Session B: Programming conversation
        sessionB := "programming-session"
        if err := client.QueryWithSession(ctx, "Remember this: language = Go", sessionB); err != nil {
            return err
        }

        // Query each session - they maintain separate contexts
        fmt.Println("\nQuerying math session:")
        if err := client.QueryWithSession(ctx, "What is x * 2?", sessionA); err != nil {
            return err
        }

        fmt.Println("\nQuerying programming session:")
        if err := client.QueryWithSession(ctx, "What language did I mention?", sessionB); err != nil {
            return err
        }

        // Default session query (separate from above)
        fmt.Println("\nDefault session (no context from above):")
        return client.Query(ctx, "What did I just ask about?") // Won't know about x or Go
    })

    if err != nil {
        log.Fatalf("Session demo failed: %v", err)
    }

    fmt.Println("Session management demo completed!")
}
```

**Traditional Client API (still supported):**

<details>
<summary>Click to see manual resource management approach</summary>

```go
func traditionalClientExample() {
    ctx := context.Background()
    
    client := claudecode.NewClient()
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer client.Disconnect() // Manual cleanup required
    
    // Use client...
}
```
</details>

## Hook System - Runtime Control & Interception

**New in v0.3.0**: Control and intercept tool execution with custom hooks (compatible with Python SDK v0.1.3):

```go
package main

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
    ctx := context.Background()
    
    // Define security hook
    securityHook := func(input claudecode.HookInput, toolUseID *string, ctx claudecode.HookContext) (claudecode.HookJSONOutput, error) {
        toolName := input["tool_name"].(string)
        
        if toolName == "Bash" {
            command := input["tool_input"].(map[string]any)["command"].(string)
            
            // Block dangerous commands
            if strings.Contains(command, "rm -rf") {
                return claudecode.NewBlockingOutput(
                    "Blocked dangerous command",
                    "Security policy violation",
                ), nil
            }
        }
        
        // Allow safe commands
        return claudecode.NewPreToolUseOutput(
            claudecode.PermissionDecisionAllow, "", nil,
        ), nil
    }
    
    // Use hook with query
    err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
        return client.Query(ctx, "List files in current directory")
    },
        // Attach hook to intercept Bash tool usage
        claudecode.WithHook(claudecode.HookEventPreToolUse, claudecode.HookMatcher{
            Matcher: "Bash",
            Hooks:   []claudecode.HookCallback{securityHook},
        }),
    )
    
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

**Hook capabilities:**
- **PreToolUse**: Intercept before tool execution, modify inputs, block dangerous operations
- **PostToolUse**: Process tool outputs, log results, transform responses
- **UserPromptSubmit**: Validate and transform user inputs
- **Stop/SubagentStop**: Handle completion events
- **PreCompact**: Manage context before compaction

**Common use cases:**
```go
// Security enforcement
claudecode.WithHook(claudecode.HookEventPreToolUse, securityHook)

// Audit logging
claudecode.WithHook(claudecode.HookEventPostToolUse, auditHook)

// Input validation
claudecode.WithHook(claudecode.HookEventUserPromptSubmit, validationHook)
```

See [`examples/12_hooks/`](examples/12_hooks/) for comprehensive hook examples including security policies, audit logging, and custom workflows.

## Tool Integration & External Services

Integrate with file systems, cloud services, databases, and development tools:

**Core Tools** (built-in file operations):
```go
// File analysis and documentation generation
claudecode.Query(ctx, "Read all Go files and create API documentation",
    claudecode.WithAllowedTools("Read", "Write"))
```

**MCP Tools** (external service integrations):
```go
// AWS infrastructure automation
claudecode.Query(ctx, "List my S3 buckets and analyze their security settings",
    claudecode.WithAllowedTools("mcp__aws-api-mcp__call_aws", "mcp__aws-api-mcp__suggest_aws_commands", "Write"))
```

## Configuration Options

Customize Claude's behavior with functional options:

**Tool & Permission Control:**
```go
claudecode.Query(ctx, prompt,
    claudecode.WithAllowedTools("Read", "Write"),
    claudecode.WithPermissionMode(claudecode.PermissionModeAcceptEdits))
```

**System Behavior:**
```go
claudecode.Query(ctx, prompt,
    claudecode.WithSystemPrompt("You are a senior Go developer"),
    claudecode.WithModel("claude-sonnet-4-5"),
    claudecode.WithMaxTurns(10))
```

**Environment Variables** (new in v0.2.5):
```go
// Proxy configuration
claudecode.NewClient(
    claudecode.WithEnv(map[string]string{
        "HTTP_PROXY":  "http://proxy.example.com:8080",
        "HTTPS_PROXY": "http://proxy.example.com:8080",
    }))

// Individual variables
claudecode.NewClient(
    claudecode.WithEnvVar("DEBUG", "1"),
    claudecode.WithEnvVar("CUSTOM_PATH", "/usr/local/bin"))
```

**Context & Working Directory:**
```go
claudecode.Query(ctx, prompt,
    claudecode.WithCwd("/path/to/project"),
    claudecode.WithAddDirs("src", "docs"))
```

**Structured Outputs** (new in v0.1.9):
```go
// Define JSON schema for validation
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "file_count": map[string]interface{}{"type": "number"},
        "has_tests":  map[string]interface{}{"type": "boolean"},
    },
    "required": []string{"file_count", "has_tests"},
}

// Get validated JSON responses
options := claudecode.NewOptions(
    claudecode.WithOutputFormat(map[string]interface{}{
        "type":   "json_schema",
        "schema": schema,
    }),
)

err := claudecode.Query(ctx, "Count Go files and check for tests", options, func(msg claudecode.Message) {
    if result, ok := msg.(*claudecode.ResultMessage); ok && result.StructuredOutput != nil {
        data := result.StructuredOutput.(map[string]interface{})
        fmt.Printf("Files: %.0f, Has tests: %v\n", data["file_count"], data["has_tests"])
    }
})
```

**Hook Integration** (enhanced in v0.1.9):
```go
// Attach hooks to control tool execution
claudecode.WithClient(ctx, func(client claudecode.Client) error {
    return client.Query(ctx, "Run system commands")
},
    claudecode.WithHook(claudecode.HookEventPreToolUse, claudecode.HookMatcher{
        Matcher: "Bash",
        Hooks:   []claudecode.HookCallback{securityHook},
        Timeout: claudecode.IntPtr(30), // 30 second timeout
    }),
    claudecode.WithHook(claudecode.HookEventPostToolUse, claudecode.HookMatcher{
        Matcher: "*", // All tools
        Hooks:   []claudecode.HookCallback{auditHook},
        Timeout: claudecode.IntPtr(60), // 60 second timeout
    }),
)
```

**Session Management** (Client API):
```go
// WithClient provides isolated session contexts
err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
    // Default session
    client.Query(ctx, "Remember: x = 5")

    // Named session (isolated context)
    return client.QueryWithSession(ctx, "What is x?", "math-session")
})
```

See [pkg.go.dev](https://pkg.go.dev/github.com/jonnyquan/claude-agent-sdk-go) for complete API reference.

## When to Use Which API

**🎯 Use Query API when you:**
- Need one-shot automation or scripting
- Have clear task completion criteria  
- Want automatic resource cleanup
- Are building CI/CD integrations
- Prefer simple, stateless operations

**🔄 Use Client API (WithClient) when you:**  
- Need interactive conversations
- Want to build context across multiple requests
- Are creating complex, multi-step workflows
- Need real-time streaming responses
- Want to iterate and refine based on previous results
- **Need automatic resource management (recommended)**

## Examples & Documentation

Comprehensive examples covering every use case:

**Learning Path (Easiest → Hardest):**
- [`examples/01_quickstart/`](examples/01_quickstart/) - Query API fundamentals
- [`examples/02_client_streaming/`](examples/02_client_streaming/) - WithClient streaming basics
- [`examples/03_client_multi_turn/`](examples/03_client_multi_turn/) - Multi-turn conversations with automatic cleanup
- [`examples/10_context_manager/`](examples/10_context_manager/) - WithClient vs manual patterns comparison
- [`examples/11_session_management/`](examples/11_session_management/) - Session isolation and context management

**Tool Integration:**
- [`examples/04_query_with_tools/`](examples/04_query_with_tools/) - File operations with Query API
- [`examples/05_client_with_tools/`](examples/05_client_with_tools/) - Interactive file workflows  
- [`examples/06_query_with_mcp/`](examples/06_query_with_mcp/) - AWS automation with Query API
- [`examples/07_client_with_mcp/`](examples/07_client_with_mcp/) - AWS management with Client API

**Advanced Patterns:**
- [`examples/08_client_advanced/`](examples/08_client_advanced/) - WithClient error handling and production patterns
- [`examples/09_client_vs_query/`](examples/09_client_vs_query/) - Modern API comparison and guidance
- [`examples/12_hooks/`](examples/12_hooks/) - **NEW**: Hook system with security, audit, and custom workflows

**📖 [Full Documentation](examples/README.md)** with usage patterns, security best practices, and troubleshooting.

## Version History

### v0.3.0 (Latest)
- **Hook System**: Complete implementation compatible with Python SDK v0.1.3
  - PreToolUse, PostToolUse, UserPromptSubmit, Stop, SubagentStop, PreCompact hooks
  - Permission control with Allow/Deny/Ask decisions
  - Runtime interception and control of tool execution
- **Security**: Custom security policies via hooks
- **Audit**: Complete audit logging capabilities
- **Examples**: Comprehensive hook examples in `examples/12_hooks/`

### v0.2.5
- Environment variable support (`WithEnv`, `WithEnvVar`)
- Proxy configuration
- Working directory and context management

### v0.2.0
- Client API with `WithClient` pattern
- Session management
- Streaming support

### v0.1.0
- Initial release with Query API
- Core tool integration
- Basic MCP support

## Development

### Testing

```bash
# Run all tests
make test

# Test hook system
make test-hooks

# Run hook examples
make example-hooks

# Full CI pipeline
make ci
```

### Building Examples

```bash
# Build all examples
make examples

# Run specific hook example
cd examples/12_hooks && go run main.go
```

See [`Makefile`](Makefile) for complete list of build targets.

## Development

If you're contributing to this project, run the initial setup script to install git hooks:

```bash
./scripts/initial-setup.sh
```

This installs a pre-push hook that runs lint checks before pushing, matching the CI workflow. To skip the hook temporarily, use `git push --no-verify`.

## License

MIT - See [LICENSE](LICENSE) for details.

Includes Hook System implementation (2025) maintaining compatibility with Python Claude Agent SDK v0.1.3.
