# Claude Agent SDK for Go

<div align="center">

[![Go Reference](https://pkg.go.dev/badge/github.com/jonnyquan/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/jonnyquan/claude-agent-sdk-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonnyquan/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/jonnyquan/claude-agent-sdk-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Version](https://img.shields.io/badge/version-0.1.9-blue.svg)](https://github.com/jonnyquan/claude-agent-sdk-go/releases)

**Production-ready Go SDK for Claude AI agent integration**

[Features](#-features) • [Installation](#-installation) • [Quick Start](#-quick-start) • [Examples](#-examples) • [Documentation](#-documentation)

</div>

---

## 🌟 Overview

The Claude Agent SDK for Go provides a clean, idiomatic interface to build AI-powered applications with Claude. Designed for production use with comprehensive error handling, automatic resource management, and full feature parity with the official Python SDK.

### Why This SDK?

- **🎯 Two Powerful APIs**: Query API for automation, Client API for interactive workflows
- **📦 Zero-Config Deployment**: Optional bundled CLI - no separate installation required
- **🔒 Production-Ready**: Comprehensive error handling, timeouts, resource cleanup
- **🔄 100% Python SDK Parity**: Same features, Go-native design (v0.1.9)
- **🪝 Advanced Hook System**: Intercept and control tool execution with custom callbacks
- **📊 Structured Outputs**: JSON schema validation for guaranteed response formats
- **🔗 Rich Integrations**: File operations, MCP servers, external tools
- **🛡️ Security-First**: Granular permissions, access controls, runtime hooks

## 📦 Installation

```bash
go get github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk
```

**Requirements:**
- Go 1.18 or later
- Claude CLI 2.0.50+ (auto-bundled or manual installation)

**Optional**: The SDK can bundle the Claude CLI for zero-dependency deployment. For custom installations:
```bash
curl -fsSL https://claude.ai/install.sh | bash
```

## 🚀 Quick Start

### New API (Recommended) - `pkg/claudesdk`

The new API provides a cleaner, more organized structure:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
    ctx := context.Background()

    // Query API - Simple one-shot operations
    messages, err := claudesdk.Query(ctx, "Explain Go channels in 2 sentences")
    if err != nil {
        log.Fatal(err)
    }

    // Process response
    for {
        msg, err := messages.Next(ctx)
        if err != nil {
            if err == claudesdk.ErrNoMoreMessages {
                break
            }
            log.Fatal(err)
        }

        if assistant, ok := msg.(*claudesdk.AssistantMessage); ok {
            for _, block := range assistant.Content {
                if text, ok := block.(*claudesdk.TextBlock); ok {
                    fmt.Println(text.Text)
                }
            }
        }
    }
}
```

### Client API - Interactive Workflows

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
    ctx := context.Background()

    // WithClient provides automatic resource management
    err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
        // Send query
        if err := client.Query(ctx, "Write a hello world in Go"); err != nil {
            return err
        }

        // Receive streaming response
        messages := client.ReceiveResponse(ctx)
        for {
            msg, err := messages.Next(ctx)
            if err != nil {
                if err == claudesdk.ErrNoMoreMessages {
                    break
                }
                return err
            }

            if assistant, ok := msg.(*claudesdk.AssistantMessage); ok {
                for _, block := range assistant.Content {
                    if text, ok := block.(*claudesdk.TextBlock); ok {
                        fmt.Print(text.Text)
                    }
                }
            }
        }
        return nil
    })

    if err != nil {
        log.Fatal(err)
    }
}
```

### Backward Compatibility

The old API is still supported via the compatibility layer:

```go
import "github.com/jonnyquan/claude-agent-sdk-go"  // Old API

// Works exactly as before
messages, err := claudecode.Query(ctx, "Hello!")
```

## ✨ Features

### Query API - Automation & Scripting

Perfect for one-shot operations, automation, and CI/CD integration:

```go
// Simple query with options
messages, err := claudesdk.Query(ctx, 
    "Analyze this codebase and suggest improvements",
    claudesdk.WithCwd("/path/to/project"),
    claudesdk.WithAllowedTools("Read", "List"),
    claudesdk.WithModel("claude-sonnet-4-5"),
)
```

**Use Query API when you:**
- Need automated code analysis or generation
- Want one-shot task completion
- Are building CI/CD integrations
- Prefer stateless operations

### Client API - Interactive & Multi-Turn

For complex workflows and interactive applications:

```go
err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    // First query
    client.Query(ctx, "Initialize a Go project")
    
    // Follow-up in same context
    client.Query(ctx, "Add unit tests")
    
    // Another follow-up
    return client.Query(ctx, "Add CI/CD configuration")
},
    claudesdk.WithSystemPrompt("You are a Go expert"),
    claudesdk.WithMaxTurns(10),
)
```

**Use Client API when you:**
- Need multi-turn conversations
- Want to build context across requests
- Are creating interactive applications
- Need real-time streaming

### Structured Outputs - Type-Safe Responses

Get validated JSON responses with schema enforcement:

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "summary": map[string]interface{}{
            "type": "string",
        },
        "line_count": map[string]interface{}{
            "type": "number",
        },
        "languages": map[string]interface{}{
            "type": "array",
            "items": map[string]interface{}{"type": "string"},
        },
    },
    "required": []string{"summary", "line_count", "languages"},
}

messages, err := claudesdk.Query(ctx,
    "Analyze this repository",
    claudesdk.WithOutputFormat(map[string]interface{}{
        "type":   "json_schema",
        "schema": schema,
    }),
)

// Get validated structured output
for {
    msg, _ := messages.Next(ctx)
    if result, ok := msg.(*claudesdk.ResultMessage); ok {
        if result.StructuredOutput != nil {
            data := result.StructuredOutput.(map[string]interface{})
            fmt.Printf("Summary: %s\n", data["summary"])
            fmt.Printf("Lines: %.0f\n", data["line_count"])
        }
    }
}
```

### Hook System - Runtime Control

Intercept and control AI operations with custom hooks:

```go
// Security hook to block dangerous operations
securityHook := func(input claudesdk.HookInput, toolUseID *string, 
                      ctx claudesdk.HookContext) (claudesdk.HookJSONOutput, error) {
    
    if toolName, ok := input["tool_name"].(string); ok && toolName == "Bash" {
        command := input["tool_input"].(map[string]any)["command"].(string)
        
        if strings.Contains(command, "rm -rf") {
            return claudesdk.HookJSONOutput{
                "hookEventName":            "PreToolUse",
                "permissionDecision":       claudesdk.PermissionDecisionDeny,
                "permissionDecisionReason": "Dangerous command blocked",
            }, nil
        }
    }
    
    return claudesdk.NewPreToolUseOutput(
        claudesdk.PermissionDecisionAllow, "", nil,
    ), nil
}

// Apply hook
err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    return client.Query(ctx, "Run system maintenance")
},
    claudesdk.WithHook(claudesdk.HookEventPreToolUse, claudesdk.HookMatcher{
        Callback: securityHook,
        Timeout:  claudesdk.IntPtr(30),
    }),
)
```

**Available hook events:**
- `PreToolUse` - Before tool execution
- `PostToolUse` - After tool execution
- `UserPromptSubmit` - On user input
- `Stop` - On conversation completion
- `SubagentStop` - On subagent completion
- `PreCompact` - Before context compaction

### Session Management

Maintain isolated conversation contexts:

```go
err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    // Session A: Math problems
    client.QueryWithSession(ctx, "x = 5", "math")
    client.QueryWithSession(ctx, "What is x * 2?", "math") // Returns 10
    
    // Session B: Coding (separate context)
    client.QueryWithSession(ctx, "language = Go", "coding")
    client.QueryWithSession(ctx, "What language?", "coding") // Returns Go
    
    // Default session (isolated from A and B)
    return client.Query(ctx, "What did I ask?") // No prior context
})
```

### MCP Server Integration

Connect to external tools and data sources:

```go
// Create custom MCP server
server := claudesdk.CreateSDKMcpServer(&claudesdk.McpSdkServerConfig{
    Name:        "calculator",
    Description: "Math operations",
    Tools: []claudesdk.McpTool{
        {
            Name:        "add",
            Description: "Add two numbers",
            Handler: func(args map[string]interface{}) (string, error) {
                a := args["a"].(float64)
                b := args["b"].(float64)
                return fmt.Sprintf("%.2f", a+b), nil
            },
        },
    },
})

// Use in query
err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    return client.Query(ctx, "Calculate 15 + 27")
},
    claudesdk.WithMcpServers(map[string]claudesdk.McpServerConfig{
        "calculator": server,
    }),
)
```

## 📚 Examples

We provide 24 comprehensive examples covering all use cases:

### Getting Started (1-11)
- **01**: Quick Start - Basic Query API
- **02**: Client Streaming - Real-time responses
- **03**: Multi-turn Conversations
- **04**: Query with Tools
- **05**: Client with Tools
- **06**: Query with MCP
- **07**: Client with MCP
- **08**: Advanced Client Patterns
- **09**: Client vs Query Comparison
- **10**: Context Manager Patterns
- **11**: Session Management

### Advanced Features (12-18)
- **12**: Hook System
- **14**: Plugin Support
- **15**: SDK MCP Server
- **17**: Structured Outputs
- **18**: New API Basics

### New API Examples (19-24) 🆕
- **19**: [Query Patterns](examples/19_new_query_patterns/) - Multiple configurations, timeouts
- **20**: [Client Streaming](examples/20_new_client_streaming/) - WithClient patterns, multi-turn
- **21**: [Structured Outputs](examples/21_new_structured_outputs/) - JSON schema validation
- **22**: [Hooks System](examples/22_new_hooks_system/) - Event handling and callbacks
- **23**: [MCP Integration](examples/23_new_mcp_integration/) - Server configuration
- **24**: [Error Handling](examples/24_new_error_handling/) - Production patterns

Run any example:
```bash
cd examples/19_new_query_patterns
go run main.go
```

## 📖 Documentation

### API Reference

Full documentation available at [pkg.go.dev](https://pkg.go.dev/github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk)

### Configuration Options

```go
// Tool and permission control
claudesdk.WithAllowedTools("Read", "Write", "List")
claudesdk.WithPermissionMode(claudesdk.PermissionModeAcceptEdits)

// System behavior
claudesdk.WithSystemPrompt("You are a helpful assistant")
claudesdk.WithModel("claude-sonnet-4-5")
claudesdk.WithMaxTurns(20)
claudesdk.WithMaxBudgetUSD(1.0)

// Context and environment
claudesdk.WithCwd("/path/to/project")
claudesdk.WithAddDirs("src", "tests")
claudesdk.WithEnv(map[string]string{"DEBUG": "1"})

// Advanced features
claudesdk.WithFallbackModel("claude-3-haiku-20240307")
claudesdk.WithOutputFormat(jsonSchema)
claudesdk.WithHook(eventType, matcher)
claudesdk.WithMcpServers(serverMap)
```

### Migration Guide

Migrating from old API to new API? See [MIGRATION.md](MIGRATION.md) for a complete guide.

**Quick reference:**
```go
// Old API
import "github.com/jonnyquan/claude-agent-sdk-go"
claudecode.Query(ctx, "Hello")

// New API (recommended)
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
claudesdk.Query(ctx, "Hello")
```

Both APIs work - the new one is better organized and recommended for new projects.

## 🏗️ Project Structure

```
claude-agent-sdk-go/
├── pkg/
│   └── claudesdk/        # 🆕 New public API
│       ├── client.go     # Client interface
│       ├── query.go      # Query operations
│       ├── types.go      # Public types
│       ├── options.go    # Configuration options
│       ├── hooks.go      # Hook system
│       ├── mcp.go        # MCP integration
│       ├── errors.go     # Error definitions
│       └── permissions.go # Permission management
├── internal/
│   ├── client/           # Client implementation
│   ├── query/            # Query implementation
│   ├── transport/        # Claude CLI communication
│   ├── discovery/        # CLI discovery and bundling
│   ├── parsing/          # Message parsing
│   ├── mcp/              # MCP server implementation
│   └── shared/           # Shared types and utilities
├── claudecode.go         # Backward compatibility layer
├── examples/             # 24 comprehensive examples
└── MIGRATION.md          # Migration guide
```

## 🔧 Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./pkg/claudesdk
go test ./internal/...

# With coverage
go test -cover ./...
```

### Building Examples

```bash
# Build all examples
for dir in examples/*/; do
    (cd "$dir" && go build)
done

# Run specific example
cd examples/19_new_query_patterns
go run main.go
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

### Guidelines

1. Follow Go best practices and idioms
2. Add tests for new features
3. Update documentation
4. Ensure backward compatibility when possible

## 📝 Version History

### v0.1.9 (Current) - 2025-01-23
- ✨ New `pkg/claudesdk` API with cleaner structure
- 📊 Structured outputs with JSON schema validation
- 🔧 CLI auto-bundling for zero-dependency deployment
- 🪝 Hook system timeout configuration
- 🐛 Enhanced error handling with AssistantMessageError
- 📚 6 new comprehensive examples (19-24)
- 🔄 Full backward compatibility via `claudecode.go`
- 📖 Complete MIGRATION.md guide

### v0.1.6 - 2025-01-20
- 🔄 Fallback model support
- 🎯 Enhanced session management
- 📝 Documentation improvements

### v0.1.0 - 2025-01-15
- 🎉 Initial release
- 🎯 Query and Client APIs
- 🪝 Hook system
- 🔗 MCP integration
- 📦 Core tool support

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

Copyright (c) 2025 Jonny Quan and Contributors

---

<div align="center">

**Built with ❤️ for the Go community**

[Report Bug](https://github.com/jonnyquan/claude-agent-sdk-go/issues) • [Request Feature](https://github.com/jonnyquan/claude-agent-sdk-go/issues) • [Documentation](https://pkg.go.dev/github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk)

</div>
