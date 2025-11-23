package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚀 Claude Agent SDK - New API Demo")
	fmt.Println("===================================")

	// Example 1: Simple Query with new API
	fmt.Println("\n📋 Example 1: Simple Query")
	fmt.Println("--------------------------")
	
	messages, err := claudesdk.Query(ctx, "What's 2+2? Please be brief.")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			log.Fatalf("Error reading message: %v", err)
		}

		if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
			fmt.Printf("Claude: %s\n", assistantMsg.Content)
		}
	}

	// Example 2: Client with new API
	fmt.Println("\n💬 Example 2: Client with Options")
	fmt.Println("----------------------------------")
	
	client := claudesdk.NewClient(
		claudesdk.WithSystemPrompt("You are a helpful math tutor. Be encouraging."),
		claudesdk.WithModel("claude-3-sonnet-20241022"),
	)
	defer client.Disconnect()

	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect client: %v", err)
	}

	if err := client.Query(ctx, "What's the square root of 16?"); err != nil {
		log.Fatalf("Failed to send query: %v", err)
	}

	result := client.ReceiveResponse(ctx)

	for {
		msg, err := result.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			log.Fatalf("Error reading response: %v", err)
		}

		if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
			fmt.Printf("Claude: %s\n", assistantMsg.Content)
		}
	}

	// Example 3: WithClient pattern (recommended)
	fmt.Println("\n✨ Example 3: WithClient Pattern")
	fmt.Println("-------------------------------")
	
	err = claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		// Send a query
		if err := client.Query(ctx, "Tell me a fun fact about mathematics in one sentence."); err != nil {
			return err
		}

		// Read the response
		result := client.ReceiveResponse(ctx)
		for {
			msg, err := result.Next(ctx)
			if err != nil {
				if err == claudesdk.ErrNoMoreMessages {
					break
				}
				return err
			}

			if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
				fmt.Printf("Claude: %s\n", assistantMsg.Content)
			}
		}

		return nil
	}, claudesdk.WithSystemPrompt("You are a fun math teacher."))
	
	if err != nil {
		log.Fatalf("WithClient failed: %v", err)
	}

	fmt.Println("\n🎊 New API Demo Complete!")
	fmt.Println("========================")
	fmt.Println("✅ All examples used the new pkg/claudesdk API")
	fmt.Println("📦 Import path: github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk")
	fmt.Println("🔄 For backward compatibility, old import still works!")
}
