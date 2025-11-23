package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
	ctx := context.Background()

	fmt.Println("🔍 Claude SDK - New Query Patterns")
	fmt.Println("==================================")

	// Example 1: Simple Query
	fmt.Println("\n📋 Example 1: Simple Query")
	fmt.Println("-------------------------")
	
	basicQuery(ctx)

	// Example 2: Query with Options
	fmt.Println("\n⚙️ Example 2: Query with Configuration")
	fmt.Println("------------------------------------")
	
	queryWithOptions(ctx)

	// Example 3: Query with Timeout
	fmt.Println("\n⏰ Example 3: Query with Timeout")
	fmt.Println("------------------------------")
	
	queryWithTimeout(ctx)

	// Example 4: Multiple Queries
	fmt.Println("\n🔄 Example 4: Multiple Queries")
	fmt.Println("-----------------------------")
	
	multipleQueries(ctx)

	fmt.Println("\n🎊 Query Patterns Demo Complete!")
}

func basicQuery(ctx context.Context) {
	messages, err := claudesdk.Query(ctx, "What's the capital of France?")
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	processMessages(ctx, messages, "Basic Query")
}

func queryWithOptions(ctx context.Context) {
	messages, err := claudesdk.Query(ctx, "Write a haiku about programming",
		claudesdk.WithSystemPrompt("You are a poetic coding assistant"),
		claudesdk.WithModel("claude-3-haiku-20240307"),
		claudesdk.WithCwd("/tmp"),
	)
	if err != nil {
		log.Printf("Query with options failed: %v", err)
		return
	}

	processMessages(ctx, messages, "Configured Query")
}

func queryWithTimeout(ctx context.Context) {
	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	messages, err := claudesdk.Query(timeoutCtx, "Explain quantum computing in simple terms",
		claudesdk.WithSystemPrompt("You are a science teacher. Keep explanations simple and engaging."),
	)
	if err != nil {
		log.Printf("Query with timeout failed: %v", err)
		return
	}

	processMessages(timeoutCtx, messages, "Timeout Query")
}

func multipleQueries(ctx context.Context) {
	queries := []struct {
		question string
		prompt   string
	}{
		{"What's 2+2?", "You are a math tutor"},
		{"Name a programming language", "You are a coding expert"},
		{"What's the weather like?", "You are a helpful assistant"},
	}

	for i, q := range queries {
		fmt.Printf("  Query %d: %s\n", i+1, q.question)
		
		messages, err := claudesdk.Query(ctx, q.question,
			claudesdk.WithSystemPrompt(q.prompt),
		)
		if err != nil {
			log.Printf("Query %d failed: %v", i+1, err)
			continue
		}

		// Just get first response for brevity
		if msg, err := messages.Next(ctx); err == nil {
			if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
				content := extractTextContent(assistantMsg.Content)
				fmt.Printf("  Answer %d: %s\n\n", i+1, truncate(content, 80))
			}
		}
	}
}

func processMessages(ctx context.Context, messages claudesdk.MessageIterator, label string) {
	fmt.Printf("  %s Response:\n", label)
	
	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			log.Printf("Error reading message: %v", err)
			break
		}

		if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
			content := extractTextContent(assistantMsg.Content)
			fmt.Printf("  Claude: %s\n", truncate(content, 100))
		}
	}
}

func extractTextContent(content []claudesdk.ContentBlock) string {
	var text strings.Builder
	for _, block := range content {
		if textBlock, ok := block.(*claudesdk.TextBlock); ok {
			text.WriteString(textBlock.Text)
			text.WriteString(" ")
		}
	}
	return strings.TrimSpace(text.String())
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
