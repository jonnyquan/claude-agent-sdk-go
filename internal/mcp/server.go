package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolHandler is a function that handles tool execution.
// It receives the tool arguments and returns the result content.
type ToolHandler func(ctx context.Context, args map[string]interface{}) ([]Content, error)

// ToolDefinition defines a tool with its handler.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema interface{} // Can be map[string]Type or map[string]interface{} (JSON Schema)
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
	case "tools/list":
		return s.handleListTools(request)
	case "tools/call":
		return s.handleCallTool(ctx, request)
	case "notifications/initialized":
		return s.successResponse(request.ID, map[string]interface{}{})
	default:
		return s.errorResponse(request.ID, -32601, fmt.Sprintf("Method '%s' not found", request.Method), nil)
	}
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
		return s.errorResponse(request.ID, -32603, fmt.Sprintf("Tool execution failed: %v", err), err)
	}

	// Build result
	result := map[string]interface{}{
		"content": content,
	}

	return s.successResponse(request.ID, result)
}

func (s *Server) successResponse(id interface{}, result interface{}) ([]byte, error) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return json.Marshal(response)
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
		// Return empty schema
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
}

// Type represents a Go type for schema definition.
type Type interface{}

var (
	TypeString  Type = "string"
	TypeInt     Type = "integer"
	TypeFloat   Type = "number"
	TypeBool    Type = "boolean"
	TypeObject  Type = "object"
	TypeArray   Type = "array"
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

// Name returns the server name.
func (s *Server) Name() string {
	return s.name
}

// Version returns the server version.
func (s *Server) Version() string {
	return s.version
}
