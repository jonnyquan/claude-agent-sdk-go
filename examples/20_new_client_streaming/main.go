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

	fmt.Println("💬 Claude SDK - New Client Streaming API")
	fmt.Println("=======================================")

	// Example 1: Manual Client Management
	fmt.Println("\n🔧 Example 1: Manual Client Management")
	fmt.Println("------------------------------------")
	manualClientExample(ctx)

	// Example 2: WithClient Pattern (Recommended)
	fmt.Println("\n✨ Example 2: WithClient Pattern (Recommended)")
	fmt.Println("--------------------------------------------")
	withClientExample(ctx)

	// Example 3: Multi-turn Conversation
	fmt.Println("\n🔄 Example 3: Multi-turn Conversation")
	fmt.Println("-----------------------------------")
	multiTurnExample(ctx)

	// Example 4: Client with Advanced Options
	fmt.Println("\n⚙️ Example 4: Client with Advanced Options")
	fmt.Println("-----------------------------------------")
	advancedClientExample(ctx)

	fmt.Println("\n🎊 Client Streaming Demo Complete!")
}

func manualClientExample(ctx context.Context) {
	// Create client with new API
	client := claudesdk.NewClient(
		claudesdk.WithSystemPrompt("You are a helpful coding assistant"),
		claudesdk.WithModel("claude-3-sonnet-20241022"),
	)
	
	// Manual lifecycle management
	if err := client.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			log.Printf("Failed to disconnect: %v", err)
		}
	}()

	// Send query and receive response
	if err := client.Query(ctx, "Write a simple Python hello world program"); err != nil {
		log.Printf("Failed to send query: %v", err)
		return
	}

	// Process streaming response
	result := client.ReceiveResponse(ctx)
	processStreamingResponse(ctx, result, "Manual Client")
}

func withClientExample(ctx context.Context) {
	// WithClient pattern - automatic resource management
	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		// Send query
		if err := client.Query(ctx, "Explain what a REST API is in one sentence"); err != nil {
			return err
		}

		// Process response
		result := client.ReceiveResponse(ctx)
		processStreamingResponse(ctx, result, "WithClient")
		return nil
	}, 
		claudesdk.WithSystemPrompt("You are a web development expert"),
		claudesdk.WithModel("claude-3-haiku-20240307"),
	)
	
	if err != nil {
		log.Printf("WithClient failed: %v", err)
	}
}

func multiTurnExample(ctx context.Context) {
	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		// First turn
		fmt.Println("  👤 User: What's your favorite programming language?")
		if err := client.Query(ctx, "What's your favorite programming language?"); err != nil {
			return err
		}
		
		result := client.ReceiveResponse(ctx)
		processStreamingResponse(ctx, result, "Turn 1")

		// Small delay for realistic conversation
		time.Sleep(1 * time.Second)

		// Second turn  
		fmt.Println("  👤 User: Why do you like it?")
		if err := client.QueryWithSession(ctx, "Why do you like it?", "conversation-1"); err != nil {
			return err
		}

		result = client.ReceiveResponse(ctx)
		processStreamingResponse(ctx, result, "Turn 2")

		return nil
	},
		claudesdk.WithSystemPrompt("You are a friendly programming tutor. Keep responses concise but informative."),
	)

	if err != nil {
		log.Printf("Multi-turn conversation failed: %v", err)
	}
}

func advancedClientExample(ctx context.Context) {
	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "List 3 benefits of using Go for backend development"); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		processStreamingResponse(ctx, result, "Advanced Options")
		return nil
	},
		claudesdk.WithSystemPrompt("You are a Go expert. Provide practical, actionable advice."),
		claudesdk.WithModel("claude-3-sonnet-20241022"),
		claudesdk.WithCwd("/tmp"),
		claudesdk.WithMaxThinkingTokens(1000),
	)

	if err != nil {
		log.Printf("Advanced client failed: %v", err)
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

func processStreamingResponse(ctx context.Context, messages claudesdk.MessageIterator, label string) {
	fmt.Printf("  🤖 %s Response:\n", label)
	
	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			log.Printf("Error reading message: %v", err)
			break
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			// Format output nicely
			content := extractTextContent(m.Content)
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("     %s\n", content)
			
		case *claudesdk.UserMessage:
			fmt.Printf("  👤 User: %s\n", m.Content)
			
		default:
			fmt.Printf("  ℹ️  Other message type: %T\n", m)
		}
	}
	fmt.Println()
}
