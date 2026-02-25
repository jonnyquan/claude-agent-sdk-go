// Package mcp provides internal MCP (Model Context Protocol) types and utilities.
// This is a minimal implementation to support SDK MCP servers without external dependencies.
package mcp

import (
	"encoding/json"
)

// ContentType represents the type of content.
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
)

// Content represents content in MCP protocol.
type Content interface {
	GetType() ContentType
}

// TextContent represents text content.
type TextContent struct {
	Type ContentType `json:"type"`
	Text string      `json:"text"`
}

// GetType returns the content type for TextContent.
func (t *TextContent) GetType() ContentType {
	return ContentTypeText
}

// ImageContent represents image content with base64 data.
type ImageContent struct {
	Type     ContentType `json:"type"`
	Data     string      `json:"data"`
	MimeType string      `json:"mimeType"`
}

// GetType returns the content type for ImageContent.
func (i *ImageContent) GetType() ContentType {
	return ContentTypeImage
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// CallToolResult represents the result of a tool call.
type CallToolResult struct {
	Content []Content              `json:"content"`
	IsError bool                   `json:"isError,omitempty"`
	Metadata map[string]interface{} `json:"_meta,omitempty"`
}

// ListToolsResult represents the result of listing tools.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// JSONRPCRequest represents a JSON-RPC request.
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response.
type JSONRPCResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Result  interface{}            `json:"result,omitempty"`
	Error   *JSONRPCError          `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MarshalContent marshals content to JSON with proper type handling.
func MarshalContent(content []Content) ([]byte, error) {
	// Convert to a slice of maps for JSON marshaling
	result := make([]map[string]interface{}, len(content))
	for i, c := range content {
		switch v := c.(type) {
		case *TextContent:
			result[i] = map[string]interface{}{
				"type": "text",
				"text": v.Text,
			}
		case *ImageContent:
			result[i] = map[string]interface{}{
				"type":     "image",
				"data":     v.Data,
				"mimeType": v.MimeType,
			}
		}
	}
	return json.Marshal(result)
}

// UnmarshalContent unmarshals JSON to content slice.
func UnmarshalContent(data []byte) ([]Content, error) {
	var raw []map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	result := make([]Content, 0, len(raw))
	for _, item := range raw {
		contentType, ok := item["type"].(string)
		if !ok {
			continue
		}

		switch ContentType(contentType) {
		case ContentTypeText:
			if text, ok := item["text"].(string); ok {
				result = append(result, &TextContent{
					Type: ContentTypeText,
					Text: text,
				})
			}
		case ContentTypeImage:
			if data, ok := item["data"].(string); ok {
				if mimeType, ok := item["mimeType"].(string); ok {
					result = append(result, &ImageContent{
						Type:     ContentTypeImage,
						Data:     data,
						MimeType: mimeType,
					})
				}
			}
		}
	}

	return result, nil
}
