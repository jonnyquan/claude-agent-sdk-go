# Budget Control and Thinking Token Limit Example

This example demonstrates two important cost control features:
1. **max_budget_usd**: Limit API costs
2. **max_thinking_tokens**: Control reasoning depth

---

## Features

### 1. Budget Control (`max_budget_usd`)

**Purpose**: Prevent unexpected API costs by setting a maximum budget.

**How it works**:
- Set a budget limit in USD
- Claude CLI tracks cumulative cost
- Stops execution when budget is exceeded
- Returns `error_max_budget_usd` result

**Example**:
```go
iter, err := claudecode.Query(
    ctx,
    "Analyze this data",
    claudecode.WithMaxBudgetUSD(0.10), // $0.10 limit
)
```

**Use Cases**:
- 🧪 Development & Testing (prevent runaway costs)
- 💰 Production Cost Control
- 📊 Budget Management
- 🎯 Per-Request Limits

---

### 2. Thinking Token Limit (`max_thinking_tokens`)

**Purpose**: Control the depth of model's reasoning (extended thinking).

**How it works**:
- Set maximum tokens for thinking blocks
- Lower = faster + cheaper responses
- Higher = better reasoning quality
- Default: 8000 tokens

**Example**:
```go
iter, err := claudecode.Query(
    ctx,
    "Solve this problem",
    claudecode.WithMaxThinkingTokens(5000), // Limit thinking
)
```

**Use Cases**:
- ⏱️ Fast responses for simple tasks
- 💰 Cost optimization
- 🎯 Task-specific tuning

---

## Running the Example

```bash
cd examples/16_max_budget_usd
go run main.go
```

---

## Example Scenarios

### Scenario 1: No Budget Limit
```go
// No restrictions - use as much as needed
iter, err := claudecode.Query(ctx, "What is 2 + 2?")
```

**Expected**: Normal operation, full cost tracking

---

### Scenario 2: Reasonable Budget
```go
// $0.10 budget - plenty for simple queries
iter, err := claudecode.Query(
    ctx,
    "What is 2 + 2?",
    claudecode.WithMaxBudgetUSD(0.10),
)
```

**Expected**: Success with cost info

---

### Scenario 3: Tight Budget
```go
// Very small budget - will be exceeded
iter, err := claudecode.Query(
    ctx,
    "Read and summarize README",
    claudecode.WithMaxBudgetUSD(0.0001),
)
```

**Expected**: Budget exceeded error

**Output**:
```
⚠️  Budget limit exceeded!
Total cost: $0.0002
Status: error_max_budget_usd
```

---

### Scenario 4: Limited Thinking
```go
// Limit thinking for faster response
iter, err := claudecode.Query(
    ctx,
    "Explain recursion",
    claudecode.WithMaxThinkingTokens(5000),
)
```

**Expected**: Faster response, lower cost

---

## Cost Tracking

Every `ResultMessage` includes cost information:

```go
case *claudecode.ResultMessage:
    if m.TotalCostUSD != nil {
        fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
    }
    
    if m.Subtype != nil && *m.Subtype == "error_max_budget_usd" {
        fmt.Println("Budget exceeded!")
    }
```

---

## Important Notes

### Budget Checking Timing

⚠️ **Budget is checked AFTER each API call completes**

This means:
- Final cost may slightly exceed budget
- Overage is at most one API call's worth
- This is unavoidable due to streaming nature

**Example**:
```
Budget:    $0.0001
Call 1:    $0.0001  ✅ Within budget
Call 2:    $0.0002  ⚠️ Exceeds budget, but call already made
Final:     $0.0002  (exceeded by $0.0001)
```

### Thinking Tokens Impact

**Lower Values (1000-3000)**:
- ✅ Faster responses
- ✅ Lower costs
- ❌ Simpler reasoning

**Medium Values (5000-8000)**:
- ⚖️ Balanced speed/quality
- ⚖️ Moderate costs
- ✅ Good for most tasks

**Higher Values (10000+)**:
- ❌ Slower responses
- ❌ Higher costs
- ✅ Complex reasoning

---

## API Reference

### WithMaxBudgetUSD

```go
func WithMaxBudgetUSD(budget float64) Option
```

**Parameters**:
- `budget`: Maximum budget in USD (e.g., 0.10 for $0.10)

**Returns**: Option function

**Example**:
```go
claudecode.WithMaxBudgetUSD(0.50) // $0.50 limit
```

---

### WithMaxThinkingTokens

```go
func WithMaxThinkingTokens(tokens int) Option
```

**Parameters**:
- `tokens`: Maximum tokens for thinking blocks

**Returns**: Option function

**Example**:
```go
claudecode.WithMaxThinkingTokens(5000) // Limit to 5000 tokens
```

---

## Error Handling

### Check for Budget Exceeded

```go
for {
    msg, err := iter.Next(ctx)
    if err != nil {
        if err == claudecode.ErrNoMoreMessages {
            break
        }
        log.Printf("Error: %v", err)
        break
    }

    if result, ok := msg.(*claudecode.ResultMessage); ok {
        if result.Subtype != nil && *result.Subtype == "error_max_budget_usd" {
            // Handle budget exceeded
            fmt.Println("Budget exceeded!")
            if result.TotalCostUSD != nil {
                fmt.Printf("Total cost: $%.4f\n", *result.TotalCostUSD)
            }
        }
    }
}
```

---

## Best Practices

### 1. Set Appropriate Budgets

```go
// Development/Testing
claudecode.WithMaxBudgetUSD(0.01) // Very low

// Simple queries
claudecode.WithMaxBudgetUSD(0.10) // Low

// Complex tasks
claudecode.WithMaxBudgetUSD(1.00) // Higher

// Production (no limit)
// Don't use WithMaxBudgetUSD
```

### 2. Tune Thinking Tokens by Task

```go
// Simple calculations
claudecode.WithMaxThinkingTokens(2000) // Fast

// General tasks
claudecode.WithMaxThinkingTokens(5000) // Balanced

// Complex reasoning
claudecode.WithMaxThinkingTokens(10000) // Deep
```

### 3. Monitor Costs

```go
totalCost := 0.0

for {
    msg, err := iter.Next(ctx)
    // ... handle errors ...

    if result, ok := msg.(*claudecode.ResultMessage); ok {
        if result.TotalCostUSD != nil {
            totalCost = *result.TotalCostUSD
            log.Printf("Current cost: $%.4f", totalCost)
        }
    }
}

log.Printf("Final cost: $%.4f", totalCost)
```

---

## Troubleshooting

### Budget Always Exceeded?

**Problem**: Even simple queries exceed tiny budgets

**Solution**: 
- Minimum meaningful budget: ~$0.001
- Single API call costs: $0.0001 - $0.01
- Set budget at least 2-3x expected cost

### Thinking Tokens Not Working?

**Problem**: Setting max_thinking_tokens has no effect

**Solution**:
- Only affects models with extended thinking
- Check if your model supports thinking blocks
- Use Claude 3.5 Sonnet or Opus models

### Cost Still High?

**Problem**: Budget limits not reducing costs

**Solution**:
- Budget limit stops execution, doesn't reduce per-call costs
- Also use WithMaxThinkingTokens to reduce reasoning costs
- Consider using a less expensive model

---

## See Also

- [Query API](../01_simple_query/)
- [Client API](../02_client_streaming/)
- [SDK MCP Server](../15_sdk_mcp_server/)
- [Cost Optimization Guide](../../docs/cost-optimization.md)

---

## Summary

This example demonstrates:
- ✅ Setting API cost budgets
- ✅ Limiting thinking token usage
- ✅ Tracking costs in real-time
- ✅ Handling budget exceeded errors
- ✅ Balancing speed, cost, and quality

**Key Takeaway**: Use `WithMaxBudgetUSD` for cost control and `WithMaxThinkingTokens` for performance tuning.
