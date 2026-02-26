package shared

import "encoding/json"

// Control Protocol message types and structures for SDK-CLI communication

// Control message types
const (
	ControlTypeRequest       = "control_request"
	ControlTypeResponse      = "control_response"
	ControlTypeCancelRequest = "control_cancel_request"
)

// Control request subtypes
const (
	ControlSubtypeInitialize        = "initialize"
	ControlSubtypeCanUseTool        = "can_use_tool"
	ControlSubtypeHookCallback      = "hook_callback"
	ControlSubtypeMCPMessage        = "mcp_message"
	ControlSubtypeMCPStatus         = "mcp_status"
	ControlSubtypeRewindFiles       = "rewind_files"
	ControlSubtypeSetPermissionMode = "set_permission_mode"
	ControlSubtypeSetModel          = "set_model"
	ControlSubtypeInterrupt         = "interrupt"
)

// Control response subtypes
const (
	ControlSubtypeSuccess = "success"
	ControlSubtypeError   = "error"
)

// ControlRequest represents a control request from SDK to CLI or vice versa.
type ControlRequest struct {
	Type      string         `json:"type"`       // "control_request"
	RequestID string         `json:"request_id"` // Unique request identifier
	Request   RequestPayload `json:"request"`    // The actual request data
}

// ControlResponse represents a control response.
type ControlResponse struct {
	Type     string          `json:"type"`     // "control_response"
	Response ResponsePayload `json:"response"` // The response data
}

// RequestPayload contains the actual request data.
type RequestPayload struct {
	Subtype string         `json:"subtype"` // Request subtype
	Data    map[string]any `json:"-"`       // Additional data (inline with request)
}

// MarshalJSON implements custom JSON marshaling for RequestPayload
func (r RequestPayload) MarshalJSON() ([]byte, error) {
	data := make(map[string]any)
	data["subtype"] = r.Subtype
	for k, v := range r.Data {
		data[k] = v
	}
	return marshalJSON(data)
}

// UnmarshalJSON implements custom JSON unmarshaling for RequestPayload
func (r *RequestPayload) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := unmarshalJSON(data, &raw); err != nil {
		return err
	}

	if subtype, ok := raw["subtype"].(string); ok {
		r.Subtype = subtype
	}

	r.Data = make(map[string]any)
	for k, v := range raw {
		if k != "subtype" {
			r.Data[k] = v
		}
	}

	return nil
}

// ResponsePayload contains the response data.
type ResponsePayload struct {
	Subtype   string         `json:"subtype"`    // "success" or "error"
	RequestID string         `json:"request_id"` // Corresponding request ID
	Response  map[string]any `json:"response,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// InitializeRequest represents initialization request data.
type InitializeRequest struct {
	Subtype string                         `json:"subtype"` // "initialize"
	Hooks   map[string][]HookMatcherConfig `json:"hooks,omitempty"`
	Agents  map[string]map[string]any      `json:"agents,omitempty"`
}

// HookMatcherConfig represents hook configuration sent to CLI.
type HookMatcherConfig struct {
	Matcher         *string  `json:"matcher"`
	HookCallbackIDs []string `json:"hookCallbackIds"`
	Timeout         *float64 `json:"timeout,omitempty"`
}

// CanUseToolRequest represents a tool permission request from CLI.
type CanUseToolRequest struct {
	Subtype               string         `json:"subtype"` // "can_use_tool"
	ToolName              string         `json:"tool_name"`
	Input                 map[string]any `json:"input"`
	PermissionSuggestions []any          `json:"permission_suggestions,omitempty"`
	BlockedPath           *string        `json:"blocked_path,omitempty"`
}

// HookCallbackRequest represents a hook callback request from CLI.
type HookCallbackRequest struct {
	Subtype    string         `json:"subtype"`     // "hook_callback"
	CallbackID string         `json:"callback_id"` // Hook callback identifier
	Input      map[string]any `json:"input"`       // Hook input data
	ToolUseID  *string        `json:"tool_use_id,omitempty"`
}

// PermissionResponse represents the response to can_use_tool request.
type PermissionResponse struct {
	Behavior           string `json:"behavior"` // "allow" or "deny"
	UpdatedInput       any    `json:"updatedInput,omitempty"`
	UpdatedPermissions []any  `json:"updatedPermissions,omitempty"`
	Message            string `json:"message,omitempty"`
	Interrupt          bool   `json:"interrupt,omitempty"`
}

// HookCallbackResponse represents the response to hook_callback request.
type HookCallbackResponse map[string]any

// RewindFilesRequest represents a request to rewind tracked files to a specific user message state.
type RewindFilesRequest struct {
	Subtype       string `json:"subtype"`         // "rewind_files"
	UserMessageID string `json:"user_message_id"` // UUID of the user message to rewind to
}

// Helper functions for JSON marshaling/unmarshaling
func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
