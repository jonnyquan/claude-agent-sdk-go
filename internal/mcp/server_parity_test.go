package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestListToolsIncludesAnthropicMaxResultSizeMeta(t *testing.T) {
	server := NewServer("test", "1.0.0")
	if err := server.RegisterTool(&ToolDefinition{
		Name:        "large_output",
		Description: "returns large output",
		InputSchema: map[string]interface{}{},
		Annotations: map[string]interface{}{"maxResultSizeChars": 500000},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return []Content{&TextContent{Type: ContentTypeText, Text: "ok"}}, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	tools := server.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if got := tools[0].Meta["anthropic/maxResultSizeChars"]; got != 500000 {
		t.Fatalf("expected anthropic/maxResultSizeChars=500000, got %#v", got)
	}
}

func TestHandleCallToolConvertsResourceLinkToText(t *testing.T) {
	server := NewServer("test", "1.0.0")
	if err := server.RegisterTool(&ToolDefinition{
		Name:        "link_tool",
		Description: "returns a resource link",
		InputSchema: map[string]interface{}{},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return []Content{
				&ResourceLinkContent{
					Type:        ContentTypeResourceLink,
					Name:        "API Docs",
					URI:         "https://api.example.com",
					Description: "The API documentation",
				},
			}, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	requestData, err := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "link_tool",
			"arguments": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	responseData, err := server.HandleJSONRPC(1, requestData)
	if err != nil {
		t.Fatalf("handle jsonrpc: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	result := response["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	first := content[0].(map[string]interface{})
	if first["type"] != "text" {
		t.Fatalf("expected text content, got %#v", first)
	}
	text := first["text"].(string)
	for _, expected := range []string{"API Docs", "https://api.example.com", "The API documentation"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in %q", expected, text)
		}
	}
}

func TestHandleCallToolConvertsEmbeddedTextResourceAndSkipsBinary(t *testing.T) {
	server := NewServer("test", "1.0.0")
	if err := server.RegisterTool(&ToolDefinition{
		Name:        "resource_tool",
		Description: "returns embedded resources",
		InputSchema: map[string]interface{}{},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return []Content{
				&ResourceContent{
					Type: ContentTypeResource,
					Resource: EmbeddedResource{
						URI:      "file:///notes.txt",
						Text:     "File contents here",
						MimeType: "text/plain",
					},
				},
				&ResourceContent{
					Type: ContentTypeResource,
					Resource: EmbeddedResource{
						URI:      "file:///image.png",
						Blob:     "iVBORw0KGgo=",
						MimeType: "image/png",
					},
				},
			}, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	requestData, err := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "resource_tool",
			"arguments": map[string]interface{}{},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	responseData, err := server.HandleJSONRPC(1, requestData)
	if err != nil {
		t.Fatalf("handle jsonrpc: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	result := response["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("expected binary embedded resource to be skipped, got %d items", len(content))
	}
	first := content[0].(map[string]interface{})
	if first["type"] != "text" || first["text"] != "File contents here" {
		t.Fatalf("unexpected normalized content: %#v", first)
	}
}
