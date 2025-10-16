package main

import (
	"context"
	"encoding/base64"
	"fmt"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

// Example demonstrating image content support in messages
// This shows how custom tools can return images (charts, screenshots, etc.)
func main() {
	fmt.Println("=== Image Content Example ===")
	fmt.Println("Demonstrating how to work with image content blocks\n")

	// Create a simple 1x1 pixel PNG for demonstration
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x09, 0x70, 0x48, 0x59, 0x73, 0x00, 0x00, 0x0b, 0x13, 0x00, 0x00, 0x0b,
		0x13, 0x01, 0x00, 0x9a, 0x9c, 0x18, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44,
		0x41, 0x54, 0x78, 0x9c, 0x63, 0x60, 0x60, 0x60, 0x00, 0x00, 0x00, 0x04,
		0x00, 0x01, 0x5d, 0x55, 0x21, 0x1c, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
		0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	pngData := base64.StdEncoding.EncodeToString(pngBytes)

	// Example 1: Create an ImageBlock
	fmt.Println("1. Creating an ImageBlock:")
	imageBlock := &claudecode.ImageBlock{
		Data:     pngData,
		MimeType: "image/png",
	}
	fmt.Printf("   - Type: %s\n", imageBlock.BlockType())
	fmt.Printf("   - MIME: %s\n", imageBlock.MimeType)
	fmt.Printf("   - Data length: %d bytes (base64)\n\n", len(imageBlock.Data))

	// Example 2: Create a message with mixed content (text + image)
	fmt.Println("2. Creating AssistantMessage with text and image:")
	textBlock := &claudecode.TextBlock{
		Text: "Here is the generated chart:",
	}

	message := &claudecode.AssistantMessage{
		Content: []claudecode.ContentBlock{textBlock, imageBlock},
		Model:   "claude-sonnet-4-5",
	}

	fmt.Printf("   - Content blocks: %d\n", len(message.Content))
	for i, block := range message.Content {
		fmt.Printf("   - Block %d: %s\n", i+1, block.BlockType())
	}
	fmt.Println()

	// Example 3: Processing messages with image content
	fmt.Println("3. Processing message content:")
	for _, block := range message.Content {
		switch b := block.(type) {
		case *claudecode.TextBlock:
			fmt.Printf("   [Text] %s\n", b.Text)
		case *claudecode.ImageBlock:
			fmt.Printf("   [Image] Type: %s, Size: %d bytes\n", b.MimeType, len(b.Data))
		}
	}
	fmt.Println()

	// Example 4: Demonstrate use case - Chart generation tool response
	fmt.Println("4. Use Case: Chart Generation Tool Response")
	fmt.Println("   When a custom tool generates a chart, it can return:")
	chartResponse := &claudecode.AssistantMessage{
		Content: []claudecode.ContentBlock{
			&claudecode.TextBlock{
				Text: "Generated sales chart for Q4 2024",
			},
			&claudecode.ImageBlock{
				Data:     pngData,
				MimeType: "image/png",
			},
		},
		Model: "claude-sonnet-4-5",
	}

	fmt.Println("   Response structure:")
	for i, block := range chartResponse.Content {
		fmt.Printf("   %d. %s\n", i+1, block.BlockType())
	}
	fmt.Println()

	// Example 5: Supported image formats
	fmt.Println("5. Supported Image Formats:")
	supportedFormats := []string{
		"image/png",
		"image/jpeg",
		"image/gif",
		"image/webp",
	}
	for _, format := range supportedFormats {
		fmt.Printf("   ✓ %s\n", format)
	}
	fmt.Println()

	// Example 6: Real-world integration example
	fmt.Println("6. Real-World Integration Example:")
	fmt.Println(`   // In a custom MCP tool:
   func GenerateChart(ctx context.Context, args map[string]any) ([]claudecode.ContentBlock, error) {
       // 1. Generate chart using your favorite library
       chartImage := createChart(args["title"].(string))
       
       // 2. Encode to base64
       base64Data := base64.StdEncoding.EncodeToString(chartImage)
       
       // 3. Return mixed content
       return []claudecode.ContentBlock{
           &claudecode.TextBlock{
               Text: "Chart generated successfully",
           },
           &claudecode.ImageBlock{
               Data:     base64Data,
               MimeType: "image/png",
           },
       }, nil
   }`)
	fmt.Println()

	fmt.Println("✅ Image content support demonstration complete!")
	fmt.Println()
	fmt.Println("Key Points:")
	fmt.Println("  • ImageBlock supports base64-encoded image data")
	fmt.Println("  • Can mix text and images in single message")
	fmt.Println("  • Useful for charts, screenshots, diagrams")
	fmt.Println("  • Works with custom MCP tools")
}

// simulateToolResponse simulates a custom tool that returns image content
func simulateToolResponse() []claudecode.ContentBlock {
	// In real implementation, this would:
	// 1. Generate actual chart/image
	// 2. Encode to base64
	// 3. Return as ImageBlock

	// For demo, using minimal PNG
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		// ... (simplified for example)
	}

	return []claudecode.ContentBlock{
		&claudecode.TextBlock{
			Text: "Generated visualization",
		},
		&claudecode.ImageBlock{
			Data:     base64.StdEncoding.EncodeToString(pngBytes),
			MimeType: "image/png",
		},
	}
}

// Example of processing a stream that might contain images
func processStreamWithImages(ctx context.Context) error {
	// This would be used in a real scenario with Client API
	iterator, err := claudecode.Query(ctx, "Generate a chart showing sales data")
	if err != nil {
		return err
	}
	defer iterator.Close()

	for {
		msg, err := iterator.Next(ctx)
		if err != nil {
			if err == claudecode.ErrNoMoreMessages {
				break
			}
			return err
		}

		if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
			for _, block := range assistantMsg.Content {
				switch b := block.(type) {
				case *claudecode.TextBlock:
					fmt.Printf("Text: %s\n", b.Text)
				case *claudecode.ImageBlock:
					fmt.Printf("Image: %s (%d bytes)\n", b.MimeType, len(b.Data))
					// In real app: save to file, display in UI, etc.
				}
			}
		}
	}

	return nil
}
