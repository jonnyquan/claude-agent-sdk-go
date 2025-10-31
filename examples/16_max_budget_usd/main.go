package main

import (
	"context"
	"fmt"
	"log"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
	// Example 1: Without budget limit
	example1WithoutBudget()

	// Example 2: With reasonable budget
	example2WithReasonableBudget()

	// Example 3: With tight budget
	example3WithTightBudget()

	// Example 4: With max thinking tokens
	example4WithMaxThinkingTokens()
}

func example1WithoutBudget() {
	fmt.Println("=== Example 1: Without Budget Limit ===")

	ctx := context.Background()
	iter, err := claudecode.Query(ctx, "What is 2 + 2?")
	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printMessages(iter, ctx)
	fmt.Println()
}

func example2WithReasonableBudget() {
	fmt.Println("=== Example 2: With Reasonable Budget ($0.10) ===")

	ctx := context.Background()

	// Set max budget to $0.10 - plenty for a simple query
	iter, err := claudecode.Query(
		ctx,
		"What is 2 + 2?",
		claudecode.WithMaxBudgetUSD(0.10),
	)
	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printMessages(iter, ctx)
	fmt.Println()
}

func example3WithTightBudget() {
	fmt.Println("=== Example 3: With Tight Budget ($0.0001) ===")

	ctx := context.Background()

	// Set very small budget - will likely be exceeded
	iter, err := claudecode.Query(
		ctx,
		"Read the README.md file and summarize it",
		claudecode.WithMaxBudgetUSD(0.0001),
	)
	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printMessages(iter, ctx)

	fmt.Println("\n⚠️  Note: The cost may exceed the budget by up to one API call's worth")
	fmt.Println("    because budget checking happens after each API call completes.")
	fmt.Println()
}

func example4WithMaxThinkingTokens() {
	fmt.Println("=== Example 4: With Max Thinking Tokens (5000) ===")

	ctx := context.Background()

	// Limit thinking tokens for faster response and lower cost
	iter, err := claudecode.Query(
		ctx,
		"Explain the concept of recursion",
		claudecode.WithMaxThinkingTokens(5000),
	)
	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printMessages(iter, ctx)

	fmt.Println("\n💡 Lower max_thinking_tokens values result in:")
	fmt.Println("   - Faster responses")
	fmt.Println("   - Lower costs")
	fmt.Println("   - But may reduce reasoning quality")
	fmt.Println()
}

func printMessages(iter claudecode.MessageIterator, ctx context.Context) {
	for {
		msg, err := iter.Next(ctx)
		if err != nil {
			if err == claudecode.ErrNoMoreMessages {
				break
			}
			log.Printf("Error reading message: %v\n", err)
			break
		}

		switch m := msg.(type) {
		case *claudecode.AssistantMessage:
			fmt.Printf("Claude: ")
			for _, block := range m.Content {
				switch b := block.(type) {
				case *claudecode.TextBlock:
					fmt.Printf("%s ", b.Text)
				case *claudecode.ToolUseBlock:
					fmt.Printf("[Using tool: %s] ", b.Name)
				}
			}
			fmt.Println()

		case *claudecode.ResultMessage:
			// Check if budget was exceeded
			if m.Subtype == "error_max_budget_usd" {
				fmt.Println("\n⚠️  Budget limit exceeded!")
				if m.TotalCostUSD != nil {
					fmt.Printf("Total cost: $%.4f\n", *m.TotalCostUSD)
				}
			} else {
				if m.TotalCostUSD != nil {
					fmt.Printf("Total cost: $%.4f\n", *m.TotalCostUSD)
				}
				if m.Subtype != "" {
					fmt.Printf("Status: %s\n", m.Subtype)
				}
			}
		}
	}
}

/*
Example Output:

=== Example 1: Without Budget Limit ===
Claude: 2 + 2 equals 4.
Total cost: $0.0015
Status: success

=== Example 2: With Reasonable Budget ($0.10) ===
Claude: 2 + 2 equals 4.
Total cost: $0.0015
Status: success

=== Example 3: With Tight Budget ($0.0001) ===
[Using tool: Read]
⚠️  Budget limit exceeded!
Total cost: $0.0002

⚠️  Note: The cost may exceed the budget by up to one API call's worth
    because budget checking happens after each API call completes.

=== Example 4: With Max Thinking Tokens (5000) ===
Claude: Recursion is when a function calls itself...
Total cost: $0.0018
Status: success

💡 Lower max_thinking_tokens values result in:
   - Faster responses
   - Lower costs
   - But may reduce reasoning quality

Key Features Demonstrated:

1. Budget Control (max_budget_usd):
   - Set API cost limits to prevent unexpected charges
   - Useful for development, testing, and cost management
   - CLI stops execution when budget is exceeded
   - Returns error_max_budget_usd result type

2. Thinking Token Limit (max_thinking_tokens):
   - Control extended thinking (reasoning) token usage
   - Balance between reasoning quality and cost
   - Lower values = faster + cheaper responses
   - Higher values = better reasoning

3. Cost Tracking:
   - Every ResultMessage includes total_cost_usd field
   - Track spending across multiple API calls
   - Monitor budget usage in real-time

4. Error Handling:
   - Check for error_max_budget_usd subtype
   - Handle budget exceeded gracefully
   - Continue or retry with adjusted budget

Use Cases:

- Development/Testing: Set low budgets to prevent runaway costs
- Production: Set reasonable budgets for each request type
- Cost Analysis: Track and optimize API spending
- Performance Tuning: Balance speed vs reasoning quality

API Reference:

WithMaxBudgetUSD(budget float64):
    Sets the maximum budget in USD for API costs.
    When exceeded, returns error_max_budget_usd result.
    
WithMaxThinkingTokens(tokens int):
    Sets the maximum tokens for thinking blocks.
    Controls extended thinking depth and quality.

Note: Budget checking happens AFTER each API call, so final cost
may slightly exceed the specified budget by up to one call's worth.
*/
