# Example 24: New Error Handling API

This example demonstrates comprehensive error handling patterns using the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **Basic Error Handling** - Handle common configuration and connection errors
2. **Timeout Handling** - Manage time-based operation limits
3. **Context Cancellation** - Handle user-initiated cancellations
4. **Graceful Error Recovery** - Implement fallback strategies
5. **Advanced Error Inspection** - Deep error analysis and classification

## 🚀 Key Features Demonstrated

### Error Types & Handling
```go
messages, err := claudesdk.Query(ctx, prompt, options...)
if err != nil {
    if isTimeoutError(err) {
        // Handle timeout
    } else if isConnectionError(err) {
        // Handle connection issues
    } else if isConfigurationError(err) {
        // Handle config problems
    }
}
```

### Context-Based Operations
```go
// With timeout
timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// With cancellation
cancelCtx, cancel := context.WithCancel(ctx)
defer cancel()

messages, err := claudesdk.Query(timeoutCtx, prompt)
```

### Graceful Recovery
```go
strategies := []func() (claudesdk.MessageIterator, error){
    func() { return claudesdk.Query(ctx, prompt, claudesdk.WithModel("primary")) },
    func() { return claudesdk.Query(ctx, prompt, claudesdk.WithModel("fallback")) },
    func() { return claudesdk.Query(ctx, prompt) }, // default config
}

for _, strategy := range strategies {
    if result, err := strategy(); err == nil {
        return result // Success!
    }
}
```

## 🔧 Running the Example

```bash
cd examples/24_new_error_handling
go run main.go
```

## 💡 Error Handling Patterns

### 1. **Timeout Management**
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

messages, err := claudesdk.Query(ctx, "Long operation...")
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Operation timed out")
}
```

### 2. **Retry Logic**
```go
func queryWithRetry(ctx context.Context, prompt string, maxRetries int) (claudesdk.MessageIterator, error) {
    for i := 0; i < maxRetries; i++ {
        result, err := claudesdk.Query(ctx, prompt)
        if err == nil {
            return result, nil
        }
        
        if isRetryableError(err) && i < maxRetries-1 {
            time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
            continue
        }
        
        return nil, err
    }
    return nil, errors.New("max retries exceeded")
}
```

### 3. **Error Wrapping**
```go
func processQuery(ctx context.Context, prompt string) error {
    messages, err := claudesdk.Query(ctx, prompt)
    if err != nil {
        return fmt.Errorf("failed to process query %q: %w", prompt, err)
    }
    
    for {
        msg, err := messages.Next(ctx)
        if err != nil {
            if err == claudesdk.ErrNoMoreMessages {
                break
            }
            return fmt.Errorf("failed to read message: %w", err)
        }
        // Process message
    }
    
    return nil
}
```

### 4. **Circuit Breaker Pattern**
```go
type CircuitBreaker struct {
    failures    int
    lastFailure time.Time
    threshold   int
    timeout     time.Duration
}

func (cb *CircuitBreaker) Call(ctx context.Context, prompt string) (claudesdk.MessageIterator, error) {
    if cb.failures >= cb.threshold && time.Since(cb.lastFailure) < cb.timeout {
        return nil, errors.New("circuit breaker open")
    }
    
    result, err := claudesdk.Query(ctx, prompt)
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        return nil, err
    }
    
    cb.failures = 0 // Reset on success
    return result, nil
}
```

## 🚨 Common Error Scenarios

### Configuration Errors
```go
// Invalid model
claudesdk.Query(ctx, prompt, claudesdk.WithModel("invalid-model"))

// Invalid working directory
claudesdk.Query(ctx, prompt, claudesdk.WithCwd("/nonexistent"))

// Invalid options combination
claudesdk.Query(ctx, prompt, 
    claudesdk.WithMaxThinkingTokens(-1),
    claudesdk.WithMaxBudgetUSD(-1.0))
```

### Runtime Errors
```go
// Network/connection issues
claudesdk.Query(ctx, prompt) // When CLI isn't available

// Timeout scenarios
shortCtx, _ := context.WithTimeout(ctx, 1*time.Millisecond)
claudesdk.Query(shortCtx, "Complex query...")

// Cancellation
cancelCtx, cancel := context.WithCancel(ctx)
cancel() // Cancel immediately
claudesdk.Query(cancelCtx, prompt)
```

## 🔍 Error Classification

The example demonstrates how to classify errors:

### Connection Errors
- CLI not found
- Network issues
- Process spawn failures

### Configuration Errors  
- Invalid options
- Bad model names
- Invalid paths

### Runtime Errors
- Timeouts
- Cancellations
- Authentication failures

### Message Processing Errors
- Malformed responses
- Parsing failures
- Protocol errors

## 📋 Best Practices

### 1. **Always Use Context**
```go
// Good
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
claudesdk.Query(ctx, prompt)

// Avoid
claudesdk.Query(context.Background(), prompt) // No timeout protection
```

### 2. **Handle Specific Errors**
```go
// Good
if errors.Is(err, context.DeadlineExceeded) {
    return handleTimeout()
} else if isConnectionError(err) {
    return handleConnectionError()
}

// Avoid generic handling
if err != nil {
    return err // Too generic
}
```

### 3. **Implement Graceful Degradation**
```go
// Try premium features first, fall back to basic
result, err := queryWithAdvancedOptions(ctx, prompt)
if err != nil {
    result, err = queryWithBasicOptions(ctx, prompt)
}
```

### 4. **Log Errors Appropriately**
```go
if err != nil {
    log.Printf("Query failed: %v", err)
    if isTemporaryError(err) {
        log.Println("This appears to be a temporary issue, retrying...")
    }
}
```

## ✨ Benefits of New API

1. **Consistent Error Types** - All errors follow Go conventions
2. **Context Integration** - Full context.Context support throughout
3. **Better Error Messages** - More descriptive error information
4. **Structured Errors** - Easy to classify and handle programmatically

Perfect for building robust, production-ready applications with comprehensive error handling!
