# Example 18: New API Structure

This example demonstrates the new package structure and API for the Claude Agent SDK for Go.

## 🎯 What's New

### New Import Path
```go
// New recommended way
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"

// Old way (still works via compatibility layer)
import "github.com/jonnyquan/claude-agent-sdk-go"
```

### Same API, Cleaner Structure
- **All functionality remains identical**
- **Cleaner package organization**
- **Better separation of public and internal APIs**
- **1:1 mapping with Python SDK structure**

## 🚀 Running the Example

```bash
go run main.go
```

## 📋 What This Example Shows

1. **Simple Query** - One-shot query using the new API
2. **Client Usage** - Creating and using a client with options
3. **WithClient Pattern** - Recommended resource management pattern

## 🔗 Benefits of New Structure

### For Users
- ✅ **Cleaner imports** - Clear public API separation
- ✅ **Better docs** - Package documentation is more focused
- ✅ **IDE support** - Better autocompletion and discovery

### For Maintainers  
- ✅ **Organized code** - Internal implementation is properly isolated
- ✅ **Python SDK mapping** - Direct correspondence with Python structure
- ✅ **Go best practices** - Follows standard Go project layout

## 🔄 Migration

### Option 1: No Changes (Recommended for existing projects)
Keep using your existing imports - they still work:
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
// All your code continues to work unchanged
```

### Option 2: New API (Recommended for new projects)
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
// Same functions, cleaner structure
```

## 📚 Documentation

See [MIGRATION.md](../../MIGRATION.md) for complete migration guide.

## 🎊 Summary

The new structure provides better organization while maintaining 100% backward compatibility. Choose the approach that works best for your project!
