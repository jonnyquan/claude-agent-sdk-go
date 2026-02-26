package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewServer tests server creation
func TestNewServer(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.Name() != "test-server" {
		t.Errorf("Expected name 'test-server', got '%s'", server.Name())
	}

	if server.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", server.Version())
	}

	if server.tools == nil {
		t.Error("Server tools map not initialized")
	}
}

// TestRegisterTool_Success tests successful tool registration
func TestRegisterTool_Success(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tool := &ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"param": "string",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return []Content{
				&TextContent{Type: ContentTypeText, Text: "test"},
			}, nil
		},
	}

	err := server.RegisterTool(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Verify tool is registered
	tools := server.ListTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
	}
}

// TestRegisterTool_NilTool tests registering nil tool
func TestRegisterTool_NilTool(t *testing.T) {
	server := NewServer("test", "1.0.0")

	err := server.RegisterTool(nil)
	if err == nil {
		t.Error("Expected error when registering nil tool")
	}

	if err.Error() != "tool definition cannot be nil" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestRegisterTool_EmptyName tests registering tool with empty name
func TestRegisterTool_EmptyName(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tool := &ToolDefinition{
		Name:    "",
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) { return nil, nil },
	}

	err := server.RegisterTool(tool)
	if err == nil {
		t.Error("Expected error when registering tool with empty name")
	}

	if err.Error() != "tool name cannot be empty" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestRegisterTool_NilHandler tests registering tool with nil handler
func TestRegisterTool_NilHandler(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tool := &ToolDefinition{
		Name:    "test",
		Handler: nil,
	}

	err := server.RegisterTool(tool)
	if err == nil {
		t.Error("Expected error when registering tool with nil handler")
	}

	if err.Error() != "tool handler cannot be nil" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestRegisterTool_DuplicateName tests registering tools with duplicate names
func TestRegisterTool_DuplicateName(t *testing.T) {
	server := NewServer("test", "1.0.0")

	handler := func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
		return []Content{&TextContent{Type: ContentTypeText, Text: "test"}}, nil
	}

	tool1 := &ToolDefinition{
		Name:        "duplicate",
		Description: "First tool",
		Handler:     handler,
	}

	tool2 := &ToolDefinition{
		Name:        "duplicate",
		Description: "Second tool",
		Handler:     handler,
	}

	// First registration should succeed
	if err := server.RegisterTool(tool1); err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Second registration with same name should overwrite (this is current behavior)
	// In production, you might want to return an error instead
	if err := server.RegisterTool(tool2); err != nil {
		t.Fatalf("Second registration failed: %v", err)
	}

	// Should have only one tool (overwritten)
	tools := server.ListTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool after duplicate registration, got %d", len(tools))
	}

	// Description should be from second tool
	if tools[0].Description != "Second tool" {
		t.Errorf("Tool was not overwritten, got description: %s", tools[0].Description)
	}
}

// TestListTools tests listing registered tools
func TestListTools(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Empty list initially
	tools := server.ListTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools initially, got %d", len(tools))
	}

	// Register multiple tools
	handler := func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
		return []Content{&TextContent{Type: ContentTypeText, Text: "test"}}, nil
	}

	for i := 0; i < 5; i++ {
		tool := &ToolDefinition{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: fmt.Sprintf("Tool %d", i),
			InputSchema: map[string]interface{}{
				"param": "string",
			},
			Handler: handler,
		}

		if err := server.RegisterTool(tool); err != nil {
			t.Fatalf("Failed to register tool %d: %v", i, err)
		}
	}

	// List should have all tools
	tools = server.ListTools()
	if len(tools) != 5 {
		t.Errorf("Expected 5 tools, got %d", len(tools))
	}

	// Verify each tool has proper schema
	for _, tool := range tools {
		// Tool.InputSchema is map[string]interface{}, not interface{}
		if tool.InputSchema["type"] != "object" {
			t.Errorf("Tool %s schema type is not 'object', got %v", tool.Name, tool.InputSchema["type"])
		}

		if tool.InputSchema["properties"] == nil {
			t.Errorf("Tool %s schema missing properties", tool.Name)
		}
	}
}

func TestListToolsIncludesAnnotations(t *testing.T) {
	server := NewServer("test", "1.0.0")

	handler := func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
		return []Content{&TextContent{Type: ContentTypeText, Text: "ok"}}, nil
	}

	tool := &ToolDefinition{
		Name:        "annotated",
		Description: "Annotated tool",
		InputSchema: map[string]interface{}{"name": "string"},
		Annotations: map[string]interface{}{
			"title":           "Annotated",
			"readOnlyHint":    true,
			"destructiveHint": false,
		},
		Handler: handler,
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	tools := server.ListTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Annotations == nil {
		t.Fatal("expected annotations to be present")
	}
	if got, ok := tools[0].Annotations["title"].(string); !ok || got != "Annotated" {
		t.Fatalf("unexpected annotations.title: %v", tools[0].Annotations["title"])
	}
}

// TestCallTool_Success tests successful tool execution
func TestCallTool_Success(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Register a tool that echoes the input
	tool := &ToolDefinition{
		Name:        "echo",
		Description: "Echo the input",
		InputSchema: map[string]interface{}{
			"message": "string",
		},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			msg, ok := args["message"].(string)
			if !ok {
				msg = "no message"
			}
			return []Content{
				&TextContent{Type: ContentTypeText, Text: msg},
			}, nil
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Call the tool
	ctx := context.Background()
	args := map[string]interface{}{
		"message": "hello world",
	}

	result, err := server.CallTool(ctx, "echo", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result))
	}

	textContent, ok := result[0].(*TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, got %T", result[0])
	}

	if textContent.Text != "hello world" {
		t.Errorf("Expected text 'hello world', got '%s'", textContent.Text)
	}
}

// TestCallTool_NotFound tests calling non-existent tool
func TestCallTool_NotFound(t *testing.T) {
	server := NewServer("test", "1.0.0")

	ctx := context.Background()
	_, err := server.CallTool(ctx, "nonexistent", nil)

	if err == nil {
		t.Error("Expected error when calling non-existent tool")
	}

	expectedMsg := "tool 'nonexistent' not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestCallTool_HandlerError tests tool handler returning error
func TestCallTool_HandlerError(t *testing.T) {
	server := NewServer("test", "1.0.0")

	expectedError := fmt.Errorf("handler error")

	tool := &ToolDefinition{
		Name:        "error_tool",
		Description: "A tool that errors",
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return nil, expectedError
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	ctx := context.Background()
	_, err := server.CallTool(ctx, "error_tool", nil)

	if err == nil {
		t.Error("Expected error from tool handler")
	}

	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

// TestCallTool_ContextCancellation tests tool respecting context cancellation
func TestCallTool_ContextCancellation(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tool := &ToolDefinition{
		Name:        "slow_tool",
		Description: "A slow tool",
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return []Content{&TextContent{Type: ContentTypeText, Text: "done"}}, nil
			}
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := server.CallTool(ctx, "slow_tool", nil)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded error, got %v", err)
	}
}

// TestBuildJSONSchema tests JSON schema generation
func TestBuildJSONSchema(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name: "simple type map",
			input: map[string]interface{}{
				"name": "string",
				"age":  "integer",
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
				},
				"required": []string{"name", "age"},
			},
		},
		{
			name: "typed map",
			input: map[string]Type{
				"name": TypeString,
				"age":  TypeInt,
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
				},
				"required": []string{"name", "age"},
			},
		},
		{
			name: "complete JSON schema",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "User's name",
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "User's name",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.buildJSONSchema(tt.input)

			// Verify type
			if result["type"] != "object" {
				t.Errorf("Expected type 'object', got '%v'", result["type"])
			}

			// Verify properties exist
			props, ok := result["properties"].(map[string]interface{})
			if !ok {
				t.Errorf("Properties is not map[string]interface{}")
			}

			expectedProps := tt.expected["properties"].(map[string]interface{})
			if len(props) != len(expectedProps) {
				t.Errorf("Expected %d properties, got %d", len(expectedProps), len(props))
			}
		})
	}
}

// TestHandleJSONRPC_ListTools tests JSON-RPC tools/list method
func TestHandleJSONRPC_ListTools(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Register a tool
	tool := &ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{"param": "string"},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return []Content{&TextContent{Type: ContentTypeText, Text: "test"}}, nil
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Create JSON-RPC request
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Handle request
	responseData, err := server.HandleJSONRPC(1, requestData)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	// Parse response
	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify response
	if response.Error != nil {
		t.Errorf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Result is not map[string]interface{}")
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("Tools is not []interface{}")
	}

	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
}

func TestHandleJSONRPC_Initialize(t *testing.T) {
	server := NewServer("test-server", "")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]interface{}{},
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	responseData, err := server.HandleJSONRPC(1, requestData)
	if err != nil {
		t.Fatalf("HandleJSONRPC returned error: %v", err)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("expected success response, got error: %+v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", response.Result)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("unexpected protocolVersion: %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected serverInfo map, got %T", result["serverInfo"])
	}
	if serverInfo["name"] != "test-server" {
		t.Fatalf("unexpected serverInfo.name: %v", serverInfo["name"])
	}
	if serverInfo["version"] != "1.0.0" {
		t.Fatalf("unexpected serverInfo.version: %v", serverInfo["version"])
	}
}

func TestHandleJSONRPC_NotificationsInitialized(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	responseData, err := server.HandleJSONRPC(nil, reqBytes)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if got, ok := response["jsonrpc"].(string); !ok || got != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %#v", response["jsonrpc"])
	}
	if _, hasID := response["id"]; hasID {
		t.Fatalf("expected notifications/initialized response without id, got %#v", response["id"])
	}
	if _, ok := response["result"].(map[string]interface{}); !ok {
		t.Fatalf("expected object result, got %#v", response["result"])
	}
}

// TestHandleJSONRPC_CallTool tests JSON-RPC tools/call method
func TestHandleJSONRPC_CallTool(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Register echo tool
	tool := &ToolDefinition{
		Name:        "echo",
		Description: "Echo input",
		InputSchema: map[string]interface{}{"message": "string"},
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			msg := args["message"].(string)
			return []Content{&TextContent{Type: ContentTypeText, Text: msg}}, nil
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Create JSON-RPC request
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "echo",
			"arguments": map[string]interface{}{
				"message": "hello",
			},
		},
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Handle request
	responseData, err := server.HandleJSONRPC(2, requestData)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	// Parse response
	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify response
	if response.Error != nil {
		t.Errorf("Expected no error, got: %v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Result is not map[string]interface{}")
	}

	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("Content is not []interface{}")
	}

	if len(content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(content))
	}
}

func TestHandleJSONRPC_CallToolHandlerErrorReturnsIsErrorResult(t *testing.T) {
	server := NewServer("test", "1.0.0")

	tool := &ToolDefinition{
		Name:        "always_fail",
		Description: "Always fails",
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			return nil, fmt.Errorf("boom")
		},
	}
	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      9,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "always_fail",
			"arguments": map[string]interface{}{},
		},
	}
	requestData, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	responseData, err := server.HandleJSONRPC(9, requestData)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("expected success response with is_error result, got error: %#v", response.Error)
	}

	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result is not map[string]interface{}, got %T", response.Result)
	}
	isError, ok := result["is_error"].(bool)
	if !ok || !isError {
		t.Fatalf("expected is_error=true, got %#v", result["is_error"])
	}
	content, ok := result["content"].([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("expected single content item, got %#v", result["content"])
	}
	first, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map content item, got %T", content[0])
	}
	if first["type"] != "text" {
		t.Fatalf("expected text content type, got %#v", first["type"])
	}
	if first["text"] != "boom" {
		t.Fatalf("expected error text 'boom', got %#v", first["text"])
	}
}

// TestHandleJSONRPC_InvalidJSON tests handling invalid JSON
func TestHandleJSONRPC_InvalidJSON(t *testing.T) {
	server := NewServer("test", "1.0.0")

	invalidJSON := []byte("{ invalid json }")

	responseData, err := server.HandleJSONRPC(nil, invalidJSON)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for invalid JSON")
	}

	if response.Error.Code != -32700 {
		t.Errorf("Expected error code -32700, got %d", response.Error.Code)
	}
}

// TestHandleJSONRPC_UnknownMethod tests handling unknown method
func TestHandleJSONRPC_UnknownMethod(t *testing.T) {
	server := NewServer("test", "1.0.0")

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "unknown/method",
		Params:  map[string]interface{}{},
	}

	requestData, _ := json.Marshal(request)

	responseData, err := server.HandleJSONRPC(3, requestData)
	if err != nil {
		t.Fatalf("HandleJSONRPC failed: %v", err)
	}

	var response JSONRPCResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for unknown method")
	}

	if response.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", response.Error.Code)
	}
}

// TestConcurrentToolRegistration tests concurrent tool registration
func TestConcurrentToolRegistration(t *testing.T) {
	server := NewServer("test", "1.0.0")

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			tool := &ToolDefinition{
				Name:        fmt.Sprintf("tool_%d", id),
				Description: fmt.Sprintf("Tool %d", id),
				Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
					return []Content{&TextContent{Type: ContentTypeText, Text: "test"}}, nil
				},
			}

			if err := server.RegisterTool(tool); err != nil {
				t.Errorf("Failed to register tool %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all tools registered
	tools := server.ListTools()
	if len(tools) != numGoroutines {
		t.Errorf("Expected %d tools, got %d", numGoroutines, len(tools))
	}
}

// TestConcurrentToolExecution tests concurrent tool execution
func TestConcurrentToolExecution(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Register a tool
	tool := &ToolDefinition{
		Name:        "concurrent",
		Description: "Test concurrent execution",
		Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			id := args["id"].(float64)
			return []Content{
				&TextContent{Type: ContentTypeText, Text: fmt.Sprintf("result_%d", int(id))},
			}, nil
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Execute tool concurrently
	var wg sync.WaitGroup
	numExecutions := 50
	results := make([][]Content, numExecutions)
	errors := make([]error, numExecutions)

	wg.Add(numExecutions)
	for i := 0; i < numExecutions; i++ {
		go func(id int) {
			defer wg.Done()

			ctx := context.Background()
			args := map[string]interface{}{"id": float64(id)}

			result, err := server.CallTool(ctx, "concurrent", args)
			results[id] = result
			errors[id] = err
		}(i)
	}

	wg.Wait()

	// Verify all executions succeeded
	for i := 0; i < numExecutions; i++ {
		if errors[i] != nil {
			t.Errorf("Execution %d failed: %v", i, errors[i])
		}

		if len(results[i]) != 1 {
			t.Errorf("Execution %d returned %d content items", i, len(results[i]))
			continue
		}

		textContent, ok := results[i][0].(*TextContent)
		if !ok {
			t.Errorf("Execution %d returned non-text content", i)
			continue
		}

		expected := fmt.Sprintf("result_%d", i)
		if textContent.Text != expected {
			t.Errorf("Execution %d: expected '%s', got '%s'", i, expected, textContent.Text)
		}
	}
}

// TestConcurrentListAndCall tests concurrent listing and calling
func TestConcurrentListAndCall(t *testing.T) {
	server := NewServer("test", "1.0.0")

	// Register tools
	for i := 0; i < 10; i++ {
		tool := &ToolDefinition{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: fmt.Sprintf("Tool %d", i),
			Handler: func(ctx context.Context, args map[string]interface{}) ([]Content, error) {
				return []Content{&TextContent{Type: ContentTypeText, Text: "test"}}, nil
			},
		}

		if err := server.RegisterTool(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}

	// Concurrently list and call tools
	var wg sync.WaitGroup
	numOperations := 100

	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()

			if id%2 == 0 {
				// List tools
				tools := server.ListTools()
				if len(tools) != 10 {
					t.Errorf("List operation %d: expected 10 tools, got %d", id, len(tools))
				}
			} else {
				// Call tool
				toolName := fmt.Sprintf("tool_%d", id%10)
				ctx := context.Background()
				_, err := server.CallTool(ctx, toolName, nil)
				if err != nil {
					t.Errorf("Call operation %d failed: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()
}
