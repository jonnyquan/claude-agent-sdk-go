package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
	// Example 1: Simple text tool
	example1SimpleTextTool()

	// Example 2: Tool with image content
	example2ImageContent()

	// Example 3: Multiple tools in one server
	example3MultipleTool()

	// Example 4: Tool with application state
	example4WithState()
}

func example1SimpleTextTool() {
	fmt.Println("=== Example 1: Simple Text Tool ===")

	ctx := context.Background()

	// Define a greet tool
	greet := &claudecode.ToolDef{
		Name:        "greet",
		Description: "Greet a user by name",
		InputSchema: map[string]interface{}{
			"name": "string",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			name, ok := args["name"].(string)
			if !ok {
				name = "User"
			}
			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("Hello, %s! 👋", name)),
			}, nil
		},
	}

	// Create SDK MCP server
	server := claudecode.CreateSDKMcpServer("greeting-tools", "1.0.0", greet)

	// Use with SDK
	iter, err := claudecode.Query(
		ctx,
		"Greet Alice",
		claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
			"greeting": server,
		}),
		claudecode.WithAllowedTools("greet"),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printIterator(iter)
}

func example2ImageContent() {
	fmt.Println("\n=== Example 2: Image Content ===")

	ctx := context.Background()

	// Create a sample image (base64 encoded test image)
	pngData := createSampleImage()

	// Define a tool that returns both text and image
	generateChart := claudecode.Tool(
		"generate_chart",
		"Generate a chart and return it as an image",
		map[string]interface{}{
			"type":  "string",
			"title": "string",
		},
		func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			chartType := args["type"]
			title := args["title"]

			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("Generated %s chart: %s", chartType, title)),
				claudecode.NewImageContent(pngData, "image/png"),
			}, nil
		},
	)

	// Create server
	server := claudecode.CreateSDKMcpServer("chart-tools", "1.0.0", generateChart)

	// Use with SDK
	iter, err := claudecode.Query(
		ctx,
		"Generate a bar chart titled 'Sales Data'",
		claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
			"charts": server,
		}),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printIterator(iter)
}

func example3MultipleTool() {
	fmt.Println("\n=== Example 3: Multiple Tools ===")

	ctx := context.Background()

	// Calculator tools
	add := &claudecode.ToolDef{
		Name:        "add",
		Description: "Add two numbers",
		InputSchema: map[string]interface{}{
			"a": "number",
			"b": "number",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			result := a + b
			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("%.2f + %.2f = %.2f", a, b, result)),
			}, nil
		},
	}

	multiply := &claudecode.ToolDef{
		Name:        "multiply",
		Description: "Multiply two numbers",
		InputSchema: map[string]interface{}{
			"a": "number",
			"b": "number",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			result := a * b
			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("%.2f × %.2f = %.2f", a, b, result)),
			}, nil
		},
	}

	// Create server with multiple tools
	server := claudecode.CreateSDKMcpServer("calculator", "2.0.0", add, multiply)

	// Use with SDK
	iter, err := claudecode.Query(
		ctx,
		"Add 15 and 27, then multiply the result by 3",
		claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
			"calc": server,
		}),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printIterator(iter)
}

// Simple data store to demonstrate state access
type DataStore struct {
	items []string
}

func example4WithState() {
	fmt.Println("\n=== Example 4: Tool with Application State ===")

	ctx := context.Background()

	// Create application state
	store := &DataStore{
		items: []string{},
	}

	// Define tools that access application state
	addItem := &claudecode.ToolDef{
		Name:        "add_item",
		Description: "Add an item to the store",
		InputSchema: map[string]interface{}{
			"item": "string",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			item := args["item"].(string)
			store.items = append(store.items, item)
			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("Added '%s' to store. Total items: %d", item, len(store.items))),
			}, nil
		},
	}

	listItems := &claudecode.ToolDef{
		Name:        "list_items",
		Description: "List all items in the store",
		InputSchema: map[string]interface{}{},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			if len(store.items) == 0 {
				return []claudecode.ToolContent{
					claudecode.NewTextContent("Store is empty"),
				}, nil
			}
			itemList := ""
			for i, item := range store.items {
				itemList += fmt.Sprintf("%d. %s\n", i+1, item)
			}
			return []claudecode.ToolContent{
				claudecode.NewTextContent(fmt.Sprintf("Store contains %d items:\n%s", len(store.items), itemList)),
			}, nil
		},
	}

	// Create server
	server := claudecode.CreateSDKMcpServer("store-tools", "1.0.0", addItem, listItems)

	// Use with SDK
	iter, err := claudecode.Query(
		ctx,
		"Add 'apple', 'banana', and 'orange' to the store, then list all items",
		claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
			"store": server,
		}),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printIterator(iter)
}

func printIterator(iter claudecode.MessageIterator) {
	for {
		msg, err := iter.Next(context.Background())
		if err != nil {
			if err == claudecode.ErrNoMoreMessages {
				break
			}
			log.Printf("Error reading message: %v\n", err)
			break
		}

		switch m := msg.(type) {
		case *claudecode.AssistantMessage:
			fmt.Printf("\nAssistant: ")
			for _, block := range m.Content {
				switch b := block.(type) {
				case *claudecode.TextBlock:
					fmt.Printf("%s ", b.Text)
				case *claudecode.ToolUseBlock:
					fmt.Printf("[Using tool: %s] ", b.Name)
				case *claudecode.ToolResultBlock:
					if b.Content != nil {
						if contentStr, ok := b.Content.(string); ok {
							fmt.Printf("[Tool result: %s] ", contentStr)
						}
					}
				}
			}
			fmt.Println()
		case *claudecode.ResultMessage:
			if m.Result != nil {
				fmt.Printf("\nResult: %s\n", *m.Result)
			}
		}
	}
}

/*
Complete Working Example:

This example demonstrates all features of SDK MCP servers:

1. Tool Definition:
   - Simple parameter schemas
   - Type-safe input handling
   - Flexible return types

2. Content Types:
   - Text content
   - Image content (base64 encoded)

3. State Access:
   - Tools can access application state
   - Direct variable access (no IPC)

4. Multiple Tools:
   - Multiple tools in one server
   - Tool composition

5. Server Creation:
   - Simple API
   - Automatic registration
   - Version management

Comparison with External MCP Servers:

SDK MCP Server (In-Process):
+ No subprocess overhead
+ Direct state access
+ Easier debugging
+ Single binary deployment
- Go code only

External MCP Server (Stdio):
+ Language agnostic
+ Process isolation
+ Separate lifecycle
- IPC overhead
- More complex setup

When to Use SDK MCP Servers:
- Need direct access to application state
- Performance critical
- Simple deployment requirements
- Go-only environment

When to Use External MCP Servers:
- Multi-language tools
- Reusable across applications
- Process isolation required
- Complex tool ecosystems
*/

// createSampleImage creates a base64 encoded sample PNG image
// This generates a 1x1 pixel PNG programmatically to avoid hardcoded base64 strings
func createSampleImage() string {
	// PNG file structure for 1x1 red pixel
	// Header, IHDR chunk, IDAT chunk, IEND chunk
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk  
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	return base64.StdEncoding.EncodeToString(png)
}

// Example: Advanced tool with context timeout
func exampleWithTimeout() {
	fmt.Println("\n=== Example: Tool with Timeout ===")

	longRunning := &claudecode.ToolDef{
		Name:        "process_data",
		Description: "Process data with timeout",
		InputSchema: map[string]interface{}{
			"data": "string",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]claudecode.ToolContent, error) {
			// Respect context timeout
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return []claudecode.ToolContent{
					claudecode.NewTextContent("Data processed successfully"),
				}, nil
			}
		},
	}

	server := claudecode.CreateSDKMcpServer("processing", "1.0.0", longRunning)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	iter, err := claudecode.Query(
		ctx,
		"Process some data",
		claudecode.WithMcpServers(map[string]claudecode.McpServerConfig{
			"proc": server,
		}),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	printIterator(iter)
}
