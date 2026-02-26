// Package parser provides JSON message parsing functionality with speculative parsing and buffer management.
package parser

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// isDebugMode checks if ANTHROPIC_LOG is set to "debug"
var isDebugMode = func() bool {
	logLevel := os.Getenv("ANTHROPIC_LOG")
	return strings.ToLower(logLevel) == "debug"
}()

// debugLog prints log only when ANTHROPIC_LOG=debug
func debugLog(format string, args ...interface{}) {
	if isDebugMode {
		log.Printf(format, args...)
	}
}

const (
	// MaxBufferSize is the maximum buffer size to prevent memory exhaustion (1MB).
	MaxBufferSize = 1024 * 1024
)

// Parser handles JSON message parsing with speculative parsing and buffer management.
// It implements the same speculative parsing strategy as the Python SDK.
type Parser struct {
	buffer        strings.Builder
	maxBufferSize int
	mu            sync.Mutex // Thread safety
}

// New creates a new JSON parser with default buffer size.
func New() *Parser {
	return &Parser{
		maxBufferSize: MaxBufferSize,
	}
}

// ProcessLine processes a line of JSON input with speculative parsing.
// Handles multiple JSON objects on single line and embedded newlines.
func (p *Parser) ProcessLine(line string) ([]shared.Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// ✅ FIX: Reset buffer at the start of each ProcessLine call
	// This ensures clean state and prevents buffer accumulation across multiple calls
	p.buffer.Reset()
	debugLog("[SDK-Parser] 🧹 Buffer reset at ProcessLine start")

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	// Debug: Log raw CLI output
	debugLog("[SDK-Parser] 📥 Raw CLI line: %s", line)

	var messages []shared.Message

	// Handle multiple JSON objects on single line by splitting on newlines
	jsonLines := strings.Split(line, "\n")
	for _, jsonLine := range jsonLines {
		jsonLine = strings.TrimSpace(jsonLine)
		if jsonLine == "" {
			continue
		}

		// Process each JSON line with speculative parsing (unlocked version)
		msg, err := p.processJSONLineUnlocked(jsonLine)
		if err != nil {
			return messages, err
		}
		if msg != nil {
			messages = append(messages, msg)
		}
	}

	// Debug: Log parsed messages
	debugLog("[SDK-Parser] 📤 Parsed %d message(s) from line", len(messages))
	for i, msg := range messages {
		debugLog("[SDK-Parser]   Message #%d: type=%T", i, msg)
	}

	return messages, nil
}

// ParseMessage parses a raw JSON object into the appropriate Message type.
// Implements type discrimination based on the "type" field.
func (p *Parser) ParseMessage(data map[string]any) (shared.Message, error) {
	// Debug: Log raw data structure
	dataJSON, _ := json.Marshal(data)
	debugLog("[SDK-Parser] 🔍 ParseMessage input: %s", string(dataJSON))

	msgType, ok := data["type"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("missing or invalid type field", data)
	}

	debugLog("[SDK-Parser] 🔍 Message type: %s", msgType)

	var msg shared.Message
	var err error

	switch msgType {
	case shared.MessageTypeUser:
		msg, err = p.parseUserMessage(data)
	case shared.MessageTypeAssistant:
		msg, err = p.parseAssistantMessage(data)
	case shared.MessageTypeSystem:
		msg, err = p.parseSystemMessage(data)
	case shared.MessageTypeResult:
		msg, err = p.parseResultMessage(data)
	case shared.MessageTypeStreamEvent:
		msg, err = p.parseStreamEvent(data)
	default:
		// Skip unknown message types for forward compatibility
		debugLog("[SDK-Parser] ⚠️ Skipping unknown message type: %s", msgType)
		return nil, nil
	}

	// Debug: Log parsed result
	if err != nil {
		debugLog("[SDK-Parser] ❌ ParseMessage error: %v", err)
	} else if msg != nil {
		parsedJSON, _ := json.Marshal(msg)
		debugLog("[SDK-Parser] ✅ ParseMessage output: %s", string(parsedJSON))
	}

	return msg, err
}

// Reset clears the internal buffer.
func (p *Parser) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buffer.Reset()
}

// BufferSize returns the current buffer size.
func (p *Parser) BufferSize() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buffer.Len()
}

// processJSONLine attempts to parse accumulated buffer as JSON using speculative parsing.
// This is the core of the speculative parsing strategy from the Python SDK.
func (p *Parser) processJSONLine(jsonLine string) (shared.Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.processJSONLineUnlocked(jsonLine)
}

// processJSONLineUnlocked is the unlocked version of processJSONLine.
// Must be called with mutex already held.
func (p *Parser) processJSONLineUnlocked(jsonLine string) (shared.Message, error) {
	debugLog("[SDK-Parser] 🔧 processJSONLineUnlocked: input length=%d", len(jsonLine))
	debugLog("[SDK-Parser] 🔧 Buffer before: length=%d", p.buffer.Len())

	p.buffer.WriteString(jsonLine)
	debugLog("[SDK-Parser] 🔧 Buffer after write: length=%d", p.buffer.Len())

	// Check buffer size limit
	if p.buffer.Len() > p.maxBufferSize {
		bufferSize := p.buffer.Len()
		p.buffer.Reset()
		return nil, shared.NewJSONDecodeError(
			"buffer overflow",
			0,
			fmt.Errorf("buffer size %d exceeds limit %d", bufferSize, p.maxBufferSize),
		)
	}

	// Attempt speculative JSON parsing
	var rawData map[string]any
	bufferContent := p.buffer.String()
	debugLog("[SDK-Parser] 🔧 Attempting to unmarshal buffer content (length=%d)", len(bufferContent))

	if err := json.Unmarshal([]byte(bufferContent), &rawData); err != nil {
		// JSON is incomplete - continue accumulating
		// This is NOT an error condition in speculative parsing!
		debugLog("[SDK-Parser] 🔧 JSON Unmarshal failed (incomplete): %v", err)
		return nil, nil
	}

	// Successfully parsed complete JSON - reset buffer and parse message
	debugLog("[SDK-Parser] 🔧 JSON Unmarshal succeeded, resetting buffer and parsing message")
	p.buffer.Reset()
	return p.ParseMessage(rawData)
}

// parseUserMessage parses a user message from raw JSON data.
func (p *Parser) parseUserMessage(data map[string]any) (*shared.UserMessage, error) {
	debugLog("[SDK-Parser] 👤 Parsing UserMessage...")

	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("user message missing message field", data)
	}

	content := messageData["content"]
	if content == nil {
		return nil, shared.NewMessageParseError("user message missing content field", data)
	}
	debugLog("[SDK-Parser] 👤 UserMessage content type: %T", content)

	// Parse uuid field (for file checkpointing support)
	var uuid *string
	if uuidVal, ok := data["uuid"].(string); ok {
		uuid = &uuidVal
	}

	// Parse parent_tool_use_id field (top-level)
	var parentToolUseID *string
	if ptuid, ok := data["parent_tool_use_id"].(string); ok {
		parentToolUseID = &ptuid
	}

	// Parse tool_use_result field (top-level, matching Python parser)
	var toolUseResult map[string]interface{}
	if tur, ok := data["tool_use_result"].(map[string]interface{}); ok {
		toolUseResult = tur
	} else if tur, ok := messageData["tool_use_result"].(map[string]interface{}); ok {
		// Fallback for compatibility with any older/non-standard payload shapes.
		toolUseResult = tur
	}

	// Handle both string content and array of content blocks
	switch c := content.(type) {
	case string:
		// String content - create directly
		debugLog("[SDK-Parser] 👤 UserMessage has string content: %q", c)
		return &shared.UserMessage{
			Content:         c,
			UUID:            uuid,
			ParentToolUseID: parentToolUseID,
			ToolUseResult:   toolUseResult,
		}, nil
	case []any:
		// Array of content blocks
		debugLog("[SDK-Parser] 👤 UserMessage has %d content block(s)", len(c))
		var blocks []shared.ContentBlock
		for i, blockData := range c {
			block, err := p.parseContentBlock(blockData)
			if err != nil {
				return nil, fmt.Errorf("failed to parse content block %d: %w", i, err)
			}
			if block != nil {
				blocks = append(blocks, block)
				debugLog("[SDK-Parser] 👤   Block #%d: type=%T", i, block)
			}
		}
		return &shared.UserMessage{
			Content:         blocks,
			UUID:            uuid,
			ParentToolUseID: parentToolUseID,
			ToolUseResult:   toolUseResult,
		}, nil
	default:
		return nil, shared.NewMessageParseError("invalid user message content type", data)
	}
}

// parseAssistantMessage parses an assistant message from raw JSON data.
func (p *Parser) parseAssistantMessage(data map[string]any) (*shared.AssistantMessage, error) {
	debugLog("[SDK-Parser] 🤖 Parsing AssistantMessage...")

	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("assistant message missing message field", data)
	}

	contentArray, ok := messageData["content"].([]any)
	if !ok {
		return nil, shared.NewMessageParseError("assistant message content must be array", data)
	}
	debugLog("[SDK-Parser] 🤖 AssistantMessage has %d content block(s)", len(contentArray))

	model, ok := messageData["model"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("assistant message missing model field", data)
	}
	debugLog("[SDK-Parser] 🤖 AssistantMessage model: %s", model)

	var blocks []shared.ContentBlock
	for i, blockData := range contentArray {
		block, err := p.parseContentBlock(blockData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse content block %d: %w", i, err)
		}
		if block != nil {
			blocks = append(blocks, block)
			debugLog("[SDK-Parser] 🤖   Block #%d: type=%T", i, block)
		}
	}

	// Parse error field from top-level data (for rate limit detection, etc.)
	var errorField *shared.AssistantMessageError
	if errVal, ok := data["error"].(string); ok {
		errType := shared.AssistantMessageError(errVal)
		errorField = &errType
	}

	// Parse parent_tool_use_id field from top-level data
	var parentToolUseID *string
	if ptuid, ok := data["parent_tool_use_id"].(string); ok {
		parentToolUseID = &ptuid
	}

	debugLog("[SDK-Parser] 🤖 AssistantMessage parsed successfully")
	return &shared.AssistantMessage{
		Content:         blocks,
		Model:           model,
		ParentToolUseID: parentToolUseID,
		Error:           errorField,
	}, nil
}

// parseSystemMessage parses a system message from raw JSON data.
func (p *Parser) parseSystemMessage(data map[string]any) (*shared.SystemMessage, error) {
	subtype, ok := data["subtype"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("system message missing subtype field", data)
	}

	return &shared.SystemMessage{
		Subtype: subtype,
		Data:    data, // Preserve all original data
	}, nil
}

// parseResultMessage parses a result message from raw JSON data.
func (p *Parser) parseResultMessage(data map[string]any) (*shared.ResultMessage, error) {
	debugLog("[SDK-Parser] ✅ Parsing ResultMessage...")

	result := &shared.ResultMessage{}

	// Required fields with validation
	if subtype, ok := data["subtype"].(string); ok {
		result.Subtype = subtype
	} else {
		return nil, shared.NewMessageParseError("result message missing subtype field", data)
	}

	if durationMS, ok := data["duration_ms"].(float64); ok {
		result.DurationMs = int(durationMS)
	} else {
		return nil, shared.NewMessageParseError("result message missing or invalid duration_ms field", data)
	}

	if durationAPIMS, ok := data["duration_api_ms"].(float64); ok {
		result.DurationAPIMs = int(durationAPIMS)
	} else {
		return nil, shared.NewMessageParseError("result message missing or invalid duration_api_ms field", data)
	}

	if isError, ok := data["is_error"].(bool); ok {
		result.IsError = isError
	} else {
		return nil, shared.NewMessageParseError("result message missing or invalid is_error field", data)
	}

	if numTurns, ok := data["num_turns"].(float64); ok {
		result.NumTurns = int(numTurns)
	} else {
		return nil, shared.NewMessageParseError("result message missing or invalid num_turns field", data)
	}

	if sessionID, ok := data["session_id"].(string); ok {
		result.SessionID = sessionID
	} else {
		return nil, shared.NewMessageParseError("result message missing session_id field", data)
	}

	// Optional fields (no validation errors if missing)
	if totalCostUSD, ok := data["total_cost_usd"].(float64); ok {
		result.TotalCostUSD = &totalCostUSD
	}

	if usage, ok := data["usage"].(map[string]any); ok {
		result.Usage = &usage
	}

	if resultData, ok := data["result"]; ok {
		if resultStr, ok := resultData.(string); ok {
			result.Result = &resultStr
		}
	}

	// Parse structured_output (optional)
	if structuredOutput, ok := data["structured_output"]; ok {
		result.StructuredOutput = structuredOutput
	}

	debugLog("[SDK-Parser] ✅ ResultMessage parsed: subtype=%s, session_id=%s, is_error=%v",
		result.Subtype, result.SessionID, result.IsError)
	return result, nil
}

// parseStreamEvent parses a stream event from raw JSON data.
func (p *Parser) parseStreamEvent(data map[string]any) (*shared.StreamEvent, error) {
	debugLog("[SDK-Parser] 📡 Parsing StreamEvent...")

	uuid, ok := data["uuid"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("stream_event missing uuid field", data)
	}

	sessionID, ok := data["session_id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("stream_event missing session_id field", data)
	}

	event, ok := data["event"].(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("stream_event missing event field", data)
	}

	var parentToolUseID *string
	if ptuid, ok := data["parent_tool_use_id"].(string); ok {
		parentToolUseID = &ptuid
	}

	return &shared.StreamEvent{
		UUID:            uuid,
		SessionID:       sessionID,
		Event:           event,
		ParentToolUseID: parentToolUseID,
	}, nil
}

// parseContentBlock parses a content block based on its type field.
func (p *Parser) parseContentBlock(blockData any) (shared.ContentBlock, error) {
	data, ok := blockData.(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("content block must be an object", blockData)
	}

	blockType, ok := data["type"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("content block missing type field", data)
	}

	switch blockType {
	case shared.ContentBlockTypeText:
		return p.parseTextBlock(data)
	case shared.ContentBlockTypeThinking:
		return p.parseThinkingBlock(data)
	case shared.ContentBlockTypeToolUse:
		return p.parseToolUseBlock(data)
	case shared.ContentBlockTypeToolResult:
		return p.parseToolResultBlock(data)
	default:
		// Skip unknown content block types (e.g., server_tool_use) to match Python SDK behavior
		debugLog("[SDK-Parser] ⚠️ Skipping unknown content block type: %s", blockType)
		return nil, nil
	}
}

func (p *Parser) parseTextBlock(data map[string]any) (shared.ContentBlock, error) {
	text, ok := data["text"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("text block missing text field", data)
	}
	return &shared.TextBlock{
		Type: shared.ContentBlockTypeText,
		Text: text,
	}, nil
}

func (p *Parser) parseThinkingBlock(data map[string]any) (shared.ContentBlock, error) {
	thinking, ok := data["thinking"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("thinking block missing thinking field", data)
	}
	signature, ok := data["signature"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("thinking block missing signature field", data)
	}
	return &shared.ThinkingBlock{
		Type:      shared.ContentBlockTypeThinking,
		Thinking:  thinking,
		Signature: signature,
	}, nil
}

func (p *Parser) parseToolUseBlock(data map[string]any) (shared.ContentBlock, error) {
	id, ok := data["id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("tool_use block missing id field", data)
	}
	name, ok := data["name"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("tool_use block missing name field", data)
	}
	input, _ := data["input"].(map[string]any) // Optional field
	if input == nil {
		input = make(map[string]any)
	}
	return &shared.ToolUseBlock{
		Type:  shared.ContentBlockTypeToolUse,
		ID:    id,
		Name:  name,
		Input: input,
	}, nil
}

func (p *Parser) parseToolResultBlock(data map[string]any) (shared.ContentBlock, error) {
	toolUseID, ok := data["tool_use_id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("tool_result block missing tool_use_id field", data)
	}

	var isError *bool
	if isErrorValue, exists := data["is_error"]; exists {
		if b, ok := isErrorValue.(bool); ok {
			isError = &b
		}
	}

	return &shared.ToolResultBlock{
		Type:      shared.ContentBlockTypeToolResult,
		ToolUseID: toolUseID,
		Content:   data["content"],
		IsError:   isError,
	}, nil
}

// ParseMessages is a convenience function to parse multiple JSON lines.
func ParseMessages(lines []string) ([]shared.Message, error) {
	parser := New()
	var allMessages []shared.Message

	for i, line := range lines {
		messages, err := parser.ProcessLine(line)
		if err != nil {
			return allMessages, fmt.Errorf("error parsing line %d: %w", i, err)
		}
		allMessages = append(allMessages, messages...)
	}

	return allMessages, nil
}
