package claudesdk

import (
	"context"
	"fmt"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/mcp"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// ToolHandler is a function that handles tool execution.
// It receives the tool arguments and returns content (text or images).
//
// Example:
//
//	func greetHandler(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
//	    name := args["name"].(string)
//	    return []ToolContent{
//	        NewTextContent(fmt.Sprintf("Hello, %s!", name)),
//	    }, nil
//	}
type ToolHandler func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error)

// ToolContent represents content returned by a tool (text or image).
type ToolContent interface {
	GetType() string
}

// TextContent represents text content returned by a tool.
type TextContent struct {
	text string
}

// NewTextContent creates new text content.
func NewTextContent(text string) *TextContent {
	return &TextContent{text: text}
}

// GetType returns the content type.
func (t *TextContent) GetType() string {
	return "text"
}

// Text returns the text content.
func (t *TextContent) Text() string {
	return t.text
}

// ImageContent represents image content returned by a tool.
type ImageContent struct {
	data     string // base64 encoded
	mimeType string
}

// NewImageContent creates new image content.
// data should be base64 encoded image data.
// mimeType should be like "image/png", "image/jpeg", etc.
func NewImageContent(data, mimeType string) *ImageContent {
	return &ImageContent{
		data:     data,
		mimeType: mimeType,
	}
}

// GetType returns the content type.
func (i *ImageContent) GetType() string {
	return "image"
}

// Data returns the base64 encoded image data.
func (i *ImageContent) Data() string {
	return i.data
}

// MimeType returns the image MIME type.
func (i *ImageContent) MimeType() string {
	return i.mimeType
}

// ToolDef defines a tool with its schema and handler.
//
// Example:
//
//	tool := &ToolDef{
//	    Name:        "greet",
//	    Description: "Greet a user by name",
//	    InputSchema: map[string]interface{}{
//	        "name": "string",
//	    },
//	    Handler: greetHandler,
//	}
type ToolDef struct {
	// Name is the unique identifier for the tool.
	Name string

	// Description explains what the tool does.
	Description string

	// InputSchema defines the tool's parameters.
	// Can be:
	// - map[string]string: Simple type mapping {"name": "string", "age": "integer"}
	// - map[string]interface{}: Full JSON Schema
	InputSchema interface{}

	// Handler is the function that executes the tool.
	Handler ToolHandler
}

// CreateSDKMcpServer creates an in-process MCP server.
//
// Unlike external MCP servers that run as separate processes, SDK MCP servers
// run directly in your application's process. This provides:
//   - Better performance (no IPC overhead)
//   - Simpler deployment (single process)
//   - Easier debugging (same process)
//   - Direct access to your application's state
//
// Example:
//
//	// Define tools
//	greet := &ToolDef{
//	    Name:        "greet",
//	    Description: "Greet a user",
//	    InputSchema: map[string]interface{}{"name": "string"},
//	    Handler: func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
//	        name := args["name"].(string)
//	        return []ToolContent{NewTextContent("Hello, " + name + "!")}, nil
//	    },
//	}
//
//	// Create server
//	server := CreateSDKMcpServer("my-tools", "1.0.0", greet)
//
//	// Use with SDK
//	options := NewOptions(
//	    WithMcpServers(map[string]McpServerConfig{
//	        "my-tools": server,
//	    }),
//	)
func CreateSDKMcpServer(name string, version string, tools ...*ToolDef) *shared.McpSdkServerConfig {
	if version == "" {
		version = "1.0.0"
	}

	// Create internal MCP server
	server := mcp.NewServer(name, version)

	// Register tools
	for _, tool := range tools {
		if tool == nil {
			continue
		}

		// Wrap the handler to convert between public and internal types
		wrappedHandler := func(ctx context.Context, args map[string]interface{}) ([]mcp.Content, error) {
			// Call the user's handler
			contents, err := tool.Handler(ctx, args)
			if err != nil {
				return nil, err
			}

			// Convert public ToolContent to internal mcp.Content
			mcpContents := make([]mcp.Content, len(contents))
			for i, content := range contents {
				switch c := content.(type) {
				case *TextContent:
					mcpContents[i] = &mcp.TextContent{
						Type: mcp.ContentTypeText,
						Text: c.text,
					}
				case *ImageContent:
					mcpContents[i] = &mcp.ImageContent{
						Type:     mcp.ContentTypeImage,
						Data:     c.data,
						MimeType: c.mimeType,
					}
				default:
					return nil, fmt.Errorf("unsupported content type: %T", content)
				}
			}

			return mcpContents, nil
		}

		// Register with internal server
		err := server.RegisterTool(&mcp.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Handler:     wrappedHandler,
		})
		if err != nil {
			// Log error but continue (don't fail server creation)
			fmt.Printf("Warning: failed to register tool %s: %v\n", tool.Name, err)
		}
	}

	// Return SDK server config
	return &shared.McpSdkServerConfig{
		Type:     shared.McpServerTypeSDK,
		Name:     name,
		Instance: server,
	}
}

// Tool is a convenience function that creates a ToolDef.
// It's equivalent to creating a ToolDef struct but more concise.
//
// Example:
//
//	greet := Tool("greet", "Greet user", map[string]interface{}{"name": "string"},
//	    func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error) {
//	        return []ToolContent{NewTextContent("Hello!")}, nil
//	    },
//	)
func Tool(name, description string, inputSchema interface{}, handler ToolHandler) *ToolDef {
	return &ToolDef{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Handler:     handler,
	}
}
