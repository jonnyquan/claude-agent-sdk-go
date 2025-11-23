# Example 22: New Hooks System API

This example demonstrates the hooks system for intercepting and controlling SDK behavior using the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **Pre-Tool Use Hook** - Intercept before tool execution
2. **Post-Tool Use Hook** - Process after tool execution
3. **Multiple Hooks** - Configure multiple hook types together
4. **Hook with Timeout** - Add timeout protection to hooks

## 🚀 Key Features Demonstrated

### Hook Creation
```go
hook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        // Your hook logic here
        return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionAllow, nil)
    },
}
```

### Hook Configuration
```go
// Single hook
claudesdk.WithHook(claudesdk.HookEventPreToolUse, hook)

// Multiple hooks
hooks := map[string][]claudesdk.HookMatcher{
    string(claudesdk.HookEventPreToolUse):  {preHook},
    string(claudesdk.HookEventPostToolUse): {postHook},
}
claudesdk.WithHooks(hooks)
```

### Hook Types Available
- `claudesdk.HookEventPreToolUse` - Before tool execution
- `claudesdk.HookEventPostToolUse` - After tool execution
- `claudesdk.HookEventUserPromptSubmit` - When user submits prompt
- `claudesdk.HookEventStop` - When conversation stops
- `claudesdk.HookEventSubagentStop` - When subagent stops
- `claudesdk.HookEventPreCompact` - Before message compaction

## 🔧 Running the Example

```bash
cd examples/22_new_hooks_system
go run main.go
```

## 💡 Hook Use Cases

### 1. **Access Control**
```go
preToolHook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        if preInput, ok := input.(*claudesdk.PreToolUseHookInput); ok {
            if preInput.ToolName == "SensitiveTool" {
                return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionDeny, 
                    strPtr("Access denied for sensitive tool"))
            }
        }
        return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionAllow, nil)
    },
}
```

### 2. **Audit Logging**
```go
postToolHook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        if postInput, ok := input.(*claudesdk.PostToolUseHookInput); ok {
            log.Printf("Tool executed: %s, Success: %v", 
                postInput.ToolName, postInput.Success)
        }
        return claudesdk.NewPostToolUseOutput(nil)
    },
}
```

### 3. **Input Validation**
```go
preToolHook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        if preInput, ok := input.(*claudesdk.PreToolUseHookInput); ok {
            if !validateToolArgs(preInput.Arguments) {
                return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionDeny,
                    strPtr("Invalid tool arguments"))
            }
        }
        return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionAllow, nil)
    },
}
```

### 4. **Performance Monitoring**
```go
var startTime time.Time

preHook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        startTime = time.Now()
        return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionAllow, nil)
    },
}

postHook := claudesdk.HookMatcher{
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        duration := time.Since(startTime)
        log.Printf("Tool execution took: %v", duration)
        return claudesdk.NewPostToolUseOutput(nil)
    },
}
```

## 📋 Permission Decisions

Hooks can control tool execution:

- `claudesdk.PermissionDecisionAllow` - Allow tool execution
- `claudesdk.PermissionDecisionDeny` - Deny tool execution  
- `claudesdk.PermissionDecisionAsk` - Ask user for permission

## ⏰ Hook Timeouts

Add timeout protection to prevent hanging hooks:

```go
hook := claudesdk.HookMatcher{
    Timeout: func() *int { t := 5; return &t }(), // 5 second timeout
    Callback: func(input claudesdk.HookInput, context claudesdk.HookContext) claudesdk.HookJSONOutput {
        // Hook logic with timeout protection
        return claudesdk.NewPreToolUseOutput(claudesdk.PermissionDecisionAllow, nil)
    },
}
```

## 🔄 Migration Notes

The new API makes hooks more explicit and organized:

Old way (still works):
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
claudecode.WithHook(claudecode.HookEventPreToolUse, hook)
```

New way (recommended):
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
claudesdk.WithHook(claudesdk.HookEventPreToolUse, hook)
```

## ✨ Benefits of New API

1. **Clear Structure** - All hook functionality in `pkg/claudesdk`
2. **Better Types** - Strong typing for hook inputs and outputs
3. **Comprehensive Events** - All hook events available as constants
4. **Timeout Support** - Built-in timeout protection

Perfect for building robust, controlled AI applications with custom behavior!
