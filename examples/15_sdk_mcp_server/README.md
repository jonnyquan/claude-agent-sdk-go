# SDK MCP Server Example

This example demonstrates how to create and use in-process MCP servers with the Go SDK.

## What are SDK MCP Servers?

SDK MCP Servers are **in-process** MCP servers that run directly in your Go application, unlike external MCP servers that run as separate processes.

### Benefits

✅ **Better Performance**: No IPC overhead  
✅ **Simpler Deployment**: Single binary  
✅ **Easier Debugging**: Same process  
✅ **Direct State Access**: Access application variables directly  
✅ **Type Safety**: Go's type system  

### Comparison with External MCP Servers

| Feature | SDK Server (In-Process) | External Server (Stdio) |
|---------|------------------------|-------------------------|
| Performance | Fast (no IPC) | Slower (IPC overhead) |
| State Access | Direct | Requires serialization |
| Deployment | Single binary | Multiple processes |
| Debugging | Easy (same process) | Complex (multiple processes) |
| Language | Go only | Any language |
| Isolation | Same process | Separate process |

## Basic Usage

### 1. Define a Tool

```go
import claudecode "github.com/jonnyquan/claude-agent-sdk-go"

greet := &claudecode.ToolDef{
    Name:        "greet",
    Description: "Greet a user by name",
    InputSchema: map[string]interface{}{
        "name": "string",
    },
    Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
        name := args["name"].(string)
        return []claudecode.ToolContent{
            claudecode.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
        }, nil
    },
}
```

### 2. Create Server

```go
server := claudecode.CreateSDKMcpServer(
    "greeting-tools",  // server name
    "1.0.0",           // version
    greet,             // tools...
)
```

### 3. Use with SDK

```go
result, err := claudecode.Query(
    ctx,
    "Greet Alice",
    claudecode.NewOptions(
        claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
            "greeting": server,
        }),
    ),
)
```

## Examples in This Directory

### Example 1: Simple Text Tool
Basic tool that returns text content.

### Example 2: Image Content
Tool that returns both text and image content (base64 encoded).

### Example 3: Multiple Tools
Multiple tools in a single server (calculator example).

### Example 4: Application State Access
Tools that access and modify application state.

## Tool Definition

### ToolDef Structure

```go
type ToolDef struct {
    // Unique identifier for the tool
    Name string
    
    // Human-readable description
    Description string
    
    // Parameter schema (can be simple or JSON Schema)
    InputSchema interface{}
    
    // Function that executes the tool
    Handler ToolHandler
}
```

### Input Schema Formats

**Simple Type Mapping**:
```go
InputSchema: map[string]interface{}{
    "name": "string",
    "age":  "integer",
}
```

**Full JSON Schema**:
```go
InputSchema: map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{
            "type":        "string",
            "description": "User's name",
        },
        "age": map[string]interface{}{
            "type":    "integer",
            "minimum": 0,
        },
    },
    "required": []string{"name"},
}
```

## Content Types

### Text Content

```go
claudecode.NewTextContent("Hello, world!")
```

### Image Content

```go
// data should be base64 encoded
claudecode.NewImageContent(base64Data, "image/png")
```

Supported image MIME types:
- `image/png`
- `image/jpeg`
- `image/gif`
- `image/webp`

## Advanced Features

### Context Support

Tools receive `context.Context` for cancellation and timeouts:

```go
Handler: func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(someWork()):
        return result, nil
    }
}
```

### State Access

Tools can directly access application state:

```go
var appState *MyAppState

tool := &ToolDef{
    Handler: func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
        // Direct access to application state
        appState.Update(args["data"])
        return []ToolContent{NewTextContent("Updated")}, nil
    },
}
```

### Error Handling

Return errors for tool failures:

```go
Handler: func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
    if invalid(args) {
        return nil, fmt.Errorf("invalid input: %v", args)
    }
    return []ToolContent{NewTextContent("Success")}, nil
}
```

## Convenience Functions

### Tool() Helper

Shorter syntax for creating tools:

```go
tool := claudecode.Tool(
    "greet",
    "Greet user",
    map[string]interface{}{"name": "string"},
    func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
        return []ToolContent{NewTextContent("Hello!")}, nil
    },
)
```

## Running the Examples

```bash
cd examples/15_sdk_mcp_server
go run main.go
```

## Best Practices

1. **Tool Names**: Use clear, descriptive names (e.g., `add_numbers` not `add`)
2. **Descriptions**: Write clear descriptions that help Claude understand when to use the tool
3. **Input Validation**: Always validate tool inputs
4. **Error Handling**: Return meaningful errors
5. **Context Respect**: Always respect context cancellation
6. **State Safety**: Use mutexes if tools access shared state concurrently

## Comparison with Python SDK

### Python SDK

```python
from claude_agent_sdk import tool, create_sdk_mcp_server

@tool("greet", "Greet user", {"name": str})
async def greet(args):
    return {"content": [{"type": "text", "text": f"Hello, {args['name']}!"}]}

server = create_sdk_mcp_server("greeting", tools=[greet])
```

### Go SDK

```go
greet := &claudecode.ToolDef{
    Name:        "greet",
    Description: "Greet user",
    InputSchema: map[string]interface{}{"name": "string"},
    Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
        name := args["name"].(string)
        return []claudecode.ToolContent{
            claudecode.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
        }, nil
    },
}

server := claudecode.CreateSDKMcpServer("greeting", "1.0.0", greet)
```

**Key Differences**:
- Go: Explicit type assertions vs Python: Dynamic typing
- Go: Context support vs Python: async/await
- Go: Error returns vs Python: Exceptions
- Both: Support text and image content

## Troubleshooting

### Tool Not Found

Make sure:
- Tool is registered with the server
- Server is added to `mcp_servers` configuration
- Tool name matches exactly

### Type Assertion Panics

Always check types before assertion:
```go
if name, ok := args["name"].(string); ok {
    // use name safely
}
```

### Context Timeout

Respect context deadlines:
```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
case result := <-workChan:
    return result, nil
}
```

## See Also

- [MCP Protocol Documentation](https://modelcontextprotocol.io)
- [External MCP Server Example](../06_query_with_mcp)
- [SDK API Reference](../../README.md)
