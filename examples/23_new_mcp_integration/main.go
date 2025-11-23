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

	fmt.Println("🔗 Claude SDK - New MCP Integration API")
	fmt.Println("=====================================")

	// Example 1: MCP Basics
	fmt.Println("\n🛠️ Example 1: MCP Integration Basics")
	fmt.Println("-----------------------------------")
	mcpBasicsExample(ctx)

	// Example 2: MCP Configuration Options
	fmt.Println("\n⚙️ Example 2: MCP Configuration Options")
	fmt.Println("-------------------------------------")
	mcpConfigExample(ctx)

	fmt.Println("\n🎊 MCP Integration Demo Complete!")
}

func mcpBasicsExample(ctx context.Context) {
	fmt.Printf("  📋 MCP (Model Context Protocol) Integration:\n")
	fmt.Printf("    - Connect AI agents to external tools and data sources\n")
	fmt.Printf("    - SDK provides claudesdk.CreateSDKMcpServer() for custom servers\n")
	fmt.Printf("    - Use claudesdk.WithMcpServers() to configure client\n")
	
	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "Explain what MCP enables for AI applications."); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		return processResponse(ctx, result, "MCP Basics")
	},
		claudesdk.WithSystemPrompt("You are an AI expert explaining the Model Context Protocol clearly."),
	)

	if err != nil {
		log.Printf("MCP basics example failed: %v", err)
	} else {
		fmt.Printf("  ✅ MCP basics explained\n")
	}
}

func mcpConfigExample(ctx context.Context) {
	fmt.Printf("  🎛️ Available MCP configuration options:\n")
	fmt.Printf("    - claudesdk.WithMcpServers(serverMap)\n")
	fmt.Printf("    - claudesdk.CreateSDKMcpServer(config)\n")
	fmt.Printf("    - Server types: SDK servers, external servers\n")
	fmt.Printf("    - Tool definition with handlers\n")

	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "What are the benefits of using MCP servers vs direct tool integration?"); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		return processResponse(ctx, result, "MCP Configuration")
	},
		claudesdk.WithSystemPrompt("You are a software architect explaining integration patterns."),
	)

	if err != nil {
		log.Printf("MCP configuration example failed: %v", err)
	} else {
		fmt.Printf("  ✅ MCP configuration options demonstrated\n")
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

func processResponse(ctx context.Context, messages claudesdk.MessageIterator, label string) error {
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
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("    🤖 %s\n", content)
			
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
