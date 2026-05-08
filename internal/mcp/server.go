package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ToolHandler is a function that handles tool execution.
// It receives the tool arguments and returns the result content.
type ToolHandler func(ctx context.Context, args map[string]interface{}) ([]Content, error)

// ToolErrorContent is the internal error type that signals a tool-level
// error with content. The handler returns this wrapped in `error`; the
// JSON-RPC bridge unwraps it via errors.As and emits a successful response
// with `is_error: true` and the carried content.
//
// The public claudesdk package's ToolError is converted to this internally.
type ToolErrorContent struct {
	Content []Content
	Message string
}

// Error implements the error interface.
func (e *ToolErrorContent) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if len(e.Content) > 0 {
		if t, ok := e.Content[0].(*TextContent); ok {
			return t.Text
		}
	}
	return "tool error"
}

// ToolDefinition defines a tool with its handler.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema interface{} // Can be map[string]Type or map[string]interface{} (JSON Schema)
	Annotations map[string]interface{}
	Handler     ToolHandler
}

// Server represents an in-process MCP server.
type Server struct {
	name    string
	version string
	tools   map[string]*ToolDefinition
	mu      sync.RWMutex
}

// NewServer creates a new MCP server instance.
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]*ToolDefinition),
	}
}

// RegisterTool registers a tool with the server.
func (s *Server) RegisterTool(tool *ToolDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tool == nil {
		return fmt.Errorf("tool definition cannot be nil")
	}
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if tool.Handler == nil {
		return fmt.Errorf("tool handler cannot be nil")
	}

	s.tools[tool.Name] = tool
	return nil
}

// ListTools returns all registered tools.
func (s *Server) ListTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Tool, 0, len(s.tools))
	for _, toolDef := range s.tools {
		schema := s.buildJSONSchema(toolDef.InputSchema)
		result = append(result, Tool{
			Name:        toolDef.Name,
			Description: toolDef.Description,
			InputSchema: schema,
			Annotations: toolDef.Annotations,
			Meta:        buildToolMeta(toolDef.Annotations),
		})
	}
	return result
}

// CallTool executes a tool by name with the given arguments.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) ([]Content, error) {
	s.mu.RLock()
	tool, ok := s.tools[name]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}

	// Execute the tool handler
	return tool.Handler(ctx, args)
}

// HandleJSONRPC handles a JSON-RPC request and returns a response.
// The requestID parameter can be any JSON-RPC request ID type (typically a number or string).
func (s *Server) HandleJSONRPC(requestID interface{}, requestData []byte) ([]byte, error) {
	var request JSONRPCRequest
	if err := json.Unmarshal(requestData, &request); err != nil {
		return s.errorResponse(nil, -32700, "Parse error", err)
	}

	// Use the parsed request ID, not the parameter
	// (The parameter is for interface compatibility but not used in SDK servers)

	// Create context for the request if not provided
	ctx := context.Background()

	switch request.Method {
	case "initialize":
		return s.handleInitialize(request)
	case "tools/list":
		return s.handleListTools(request)
	case "tools/call":
		return s.handleCallTool(ctx, request)
	case "notifications/initialized":
		// Match Python SDK bridge behavior: notifications/initialized
		// returns a JSON-RPC success payload without an id field.
		return s.notificationSuccessResponse(map[string]interface{}{})
	default:
		return s.errorResponse(request.ID, -32601, fmt.Sprintf("Method '%s' not found", request.Method), nil)
	}
}

func (s *Server) handleInitialize(request JSONRPCRequest) ([]byte, error) {
	version := s.version
	if version == "" {
		version = "1.0.0"
	}

	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.name,
			"version": version,
		},
	}

	return s.successResponse(request.ID, result)
}

func (s *Server) handleListTools(request JSONRPCRequest) ([]byte, error) {
	tools := s.ListTools()
	result := ListToolsResult{Tools: tools}
	return s.successResponse(request.ID, result)
}

func (s *Server) handleCallTool(ctx context.Context, request JSONRPCRequest) ([]byte, error) {
	// Extract tool name and arguments
	name, ok := request.Params["name"].(string)
	if !ok {
		return s.errorResponse(request.ID, -32602, "Invalid params: missing 'name'", nil)
	}

	args, _ := request.Params["arguments"].(map[string]interface{})
	if args == nil {
		args = make(map[string]interface{})
	}

	// Call the tool
	content, err := s.CallTool(ctx, name, args)
	if err != nil {
		// ToolErrorContent signals a tool-level error: emit a successful
		// response carrying the user's content with is_error=true.
		var tec *ToolErrorContent
		if errors.As(err, &tec) {
			content := tec.Content
			if len(content) == 0 {
				content = []Content{&TextContent{Type: ContentTypeText, Text: tec.Message}}
			}
			result := map[string]interface{}{
				"content":  normalizeToolResultContent(content),
				"is_error": true,
			}
			return s.successResponse(request.ID, result)
		}
		// Match Python SDK bridge behavior: plain handler errors are
		// returned as successful tool results with is_error=true and a
		// text content carrying err.Error().
		result := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": err.Error(),
				},
			},
			"is_error": true,
		}
		return s.successResponse(request.ID, result)
	}

	// Build result
	result := map[string]interface{}{
		"content": normalizeToolResultContent(content),
	}

	return s.successResponse(request.ID, result)
}

func buildToolMeta(annotations map[string]interface{}) map[string]interface{} {
	if annotations == nil {
		return nil
	}
	raw, ok := annotations["maxResultSizeChars"]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case int:
		return map[string]interface{}{"anthropic/maxResultSizeChars": value}
	case int32:
		return map[string]interface{}{"anthropic/maxResultSizeChars": value}
	case int64:
		return map[string]interface{}{"anthropic/maxResultSizeChars": value}
	case float64:
		return map[string]interface{}{"anthropic/maxResultSizeChars": int(value)}
	default:
		return nil
	}
}

func normalizeToolResultContent(content []Content) []Content {
	result := make([]Content, 0, len(content))
	for _, item := range content {
		switch value := item.(type) {
		case *ResourceLinkContent:
			parts := make([]string, 0, 3)
			if value.Name != "" {
				parts = append(parts, value.Name)
			}
			if value.URI != "" {
				parts = append(parts, value.URI)
			}
			if value.Description != "" {
				parts = append(parts, value.Description)
			}
			text := "Resource link"
			if len(parts) > 0 {
				text = strings.Join(parts, "\n")
			}
			result = append(result, &TextContent{Type: ContentTypeText, Text: text})
		case *ResourceContent:
			if value.Resource.Text == "" {
				continue
			}
			result = append(result, &TextContent{Type: ContentTypeText, Text: value.Resource.Text})
		default:
			result = append(result, item)
		}
	}
	return result
}

func (s *Server) successResponse(id interface{}, result interface{}) ([]byte, error) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(response)
}

func (s *Server) notificationSuccessResponse(result interface{}) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  result,
	})
}

func (s *Server) errorResponse(id interface{}, code int, message string, data interface{}) ([]byte, error) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(response)
}

// buildJSONSchema converts input schema to JSON Schema format.
func (s *Server) buildJSONSchema(schema interface{}) map[string]interface{} {
	switch v := schema.(type) {
	case map[string]interface{}:
		// Already a JSON schema or will be converted
		if _, hasType := v["type"]; hasType {
			if _, hasProps := v["properties"]; hasProps {
				// Already a complete JSON schema
				return v
			}
		}
		// Convert simple type map to JSON schema
		return s.convertSimpleSchemaToJSON(v)
	case map[string]Type:
		// Convert typed map to JSON schema
		return s.convertTypedMapToJSON(v)
	default:
		if converted := s.convertStructSchemaToJSON(v); converted != nil {
			return converted
		}
		// Return empty schema
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
}

func (s *Server) convertStructSchemaToJSON(schema interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}
	t := reflect.TypeOf(schema)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return schemaForType(t)
}

// Type represents a Go type for schema definition.
type Type interface{}

var (
	TypeString Type = "string"
	TypeInt    Type = "integer"
	TypeFloat  Type = "number"
	TypeBool   Type = "boolean"
	TypeObject Type = "object"
	TypeArray  Type = "array"
)

func (s *Server) convertTypedMapToJSON(typeMap map[string]Type) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0, len(typeMap))

	for name, typ := range typeMap {
		required = append(required, name)

		switch typ {
		case TypeString, "string":
			properties[name] = map[string]interface{}{"type": "string"}
		case TypeInt, "integer", "int":
			properties[name] = map[string]interface{}{"type": "integer"}
		case TypeFloat, "number", "float", "float64":
			properties[name] = map[string]interface{}{"type": "number"}
		case TypeBool, "boolean", "bool":
			properties[name] = map[string]interface{}{"type": "boolean"}
		case TypeObject, "object":
			properties[name] = map[string]interface{}{"type": "object"}
		case TypeArray, "array":
			properties[name] = map[string]interface{}{"type": "array"}
		default:
			// Default to string
			properties[name] = map[string]interface{}{"type": "string"}
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func (s *Server) convertSimpleSchemaToJSON(schema map[string]interface{}) map[string]interface{} {
	// Try to convert simple type specifications
	properties := make(map[string]interface{})
	required := make([]string, 0, len(schema))

	for name, value := range schema {
		required = append(required, name)

		// Check if it's a type string
		if typeStr, ok := value.(string); ok {
			properties[name] = map[string]interface{}{"type": typeStr}
		} else if typeMap, ok := value.(map[string]interface{}); ok {
			properties[name] = typeMap
		} else {
			// Default to string
			properties[name] = map[string]interface{}{"type": "string"}
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func schemaForType(t reflect.Type) map[string]interface{} {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]interface{}{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Map, reflect.Interface:
		return map[string]interface{}{"type": "object"}
	case reflect.Struct:
		properties := make(map[string]interface{})
		required := make([]string, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, optional, ok := schemaFieldName(field)
			if !ok {
				continue
			}
			fieldSchema := schemaForType(field.Type)
			if desc := field.Tag.Get("description"); desc != "" {
				fieldSchema["description"] = desc
			} else if desc := field.Tag.Get("jsonschema_description"); desc != "" {
				fieldSchema["description"] = desc
			}
			properties[name] = fieldSchema
			if !optional {
				required = append(required, name)
			}
		}

		schema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	default:
		return map[string]interface{}{"type": "string"}
	}
}

func schemaFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}

	name := field.Name
	optional := field.Type.Kind() == reflect.Pointer
	if tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			name = parts[0]
		}
		for _, part := range parts[1:] {
			if part == "omitempty" {
				optional = true
			}
		}
	}

	return name, optional, true
}

// Name returns the server name.
func (s *Server) Name() string {
	return s.name
}

// Version returns the server version.
func (s *Server) Version() string {
	return s.version
}
