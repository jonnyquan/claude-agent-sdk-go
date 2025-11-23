# Example 20: New Client Streaming API

This example demonstrates streaming client patterns using the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **Manual Client Management** - Creating and managing client lifecycle manually
2. **WithClient Pattern** - Recommended resource management pattern
3. **Multi-turn Conversation** - Handling conversation sessions
4. **Advanced Options** - Using various client configuration options

## 🚀 Key Features Demonstrated

### Client Creation
```go
client := claudesdk.NewClient(
    claudesdk.WithSystemPrompt("You are helpful"),
    claudesdk.WithModel("claude-3-sonnet-20241022"),
)
```

### Resource Management Patterns

#### Manual (for fine control):
```go
client := claudesdk.NewClient(options...)
client.Connect(ctx)
defer client.Disconnect()
```

#### WithClient (recommended):
```go
claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    // Use client here
    return nil
}, options...)
```

### Streaming Communication
- `client.Query(ctx, prompt)` - Send queries
- `client.QueryWithSession(ctx, prompt, sessionID)` - Session-aware queries  
- `client.ReceiveResponse(ctx)` - Get streaming responses
- `messages.Next(ctx)` - Process individual messages

## 🔧 Running the Example

```bash
cd examples/20_new_client_streaming
go run main.go
```

## 💡 Key Patterns

### 1. Resource Management
The new API provides two patterns:

**Manual (when you need fine control):**
```go
client := claudesdk.NewClient()
if err := client.Connect(ctx); err != nil {
    return err
}
defer client.Disconnect()
```

**WithClient (recommended for most cases):**
```go
err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
    // Client is automatically connected and will be disconnected
    return client.Query(ctx, "Hello!")
})
```

### 2. Configuration Options
All the same options are available:
- `claudesdk.WithSystemPrompt(prompt)` - Set system prompt
- `claudesdk.WithModel(model)` - Choose model
- `claudesdk.WithCwd(path)` - Set working directory
- `claudesdk.WithMaxThinkingTokens(tokens)` - Control thinking tokens

### 3. Session Management
Use `QueryWithSession` for multi-turn conversations:
```go
client.Query(ctx, "First question")
client.QueryWithSession(ctx, "Follow-up question", "session-id")
```

## 📚 Benefits of New API

1. **Cleaner Imports** - `pkg/claudesdk` makes intent clear
2. **Better Organization** - All client functionality in one place
3. **Consistent Naming** - All functions start with `claudesdk.`
4. **Same Functionality** - No features lost in migration

## 🔄 Migration from Old API

Old way (still works):
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
client := claudecode.NewClient()
```

New way (recommended):
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"  
client := claudesdk.NewClient()
```

Exact same functionality, just cleaner organization!
