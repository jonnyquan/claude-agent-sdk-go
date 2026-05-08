package claudesdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/mcp"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// convertContentsToMCP converts a slice of public ToolContent values into
// the internal mcp.Content shape. Used by both the success and ToolError
// paths.
func convertContentsToMCP(contents []ToolContent) ([]mcp.Content, error) {
	mcpContents := make([]mcp.Content, len(contents))
	for i, content := range contents {
		switch c := content.(type) {
		case *TextContent:
			mcpContents[i] = &mcp.TextContent{Type: mcp.ContentTypeText, Text: c.text}
		case *ImageContent:
			mcpContents[i] = &mcp.ImageContent{Type: mcp.ContentTypeImage, Data: c.data, MimeType: c.mimeType}
		case *AudioContent:
			mcpContents[i] = &mcp.AudioContent{Type: mcp.ContentTypeAudio, Data: c.data, MimeType: c.mimeType}
		case *ResourceLinkContent:
			mcpContents[i] = &mcp.ResourceLinkContent{Type: mcp.ContentTypeResourceLink, Name: c.name, URI: c.uri, Description: c.description}
		case *ResourceContent:
			mcpContents[i] = &mcp.ResourceContent{Type: mcp.ContentTypeResource, Resource: mcp.EmbeddedResource{URI: c.uri, Text: c.text, Blob: c.blob, MimeType: c.mimeType}}
		default:
			return nil, fmt.Errorf("unsupported content type: %T", content)
		}
	}
	return mcpContents, nil
}

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
//
// To return a tool-level error with custom content (Python parity for
// `{"content": [...], "is_error": true}`), return a *ToolError. Plain
// errors are converted to is_error=true with the error text as content.
type ToolHandler func(ctx context.Context, args map[string]interface{}) ([]ToolContent, error)

// ToolError carries tool-level error content for the MCP response.
//
// Returning a *ToolError from a ToolHandler tells the SDK MCP bridge to
// emit a successful JSON-RPC response with `is_error: true` and the
// provided Content (or a single TextContent built from Message if Content
// is empty). Mirrors Python SDK's `{"content": [...], "is_error": True}`
// dict return.
type ToolError struct {
	Content []ToolContent
	Message string
}

// Error implements the error interface.
func (e *ToolError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if len(e.Content) > 0 {
		if t, ok := e.Content[0].(*TextContent); ok {
			return t.text
		}
	}
	return "tool error"
}

// NewToolError constructs a ToolError with a single TextContent message.
func NewToolError(message string) *ToolError {
	return &ToolError{Message: message}
}

// NewToolErrorWithContent constructs a ToolError that carries arbitrary
// ToolContent items in the is_error=true response.
func NewToolErrorWithContent(content ...ToolContent) *ToolError {
	return &ToolError{Content: content}
}

// ToolAnnotations represents public MCP tool annotation metadata.
type ToolAnnotations = map[string]interface{}

// ToolContent represents content returned by a tool.
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

// AudioContent represents audio content returned by a tool.
type AudioContent struct {
	data     string
	mimeType string
}

// NewAudioContent creates new audio content.
func NewAudioContent(data, mimeType string) *AudioContent {
	return &AudioContent{data: data, mimeType: mimeType}
}

// GetType returns the content type.
func (a *AudioContent) GetType() string {
	return "audio"
}

// Data returns the base64 encoded audio data.
func (a *AudioContent) Data() string {
	return a.data
}

// MimeType returns the audio MIME type.
func (a *AudioContent) MimeType() string {
	return a.mimeType
}

// ResourceLinkContent represents a resource link tool result.
type ResourceLinkContent struct {
	name        string
	uri         string
	description string
}

// NewResourceLinkContent creates a resource link content block.
func NewResourceLinkContent(name, uri, description string) *ResourceLinkContent {
	return &ResourceLinkContent{name: name, uri: uri, description: description}
}

// GetType returns the content type.
func (r *ResourceLinkContent) GetType() string {
	return "resource_link"
}

// Name returns the resource display name.
func (r *ResourceLinkContent) Name() string {
	return r.name
}

// URI returns the resource URI.
func (r *ResourceLinkContent) URI() string {
	return r.uri
}

// Description returns the resource description.
func (r *ResourceLinkContent) Description() string {
	return r.description
}

// ResourceContent represents an embedded resource tool result.
type ResourceContent struct {
	uri      string
	text     string
	blob     string
	mimeType string
}

// NewResourceTextContent creates an embedded text resource content block.
func NewResourceTextContent(uri, text, mimeType string) *ResourceContent {
	return &ResourceContent{uri: uri, text: text, mimeType: mimeType}
}

// NewResourceBlobContent creates an embedded binary resource content block.
func NewResourceBlobContent(uri, blob, mimeType string) *ResourceContent {
	return &ResourceContent{uri: uri, blob: blob, mimeType: mimeType}
}

// GetType returns the content type.
func (r *ResourceContent) GetType() string {
	return "resource"
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

	// Annotations provides optional MCP tool metadata.
	Annotations ToolAnnotations

	// InputSchema defines the tool's parameters.
	// Can be:
	// - map[string]string: Simple type mapping {"name": "string", "age": "integer"}
	// - map[string]interface{}: Full JSON Schema
	InputSchema interface{}

	// Handler is the function that executes the tool.
	Handler ToolHandler
}

// SdkMcpTool is the public SDK MCP tool definition type.
type SdkMcpTool = ToolDef

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
				// ToolError signals a tool-level error: surface as a
				// successful response with is_error=true plus the carried
				// Content. The internal MCP bridge does this when the
				// returned error is *mcp.ToolErrorContent.
				var te *ToolError
				if errors.As(err, &te) {
					mcpContents, convErr := convertContentsToMCP(te.Content)
					if convErr != nil {
						return nil, convErr
					}
					return nil, &mcp.ToolErrorContent{Content: mcpContents, Message: te.Message}
				}
				return nil, err
			}

			// Convert public ToolContent to internal mcp.Content
			return convertContentsToMCP(contents)
		}

		// Register with internal server
		err := server.RegisterTool(&mcp.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Annotations: tool.Annotations,
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

// ToolWithAnnotations is a convenience helper for creating tools with MCP annotations.
func ToolWithAnnotations(
	name, description string,
	inputSchema interface{},
	annotations ToolAnnotations,
	handler ToolHandler,
) *ToolDef {
	return &ToolDef{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Annotations: annotations,
		Handler:     handler,
	}
}
