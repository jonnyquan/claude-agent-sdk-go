# Example 19: New Query Patterns

This example demonstrates various query patterns using the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **Simple Query** - Basic one-shot query usage
2. **Query with Options** - Configuring queries with system prompts, models, etc.
3. **Query with Timeout** - Using context timeout for query management
4. **Multiple Queries** - Handling multiple concurrent queries

## 🚀 Key Features Demonstrated

### New API Import
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
```

### Query Patterns
- `claudesdk.Query(ctx, prompt)` - Simple query
- `claudesdk.Query(ctx, prompt, options...)` - Configured query
- Context-based timeout management
- Multiple query handling

### Configuration Options
- `claudesdk.WithSystemPrompt()` - Set system prompt
- `claudesdk.WithModel()` - Choose specific model
- `claudesdk.WithCwd()` - Set working directory

## 🔧 Running the Example

```bash
cd examples/19_new_query_patterns
go run main.go
```

## 📚 Benefits of New API

1. **Clear Import Path** - Explicit pkg/claudesdk import
2. **Consistent Patterns** - All claudesdk.* functions  
3. **Better Organization** - Related functionality grouped together
4. **IDE Support** - Better autocompletion and documentation

## 💡 Migration Notes

Old way (still works):
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
claudecode.Query(ctx, prompt)
```

New way (recommended):
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
claudesdk.Query(ctx, prompt)
```

Same functionality, cleaner organization!
