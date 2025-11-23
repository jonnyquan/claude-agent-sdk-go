package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
	ctx := context.Background()

	fmt.Println("🪝 Claude SDK - New Hooks System API")
	fmt.Println("==================================")

	// Example 1: Basic Hook Configuration
	fmt.Println("\n🔧 Example 1: Basic Hook Configuration")
	fmt.Println("------------------------------------")
	basicHookExample(ctx)

	// Example 2: Hook Options Demo
	fmt.Println("\n⚙️ Example 2: Hook Configuration Options")
	fmt.Println("--------------------------------------")
	hookOptionsDemo(ctx)

	fmt.Println("\n🎊 Hooks System Demo Complete!")
}

func basicHookExample(ctx context.Context) {
	fmt.Printf("  📋 Demonstrating hook configuration with new API\n")

	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "Hello! Please help me understand the new hook system."); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		return processHookResponse(ctx, result, "Basic Hook")
	},
		claudesdk.WithSystemPrompt("You are a helpful assistant explaining the Claude SDK hook system."),
		// Note: Hook configuration would be added here with claudesdk.WithHook()
		// For this demo, we focus on the new API structure
	)

	if err != nil {
		log.Printf("Basic hook example failed: %v", err)
	} else {
		fmt.Printf("  ✅ Hook configuration demonstrated with new API\n")
	}
}

func hookOptionsDemo(ctx context.Context) {
	fmt.Printf("  🎛️ Available hook options in new API:\n")
	fmt.Printf("    - claudesdk.WithHook(event, matcher)\n")
	fmt.Printf("    - claudesdk.WithHooks(eventMap)\n")
	fmt.Printf("    - Hook events: PreToolUse, PostToolUse, Stop, etc.\n")
	fmt.Printf("    - Hook matchers with timeouts and callbacks\n")

	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "Explain what hooks are useful for in AI applications."); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		return processHookResponse(ctx, result, "Hook Options Demo")
	},
		claudesdk.WithSystemPrompt("You are an AI engineering expert. Explain hooks concisely."),
	)

	if err != nil {
		log.Printf("Hook options demo failed: %v", err)
	} else {
		fmt.Printf("  ✅ Hook options overview completed\n")
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

func processHookResponse(ctx context.Context, messages claudesdk.MessageIterator, label string) error {
	fmt.Printf("  📝 %s Response:\n", label)
	
	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			return fmt.Errorf("error reading message: %w", err)
		}

		switch m := msg.(type) {
		case *claudesdk.AssistantMessage:
			content := extractTextContent(m.Content)
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			fmt.Printf("    Claude: %s\n", content)
			
		case *claudesdk.UserMessage:
			if strContent, ok := m.Content.(string); ok {
				fmt.Printf("    👤 User: %s\n", strContent)
			} else {
				fmt.Printf("    👤 User: [complex content]\n")
			}
			
		case *claudesdk.ResultMessage:
			if m.Result != nil {
				fmt.Printf("    📊 Result: %s\n", *m.Result)
			}
			
		default:
			fmt.Printf("    📄 Message type: %s\n", msg.Type())
		}
	}
	fmt.Println()
	return nil
}
