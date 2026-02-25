package shared

import (
	"encoding/json"
)

// Message type constants
const (
	MessageTypeUser        = "user"
	MessageTypeAssistant   = "assistant"
	MessageTypeSystem      = "system"
	MessageTypeResult      = "result"
	MessageTypeStreamEvent = "stream_event"
)

// Content block type constants
const (
	ContentBlockTypeText       = "text"
	ContentBlockTypeThinking   = "thinking"
	ContentBlockTypeToolUse    = "tool_use"
	ContentBlockTypeToolResult = "tool_result"
	// Note: Python SDK does not include "image" in ContentBlock types
)

// AssistantMessageError represents error types for assistant messages
type AssistantMessageError string

const (
	AssistantMessageErrorAuthenticationFailed AssistantMessageError = "authentication_failed"
	AssistantMessageErrorBillingError         AssistantMessageError = "billing_error"
	AssistantMessageErrorRateLimit            AssistantMessageError = "rate_limit"
	AssistantMessageErrorInvalidRequest       AssistantMessageError = "invalid_request"
	AssistantMessageErrorServerError          AssistantMessageError = "server_error"
	AssistantMessageErrorUnknown              AssistantMessageError = "unknown"
)

// Message represents any message type in the Claude Code protocol.
type Message interface {
	Type() string
}

// ContentBlock represents any content block within a message.
type ContentBlock interface {
	BlockType() string
}

// UserMessage represents a message from the user.
type UserMessage struct {
	MessageType     string                 `json:"type"`
	Content         interface{}            `json:"content"` // string or []ContentBlock
	UUID            *string                `json:"uuid,omitempty"`
	ParentToolUseID *string                `json:"parent_tool_use_id,omitempty"`
	ToolUseResult   map[string]interface{} `json:"tool_use_result,omitempty"`
}

// Type returns the message type for UserMessage.
func (m *UserMessage) Type() string {
	return MessageTypeUser
}

// MarshalJSON implements custom JSON marshaling for UserMessage
func (m *UserMessage) MarshalJSON() ([]byte, error) {
	type userMessage UserMessage
	temp := struct {
		Type string `json:"type"`
		*userMessage
	}{
		Type:        MessageTypeUser,
		userMessage: (*userMessage)(m),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for UserMessage
func (m *UserMessage) UnmarshalJSON(data []byte) error {
	// Parse the basic structure first
	var raw struct {
		Type            string          `json:"type"`
		Content         json.RawMessage `json:"content"`
		UUID            *string         `json:"uuid,omitempty"`
		ParentToolUseID *string         `json:"parent_tool_use_id,omitempty"`
	}
	
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	m.MessageType = MessageTypeUser
	m.UUID = raw.UUID
	m.ParentToolUseID = raw.ParentToolUseID
	
	// Try to unmarshal content as string first
	var strContent string
	if err := json.Unmarshal(raw.Content, &strContent); err == nil {
		m.Content = strContent
		return nil
	}
	
	// If not a string, try to unmarshal as array of ContentBlocks
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(raw.Content, &rawBlocks); err != nil {
		return err
	}
	
	blocks := make([]ContentBlock, 0, len(rawBlocks))
	for _, blockData := range rawBlocks {
		block, err := unmarshalContentBlock(blockData)
		if err != nil {
			return err
		}
		if block != nil {
			blocks = append(blocks, block)
		}
	}
	
	m.Content = blocks
	return nil
}

// AssistantMessage represents a message from the assistant.
type AssistantMessage struct {
	MessageType     string                  `json:"type"`
	Content         []ContentBlock          `json:"content"`
	Model           string                  `json:"model"`
	ParentToolUseID *string                 `json:"parent_tool_use_id,omitempty"`
	Error           *AssistantMessageError  `json:"error,omitempty"`
}

// Type returns the message type for AssistantMessage.
func (m *AssistantMessage) Type() string {
	return MessageTypeAssistant
}

// MarshalJSON implements custom JSON marshaling for AssistantMessage
func (m *AssistantMessage) MarshalJSON() ([]byte, error) {
	type assistantMessage AssistantMessage
	temp := struct {
		Type string `json:"type"`
		*assistantMessage
	}{
		Type:             MessageTypeAssistant,
		assistantMessage: (*assistantMessage)(m),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for AssistantMessage
func (m *AssistantMessage) UnmarshalJSON(data []byte) error {
	// Parse the basic structure first
	var raw struct {
		Type            string            `json:"type"`
		Content         []json.RawMessage `json:"content"`
		Model           string            `json:"model"`
		ParentToolUseID *string           `json:"parent_tool_use_id,omitempty"`
	}
	
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	// Set the basic fields
	m.MessageType = MessageTypeAssistant
	m.Model = raw.Model
	m.ParentToolUseID = raw.ParentToolUseID
	
	// Parse each content block based on its type
	m.Content = make([]ContentBlock, 0, len(raw.Content))
	for _, blockData := range raw.Content {
		block, err := unmarshalContentBlock(blockData)
		if err != nil {
			return err
		}
		if block != nil {
			m.Content = append(m.Content, block)
		}
	}
	
	return nil
}

// unmarshalContentBlock unmarshals a JSON block into the appropriate ContentBlock type
func unmarshalContentBlock(data []byte) (ContentBlock, error) {
	var meta struct {
		Type string `json:"type"`
	}
	
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	
	switch meta.Type {
	case ContentBlockTypeText:
		var block TextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return &block, nil
		
	case ContentBlockTypeThinking:
		var block ThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return &block, nil
		
	case ContentBlockTypeToolUse:
		var block ToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return &block, nil
		
	case ContentBlockTypeToolResult:
		var block ToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, err
		}
		return &block, nil
		
	default:
		// Unknown block type, skip it
		// Note: Python SDK does not include image blocks in ContentBlock types
		return nil, nil
	}
}

// SystemMessage represents a system message.
type SystemMessage struct {
	MessageType string         `json:"type"`
	Subtype     string         `json:"subtype"`
	Data        map[string]any `json:"-"` // Preserve all original data
}

// Type returns the message type for SystemMessage.
func (m *SystemMessage) Type() string {
	return MessageTypeSystem
}

// MarshalJSON implements custom JSON marshaling for SystemMessage
func (m *SystemMessage) MarshalJSON() ([]byte, error) {
	data := make(map[string]any)
	for k, v := range m.Data {
		data[k] = v
	}
	data["type"] = MessageTypeSystem
	data["subtype"] = m.Subtype
	return json.Marshal(data)
}

// UnmarshalJSON implements custom JSON unmarshaling for SystemMessage
func (m *SystemMessage) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	
	if subtype, ok := raw["subtype"].(string); ok {
		m.Subtype = subtype
	}
	
	m.Data = raw
	m.MessageType = MessageTypeSystem
	return nil
}

// ResultMessage represents the final result of a conversation turn.
type ResultMessage struct {
	MessageType      string          `json:"type"`
	Subtype          string          `json:"subtype"`
	DurationMs       int             `json:"duration_ms"`
	DurationAPIMs    int             `json:"duration_api_ms"`
	IsError          bool            `json:"is_error"`
	NumTurns         int             `json:"num_turns"`
	SessionID        string          `json:"session_id"`
	TotalCostUSD     *float64        `json:"total_cost_usd,omitempty"`
	Usage            *map[string]any `json:"usage,omitempty"`
	Result           *string         `json:"result,omitempty"`           // Note: Python SDK uses string type
	StructuredOutput interface{}     `json:"structured_output,omitempty"` // Structured output when using JSON schema
}

// Type returns the message type for ResultMessage.
func (m *ResultMessage) Type() string {
	return MessageTypeResult
}

// MarshalJSON implements custom JSON marshaling for ResultMessage
func (m *ResultMessage) MarshalJSON() ([]byte, error) {
	type resultMessage ResultMessage
	temp := struct {
		Type string `json:"type"`
		*resultMessage
	}{
		Type:          MessageTypeResult,
		resultMessage: (*resultMessage)(m),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for ResultMessage
func (m *ResultMessage) UnmarshalJSON(data []byte) error {
	type resultMessage ResultMessage
	temp := (*resultMessage)(m)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	m.MessageType = MessageTypeResult
	return nil
}

// TextBlock represents text content.
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// BlockType returns the content block type for TextBlock.
func (b *TextBlock) BlockType() string {
	return ContentBlockTypeText
}

// MarshalJSON implements custom JSON marshaling for TextBlock
func (b *TextBlock) MarshalJSON() ([]byte, error) {
	type textBlock TextBlock
	temp := struct {
		Type string `json:"type"`
		*textBlock
	}{
		Type:      ContentBlockTypeText,
		textBlock: (*textBlock)(b),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for TextBlock
func (b *TextBlock) UnmarshalJSON(data []byte) error {
	type textBlock TextBlock
	temp := (*textBlock)(b)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	b.Type = ContentBlockTypeText
	return nil
}

// ThinkingBlock represents thinking content with signature.
type ThinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// BlockType returns the content block type for ThinkingBlock.
func (b *ThinkingBlock) BlockType() string {
	return ContentBlockTypeThinking
}

// MarshalJSON implements custom JSON marshaling for ThinkingBlock
func (b *ThinkingBlock) MarshalJSON() ([]byte, error) {
	type thinkingBlock ThinkingBlock
	temp := struct {
		Type string `json:"type"`
		*thinkingBlock
	}{
		Type:          ContentBlockTypeThinking,
		thinkingBlock: (*thinkingBlock)(b),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for ThinkingBlock
func (b *ThinkingBlock) UnmarshalJSON(data []byte) error {
	type thinkingBlock ThinkingBlock
	temp := (*thinkingBlock)(b)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	b.Type = ContentBlockTypeThinking
	return nil
}

// ToolUseBlock represents a tool use request.
type ToolUseBlock struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"` // Note: Python SDK uses "id", not "tool_use_id"
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

// BlockType returns the content block type for ToolUseBlock.
func (b *ToolUseBlock) BlockType() string {
	return ContentBlockTypeToolUse
}

// MarshalJSON implements custom JSON marshaling for ToolUseBlock
func (b *ToolUseBlock) MarshalJSON() ([]byte, error) {
	type toolUseBlock ToolUseBlock
	temp := struct {
		Type string `json:"type"`
		*toolUseBlock
	}{
		Type:         ContentBlockTypeToolUse,
		toolUseBlock: (*toolUseBlock)(b),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for ToolUseBlock
func (b *ToolUseBlock) UnmarshalJSON(data []byte) error {
	type toolUseBlock ToolUseBlock
	temp := (*toolUseBlock)(b)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	b.Type = ContentBlockTypeToolUse
	return nil
}

// ToolResultBlock represents the result of a tool use.
type ToolResultBlock struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"` // string or structured data
	IsError   *bool       `json:"is_error,omitempty"`
}

// BlockType returns the content block type for ToolResultBlock.
func (b *ToolResultBlock) BlockType() string {
	return ContentBlockTypeToolResult
}

// MarshalJSON implements custom JSON marshaling for ToolResultBlock
func (b *ToolResultBlock) MarshalJSON() ([]byte, error) {
	type toolResultBlock ToolResultBlock
	temp := struct {
		Type string `json:"type"`
		*toolResultBlock
	}{
		Type:            ContentBlockTypeToolResult,
		toolResultBlock: (*toolResultBlock)(b),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for ToolResultBlock
func (b *ToolResultBlock) UnmarshalJSON(data []byte) error {
	type toolResultBlock ToolResultBlock
	temp := (*toolResultBlock)(b)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	b.Type = ContentBlockTypeToolResult
	return nil
}

// StreamEvent represents a stream event for partial message updates during streaming.
type StreamEvent struct {
	MessageType     string         `json:"type"`
	UUID            string         `json:"uuid"`
	SessionID       string         `json:"session_id"`
	Event           map[string]any `json:"event"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
}

// Type returns the message type for StreamEvent.
func (m *StreamEvent) Type() string {
	return MessageTypeStreamEvent
}

// MarshalJSON implements custom JSON marshaling for StreamEvent
func (m *StreamEvent) MarshalJSON() ([]byte, error) {
	type streamEvent StreamEvent
	temp := struct {
		Type string `json:"type"`
		*streamEvent
	}{
		Type:        MessageTypeStreamEvent,
		streamEvent: (*streamEvent)(m),
	}
	return json.Marshal(temp)
}

// UnmarshalJSON implements custom JSON unmarshaling for StreamEvent
func (m *StreamEvent) UnmarshalJSON(data []byte) error {
	type streamEvent StreamEvent
	temp := (*streamEvent)(m)
	if err := json.Unmarshal(data, temp); err != nil {
		return err
	}
	m.MessageType = MessageTypeStreamEvent
	return nil
}

// Note: ImageBlock is NOT part of Python SDK's ContentBlock types
// The Python SDK only includes: TextBlock, ThinkingBlock, ToolUseBlock, ToolResultBlock
// ImageBlock has been removed to maintain compatibility with Python SDK
