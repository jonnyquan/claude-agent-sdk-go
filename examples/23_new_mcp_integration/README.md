# Example 23: New MCP Integration API

This example demonstrates MCP (Model Context Protocol) integration using the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **Simple SDK MCP Server** - Create in-process MCP server with custom tools
2. **Advanced SDK MCP Server** - Multiple tools with complex logic
3. **External MCP Server** - Connect to external MCP servers
4. **Multiple MCP Servers** - Use multiple servers simultaneously

## 🚀 Key Features Demonstrated

### SDK MCP Server Creation
```go
// Create custom tools
greetTool := claudesdk.Tool("greet", "Greet a user",
    schema,
    func(ctx context.Context, args map[string]interface{}) ([]claudesdk.ToolContent, error) {
        name := args["name"].(string)
        return []claudesdk.ToolContent{
            claudesdk.NewTextContent(fmt.Sprintf("Hello, %s!", name)),
        }, nil
    },
)

// Create MCP server
server := claudesdk.CreateSDKMcpServer("my-server", "1.0.0", greetTool)
```

### MCP Server Configuration
```go
claudesdk.WithMcpServers(map[string]claudesdk.McpServerConfig{
    "my-tools": *server,
    "filesystem": {
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
        Env:     map[string]string{},
    },
})
```

### Tool Response Types
```go
// Text content
claudesdk.NewTextContent("Hello, world!")

// Image content (for tools that return images)
claudesdk.NewImageContent(imageData, "image/png")
```

## 🔧 Running the Example

```bash
cd examples/23_new_mcp_integration
go run main.go
```

## 💡 MCP Use Cases

### 1. **Custom Business Logic**
```go
orderTool := claudesdk.Tool("process_order", "Process customer order",
    orderSchema,
    func(ctx context.Context, args map[string]interface{}) ([]claudesdk.ToolContent, error) {
        orderID := args["order_id"].(string)
        // Process order in your business system
        result := processOrder(orderID)
        return []claudesdk.ToolContent{
            claudesdk.NewTextContent(fmt.Sprintf("Order %s processed: %s", orderID, result)),
        }, nil
    },
)
```

### 2. **Data Access Tools**
```go
dbTool := claudesdk.Tool("query_database", "Query customer database",
    querySchema,
    func(ctx context.Context, args map[string]interface{}) ([]claudesdk.ToolContent, error) {
        query := args["query"].(string)
        results := executeQuery(query)
        return []claudesdk.ToolContent{
            claudesdk.NewTextContent(formatResults(results)),
        }, nil
    },
)
```

### 3. **API Integration Tools**
```go
apiTool := claudesdk.Tool("call_api", "Call external API",
    apiSchema,
    func(ctx context.Context, args map[string]interface{}) ([]claudesdk.ToolContent, error) {
        endpoint := args["endpoint"].(string)
        response := callExternalAPI(endpoint)
        return []claudesdk.ToolContent{
            claudesdk.NewTextContent(string(response)),
        }, nil
    },
)
```

### 4. **File Processing Tools**
```go
fileTool := claudesdk.Tool("process_file", "Process uploaded file",
    fileSchema,
    func(ctx context.Context, args map[string]interface{}) ([]claudesdk.ToolContent, error) {
        filename := args["filename"].(string)
        content := processFile(filename)
        return []claudesdk.ToolContent{
            claudesdk.NewTextContent(fmt.Sprintf("Processed file %s: %s", filename, content)),
        }, nil
    },
)
```

## 📋 Tool Schema Examples

### Simple Schema
```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{
            "type": "string",
            "description": "User name",
        },
    },
    "required": []string{"name"},
}
```

### Complex Schema
```go
complexSchema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "operation": map[string]interface{}{
            "type": "string",
            "enum": []string{"create", "update", "delete"},
            "description": "Operation to perform",
        },
        "data": map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "id": map[string]interface{}{"type": "integer"},
                "name": map[string]interface{}{"type": "string"},
            },
        },
    },
    "required": []string{"operation", "data"},
}
```

## 🌐 External MCP Servers

Connect to community MCP servers:

### Filesystem Server
```go
"filesystem": {
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/path"},
    Env:     map[string]string{},
}
```

### Git Server
```go
"git": {
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-git", "--repository", "."},
    Env:     map[string]string{},
}
```

### Database Server
```go
"database": {
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://..."},
    Env:     map[string]string{},
}
```

## 🔄 Migration Notes

The new API makes MCP integration more explicit:

Old way (still works):
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
server := claudecode.CreateSDKMcpServer("name", "1.0.0", tools...)
```

New way (recommended):
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
server := claudesdk.CreateSDKMcpServer("name", "1.0.0", tools...)
```

## ✨ Benefits of New API

1. **Organized Structure** - All MCP functionality in `pkg/claudesdk`
2. **Clear Tool Creation** - `claudesdk.Tool()` for easy tool definition
3. **Better Error Handling** - Improved error propagation from tools
4. **Type Safety** - Strong typing for tool arguments and responses

Perfect for building AI applications with custom business logic and external integrations!
