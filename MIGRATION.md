# 📚 Migration Guide: New Package Structure

## 🎯 What Changed

We've restructured the Claude Agent SDK for Go to follow best practices and improve maintainability while maintaining **100% backward compatibility**.

### New Package Structure

```
claude-agent-sdk-go/
├── pkg/claudesdk/          # 🆕 Public API (recommended)
│   ├── client.go
│   ├── query.go  
│   ├── types.go
│   └── ...
├── internal/               # ♻️  Internal implementation (reorganized)
│   ├── transport/          # (was subprocess/)
│   ├── discovery/          # (was cli/)
│   ├── parsing/            # (was parser/)
│   └── ...
├── claudecode.go           # 🔄 Backward compatibility layer
└── examples/               # 📖 Updated examples
```

## 🚀 Migration Options

### Option 1: **Recommended** - Use New Package Structure

Update your imports to use the new `pkg/claudesdk` package:

```go
// Before
import "github.com/jonnyquan/claude-agent-sdk-go"

// After  
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"

// Usage remains the same
client := claudesdk.NewClient()
messages, err := claudesdk.Query(ctx, "Hello!")
```

### Option 2: **No Changes Required** - Use Compatibility Layer

Keep using the existing import - everything continues to work:

```go
// This still works!
import "github.com/jonnyquan/claude-agent-sdk-go"

client := claudecode.NewClient()
messages, err := claudecode.Query(ctx, "Hello!")
```

## 📦 Benefits of New Structure

### ✅ **Cleaner Organization**
- Public API is clearly separated in `pkg/claudesdk/`
- Internal implementation is protected in `internal/`
- Better module boundaries and dependencies

### ✅ **Go Best Practices**  
- Follows standard Go project layout
- Uses `pkg/` for library packages
- Uses `internal/` to hide implementation details

### ✅ **Better Maintainability**
- Each package has a single responsibility
- Easier to test and develop
- Clear separation of public and private APIs

### ✅ **1:1 Python SDK Mapping**
```
Python SDK              →  Go SDK
claude_agent_sdk/       →  pkg/claudesdk/
claude_agent_sdk/_internal/ → internal/
```

## 🔄 API Changes

**None!** All functions, types, and behavior remain identical. Only the import path changes if you choose to migrate.

## ⏰ Timeline

- ✅ **Now**: Both old and new APIs available
- 📅 **Future**: Old root-level API marked as deprecated (but still functional)
- 🎯 **Long-term**: New package structure becomes the primary recommendation

## 🛠️ Examples

### Query API
```go
// New recommended way
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"

messages, err := claudesdk.Query(ctx, "What's the weather?")

// Old way (still works)
import "github.com/jonnyquan/claude-agent-sdk-go"

messages, err := claudecode.Query(ctx, "What's the weather?")
```

### Client API  
```go
// New recommended way
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"

client := claudesdk.NewClient(
    claudesdk.WithSystemPrompt("You are helpful"),
)

// Old way (still works)  
import "github.com/jonnyquan/claude-agent-sdk-go"

client := claudecode.NewClient(
    claudecode.WithSystemPrompt("You are helpful"),
)
```

## 🎯 Recommendation

**For new projects**: Use `github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk`

**For existing projects**: No immediate changes required, migrate when convenient

## ❓ Questions

If you have any questions about the migration, please [open an issue](https://github.com/jonnyquan/claude-agent-sdk-go/issues).
