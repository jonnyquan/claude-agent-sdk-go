// Package mcp provides internal MCP (Model Context Protocol) types and utilities.
// This is a minimal implementation to support SDK MCP servers without external dependencies.
package mcp

import (
	"encoding/json"
)

// ContentType represents the type of content.
type ContentType string

const (
	ContentTypeText         ContentType = "text"
	ContentTypeImage        ContentType = "image"
	ContentTypeAudio        ContentType = "audio"
	ContentTypeResourceLink ContentType = "resource_link"
	ContentTypeResource     ContentType = "resource"
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

// AudioContent represents audio content with base64 data.
type AudioContent struct {
	Type     ContentType `json:"type"`
	Data     string      `json:"data"`
	MimeType string      `json:"mimeType"`
}

// GetType returns the content type for AudioContent.
func (a *AudioContent) GetType() ContentType {
	return ContentTypeAudio
}

// ResourceLinkContent represents a resource link content block.
type ResourceLinkContent struct {
	Type        ContentType `json:"type"`
	Name        string      `json:"name,omitempty"`
	URI         string      `json:"uri,omitempty"`
	Description string      `json:"description,omitempty"`
}

// GetType returns the content type for ResourceLinkContent.
func (r *ResourceLinkContent) GetType() ContentType {
	return ContentTypeResourceLink
}

// EmbeddedResource represents an embedded resource payload.
type EmbeddedResource struct {
	URI      string `json:"uri,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ResourceContent represents an embedded resource content block.
type ResourceContent struct {
	Type     ContentType      `json:"type"`
	Resource EmbeddedResource `json:"resource"`
}

// GetType returns the content type for ResourceContent.
func (r *ResourceContent) GetType() ContentType {
	return ContentTypeResource
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	Meta        map[string]interface{} `json:"_meta,omitempty"`
}

// CallToolResult represents the result of a tool call.
type CallToolResult struct {
	Content  []Content              `json:"content"`
	IsError  bool                   `json:"isError,omitempty"`
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
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
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
		case *AudioContent:
			result[i] = map[string]interface{}{
				"type":     "audio",
				"data":     v.Data,
				"mimeType": v.MimeType,
			}
		case *ResourceLinkContent:
			result[i] = map[string]interface{}{
				"type":        "resource_link",
				"name":        v.Name,
				"uri":         v.URI,
				"description": v.Description,
			}
		case *ResourceContent:
			result[i] = map[string]interface{}{
				"type":     "resource",
				"resource": v.Resource,
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
		case ContentTypeAudio:
			if data, ok := item["data"].(string); ok {
				if mimeType, ok := item["mimeType"].(string); ok {
					result = append(result, &AudioContent{
						Type:     ContentTypeAudio,
						Data:     data,
						MimeType: mimeType,
					})
				}
			}
		case ContentTypeResourceLink:
			link := &ResourceLinkContent{Type: ContentTypeResourceLink}
			link.Name, _ = item["name"].(string)
			link.URI, _ = item["uri"].(string)
			link.Description, _ = item["description"].(string)
			result = append(result, link)
		case ContentTypeResource:
			resourceMap, ok := item["resource"].(map[string]interface{})
			if !ok {
				continue
			}
			resource := EmbeddedResource{}
			resource.URI, _ = resourceMap["uri"].(string)
			resource.Text, _ = resourceMap["text"].(string)
			resource.Blob, _ = resourceMap["blob"].(string)
			resource.MimeType, _ = resourceMap["mimeType"].(string)
			result = append(result, &ResourceContent{
				Type:     ContentTypeResource,
				Resource: resource,
			})
		}
	}

	return result, nil
}
