package mcp

import (
	"encoding/json"
	"testing"
)

// TestTextContent tests TextContent creation and methods
func TestTextContent(t *testing.T) {
	text := &TextContent{
		Type: ContentTypeText,
		Text: "Hello, world!",
	}
	
	if text.GetType() != ContentTypeText {
		t.Errorf("Expected type %s, got %s", ContentTypeText, text.GetType())
	}
	
	if text.Text != "Hello, world!" {
		t.Errorf("Expected text 'Hello, world!', got '%s'", text.Text)
	}
}

// TestImageContent tests ImageContent creation and methods
func TestImageContent(t *testing.T) {
	image := &ImageContent{
		Type:     ContentTypeImage,
		Data:     "base64data",
		MimeType: "image/png",
	}
	
	if image.GetType() != ContentTypeImage {
		t.Errorf("Expected type %s, got %s", ContentTypeImage, image.GetType())
	}
	
	if image.Data != "base64data" {
		t.Errorf("Expected data 'base64data', got '%s'", image.Data)
	}
	
	if image.MimeType != "image/png" {
		t.Errorf("Expected MIME type 'image/png', got '%s'", image.MimeType)
	}
}

// TestMarshalContent tests marshaling content to JSON
func TestMarshalContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []Content
		expected string
	}{
		{
			name: "text content",
			content: []Content{
				&TextContent{Type: ContentTypeText, Text: "hello"},
			},
			expected: `[{"text":"hello","type":"text"}]`,
		},
		{
			name: "image content",
			content: []Content{
				&ImageContent{
					Type:     ContentTypeImage,
					Data:     "abc123",
					MimeType: "image/jpeg",
				},
			},
			expected: `[{"data":"abc123","mimeType":"image/jpeg","type":"image"}]`,
		},
		{
			name: "mixed content",
			content: []Content{
				&TextContent{Type: ContentTypeText, Text: "description"},
				&ImageContent{
					Type:     ContentTypeImage,
					Data:     "xyz789",
					MimeType: "image/png",
				},
			},
			expected: `[{"text":"description","type":"text"},{"data":"xyz789","mimeType":"image/png","type":"image"}]`,
		},
		{
			name:     "empty content",
			content:  []Content{},
			expected: `[]`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalContent(tt.content)
			if err != nil {
				t.Fatalf("MarshalContent failed: %v", err)
			}
			
			// Compare JSON (normalize by unmarshaling and remarshaling)
			var got, expected interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected: %v", err)
			}
			
			gotJSON, _ := json.Marshal(got)
			expectedJSON, _ := json.Marshal(expected)
			
			if string(gotJSON) != string(expectedJSON) {
				t.Errorf("Expected JSON:\n%s\nGot:\n%s", expectedJSON, gotJSON)
			}
		})
	}
}

// TestUnmarshalContent tests unmarshaling JSON to content
func TestUnmarshalContent(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedLen int
		validate    func(t *testing.T, content []Content)
	}{
		{
			name:        "text content",
			input:       `[{"type":"text","text":"hello"}]`,
			expectedLen: 1,
			validate: func(t *testing.T, content []Content) {
				text, ok := content[0].(*TextContent)
				if !ok {
					t.Fatalf("Expected TextContent, got %T", content[0])
				}
				if text.Text != "hello" {
					t.Errorf("Expected text 'hello', got '%s'", text.Text)
				}
			},
		},
		{
			name:        "image content",
			input:       `[{"type":"image","data":"abc123","mimeType":"image/png"}]`,
			expectedLen: 1,
			validate: func(t *testing.T, content []Content) {
				image, ok := content[0].(*ImageContent)
				if !ok {
					t.Fatalf("Expected ImageContent, got %T", content[0])
				}
				if image.Data != "abc123" {
					t.Errorf("Expected data 'abc123', got '%s'", image.Data)
				}
				if image.MimeType != "image/png" {
					t.Errorf("Expected MIME type 'image/png', got '%s'", image.MimeType)
				}
			},
		},
		{
			name:        "mixed content",
			input:       `[{"type":"text","text":"desc"},{"type":"image","data":"xyz","mimeType":"image/jpeg"}]`,
			expectedLen: 2,
			validate: func(t *testing.T, content []Content) {
				// Validate first item (text)
				text, ok := content[0].(*TextContent)
				if !ok {
					t.Fatalf("Expected TextContent at index 0, got %T", content[0])
				}
				if text.Text != "desc" {
					t.Errorf("Expected text 'desc', got '%s'", text.Text)
				}
				
				// Validate second item (image)
				image, ok := content[1].(*ImageContent)
				if !ok {
					t.Fatalf("Expected ImageContent at index 1, got %T", content[1])
				}
				if image.Data != "xyz" {
					t.Errorf("Expected data 'xyz', got '%s'", image.Data)
				}
			},
		},
		{
			name:        "empty array",
			input:       `[]`,
			expectedLen: 0,
			validate:    func(t *testing.T, content []Content) {},
		},
		{
			name:        "unknown type ignored",
			input:       `[{"type":"unknown","data":"test"},{"type":"text","text":"valid"}]`,
			expectedLen: 1,
			validate: func(t *testing.T, content []Content) {
				text, ok := content[0].(*TextContent)
				if !ok {
					t.Fatalf("Expected TextContent, got %T", content[0])
				}
				if text.Text != "valid" {
					t.Errorf("Expected text 'valid', got '%s'", text.Text)
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := UnmarshalContent([]byte(tt.input))
			if err != nil {
				t.Fatalf("UnmarshalContent failed: %v", err)
			}
			
			if len(content) != tt.expectedLen {
				t.Fatalf("Expected %d content items, got %d", tt.expectedLen, len(content))
			}
			
			tt.validate(t, content)
		})
	}
}

// TestUnmarshalContent_InvalidJSON tests handling of invalid JSON
func TestUnmarshalContent_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{invalid json}`)
	
	_, err := UnmarshalContent(invalidJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestUnmarshalContent_MissingFields tests handling of missing required fields
func TestUnmarshalContent_MissingFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "text without text field",
			input: `[{"type":"text"}]`,
		},
		{
			name:  "image without data",
			input: `[{"type":"image","mimeType":"image/png"}]`,
		},
		{
			name:  "image without mimeType",
			input: `[{"type":"image","data":"abc"}]`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := UnmarshalContent([]byte(tt.input))
			if err != nil {
				t.Fatalf("UnmarshalContent failed: %v", err)
			}
			
			// Should result in empty array (invalid items skipped)
			if len(content) != 0 {
				t.Errorf("Expected 0 content items (invalid skipped), got %d", len(content))
			}
		})
	}
}

// TestJSONRPCRequest tests JSON-RPC request marshaling/unmarshaling
func TestJSONRPCRequest(t *testing.T) {
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      123,
		Method:  "tools/list",
		Params: map[string]interface{}{
			"filter": "active",
		},
	}
	
	// Marshal
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	// Unmarshal
	var decoded JSONRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}
	
	// Verify fields
	if decoded.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got '%s'", decoded.JSONRPC)
	}
	
	if decoded.Method != "tools/list" {
		t.Errorf("Expected method 'tools/list', got '%s'", decoded.Method)
	}
	
	// ID could be number or string, check it exists
	if decoded.ID == nil {
		t.Error("Expected ID to be present")
	}
}

// TestJSONRPCResponse_Success tests successful JSON-RPC response
func TestJSONRPCResponse_Success(t *testing.T) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      456,
		Result: map[string]interface{}{
			"status": "success",
			"data":   []string{"item1", "item2"},
		},
	}
	
	// Marshal
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}
	
	// Unmarshal
	var decoded JSONRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Verify
	if decoded.Error != nil {
		t.Errorf("Expected no error, got %v", decoded.Error)
	}
	
	if decoded.Result == nil {
		t.Error("Expected result to be present")
	}
	
	result, ok := decoded.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map, got %T", decoded.Result)
	}
	
	if result["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", result["status"])
	}
}

// TestJSONRPCResponse_Error tests error JSON-RPC response
func TestJSONRPCResponse_Error(t *testing.T) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      789,
		Error: &JSONRPCError{
			Code:    -32600,
			Message: "Invalid Request",
			Data:    map[string]interface{}{"detail": "missing method"},
		},
	}
	
	// Marshal
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}
	
	// Unmarshal
	var decoded JSONRPCResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	// Verify
	if decoded.Result != nil {
		t.Errorf("Expected no result in error response, got %v", decoded.Result)
	}
	
	if decoded.Error == nil {
		t.Fatal("Expected error to be present")
	}
	
	if decoded.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", decoded.Error.Code)
	}
	
	if decoded.Error.Message != "Invalid Request" {
		t.Errorf("Expected message 'Invalid Request', got '%s'", decoded.Error.Message)
	}
}

// TestTool_JSONSerialization tests Tool struct JSON serialization
func TestTool_JSONSerialization(t *testing.T) {
	tool := Tool{
		Name:        "my_tool",
		Description: "A sample tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
	
	// Marshal
	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}
	
	// Unmarshal
	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal tool: %v", err)
	}
	
	// Verify
	if decoded.Name != "my_tool" {
		t.Errorf("Expected name 'my_tool', got '%s'", decoded.Name)
	}
	
	if decoded.Description != "A sample tool" {
		t.Errorf("Expected description 'A sample tool', got '%s'", decoded.Description)
	}
	
	if decoded.InputSchema == nil {
		t.Error("Expected InputSchema to be present")
	}
}

// TestListToolsResult tests ListToolsResult serialization
func TestListToolsResult(t *testing.T) {
	result := ListToolsResult{
		Tools: []Tool{
			{
				Name:        "tool1",
				Description: "First tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
	}
	
	// Marshal
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}
	
	// Unmarshal
	var decoded ListToolsResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	// Verify
	if len(decoded.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(decoded.Tools))
	}
	
	if decoded.Tools[0].Name != "tool1" {
		t.Errorf("Expected first tool name 'tool1', got '%s'", decoded.Tools[0].Name)
	}
}

// TestContentType tests ContentType constants
func TestContentType(t *testing.T) {
	if ContentTypeText != "text" {
		t.Errorf("Expected ContentTypeText to be 'text', got '%s'", ContentTypeText)
	}
	
	if ContentTypeImage != "image" {
		t.Errorf("Expected ContentTypeImage to be 'image', got '%s'", ContentTypeImage)
	}
}

// TestTypeConstants tests Type constants
func TestTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		typeVal  Type
		expected string
	}{
		{"TypeString", TypeString, "string"},
		{"TypeInt", TypeInt, "integer"},
		{"TypeFloat", TypeFloat, "number"},
		{"TypeBool", TypeBool, "boolean"},
		{"TypeObject", TypeObject, "object"},
		{"TypeArray", TypeArray, "array"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.typeVal != Type(tt.expected) {
				t.Errorf("Expected %s to be '%s', got '%v'", tt.name, tt.expected, tt.typeVal)
			}
		})
	}
}

// TestRoundTripContentSerialization tests full round-trip of content serialization
func TestRoundTripContentSerialization(t *testing.T) {
	original := []Content{
		&TextContent{
			Type: ContentTypeText,
			Text: "This is a test message",
		},
		&ImageContent{
			Type:     ContentTypeImage,
			Data:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ",
			MimeType: "image/png",
		},
		&TextContent{
			Type: ContentTypeText,
			Text: "Another text block",
		},
	}
	
	// Marshal
	data, err := MarshalContent(original)
	if err != nil {
		t.Fatalf("MarshalContent failed: %v", err)
	}
	
	// Unmarshal
	decoded, err := UnmarshalContent(data)
	if err != nil {
		t.Fatalf("UnmarshalContent failed: %v", err)
	}
	
	// Verify
	if len(decoded) != len(original) {
		t.Fatalf("Expected %d content items, got %d", len(original), len(decoded))
	}
	
	// Verify first text content
	text1, ok := decoded[0].(*TextContent)
	if !ok {
		t.Fatalf("Expected TextContent at index 0, got %T", decoded[0])
	}
	if text1.Text != "This is a test message" {
		t.Errorf("Text content mismatch at index 0")
	}
	
	// Verify image content
	image, ok := decoded[1].(*ImageContent)
	if !ok {
		t.Fatalf("Expected ImageContent at index 1, got %T", decoded[1])
	}
	if image.Data != "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJ" {
		t.Errorf("Image data mismatch at index 1")
	}
	if image.MimeType != "image/png" {
		t.Errorf("Image MIME type mismatch at index 1")
	}
	
	// Verify second text content
	text2, ok := decoded[2].(*TextContent)
	if !ok {
		t.Fatalf("Expected TextContent at index 2, got %T", decoded[2])
	}
	if text2.Text != "Another text block" {
		t.Errorf("Text content mismatch at index 2")
	}
}
