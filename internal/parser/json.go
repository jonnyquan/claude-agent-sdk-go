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
	case shared.MessageTypeRateLimitEvent:
		msg, err = p.parseRateLimitEvent(data)
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
		Usage:           mapValue(messageData, "usage"),
		MessageID:       stringPtr(messageData, "id"),
		StopReason:      stringPtr(messageData, "stop_reason"),
		SessionID:       stringPtr(data, "session_id"),
		UUID:            stringPtr(data, "uuid"),
	}, nil
}

// parseSystemMessage parses a system message from raw JSON data.
func (p *Parser) parseSystemMessage(data map[string]any) (shared.Message, error) {
	subtype, ok := data["subtype"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("system message missing subtype field", data)
	}

	// Hook events (emitted when IncludeHookEvents is enabled) arrive as
	// system messages with subtype hook_started or hook_response. Route
	// them to HookEventMessage before the switch below.
	if subtype == "hook_started" || subtype == "hook_response" {
		base := shared.SystemMessage{Subtype: subtype, Data: data}
		hookEventName := ""
		if v, ok := data["hook_event"].(string); ok {
			hookEventName = v
		} else if v, ok := data["hook_name"].(string); ok {
			hookEventName = v
		} else if v, ok := data["hook_event_name"].(string); ok {
			hookEventName = v
		}
		msg := &shared.HookEventMessage{
			SystemMessage: base,
			HookEventName: hookEventName,
		}
		if v, ok := data["session_id"].(string); ok {
			msg.SessionID = &v
		}
		if v, ok := data["uuid"].(string); ok {
			msg.UUID = &v
		}
		return msg, nil
	}

	base := shared.SystemMessage{
		Subtype: subtype,
		Data:    data,
	}

	switch subtype {
	case "task_started":
		taskID, ok := data["task_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing task_id field", data)
		}
		description, ok := data["description"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing description field", data)
		}
		uuid, ok := data["uuid"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing uuid field", data)
		}
		sessionID, ok := data["session_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing session_id field", data)
		}
		return &shared.TaskStartedMessage{
			SystemMessage: base,
			TaskID:        taskID,
			Description:   description,
			UUID:          uuid,
			SessionID:     sessionID,
			ToolUseID:     stringPtr(data, "tool_use_id"),
			TaskType:      stringPtr(data, "task_type"),
		}, nil
	case "task_progress":
		taskID, ok := data["task_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing task_id field", data)
		}
		description, ok := data["description"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing description field", data)
		}
		uuid, ok := data["uuid"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing uuid field", data)
		}
		sessionID, ok := data["session_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing session_id field", data)
		}
		usageMap, ok := data["usage"].(map[string]any)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing usage field", data)
		}
		usage, err := parseTaskUsage(usageMap, data)
		if err != nil {
			return nil, err
		}
		return &shared.TaskProgressMessage{
			SystemMessage: base,
			TaskID:        taskID,
			Description:   description,
			Usage:         usage,
			UUID:          uuid,
			SessionID:     sessionID,
			ToolUseID:     stringPtr(data, "tool_use_id"),
			LastToolName:  stringPtr(data, "last_tool_name"),
		}, nil
	case "task_notification":
		taskID, ok := data["task_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing task_id field", data)
		}
		status, ok := data["status"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing status field", data)
		}
		outputFile, ok := data["output_file"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing output_file field", data)
		}
		summary, ok := data["summary"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing summary field", data)
		}
		uuid, ok := data["uuid"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing uuid field", data)
		}
		sessionID, ok := data["session_id"].(string)
		if !ok {
			return nil, shared.NewMessageParseError("system message missing session_id field", data)
		}
		var usage *shared.TaskUsage
		if usageMap, ok := data["usage"].(map[string]any); ok {
			parsedUsage, err := parseTaskUsage(usageMap, data)
			if err != nil {
				return nil, err
			}
			usage = &parsedUsage
		}
		return &shared.TaskNotificationMessage{
			SystemMessage: base,
			TaskID:        taskID,
			Status:        shared.TaskNotificationStatus(status),
			OutputFile:    outputFile,
			Summary:       summary,
			UUID:          uuid,
			SessionID:     sessionID,
			ToolUseID:     stringPtr(data, "tool_use_id"),
			Usage:         usage,
		}, nil
	case "mirror_error":
		// SDK-synthesized via reportMirrorError — never emitted by the CLI subprocess.
		errMsg := ""
		if s, ok := data["error"].(string); ok {
			errMsg = s
		}
		mirror := &shared.MirrorErrorMessage{
			SystemMessage: base,
			Error:         errMsg,
		}
		if keyMap, ok := data["key"].(map[string]any); ok {
			key := &shared.SessionKey{}
			if v, ok := keyMap["project_key"].(string); ok {
				key.ProjectKey = v
			}
			if v, ok := keyMap["session_id"].(string); ok {
				key.SessionID = v
			}
			if v, ok := keyMap["subpath"].(string); ok {
				key.Subpath = v
			}
			mirror.Key = key
		}
		return mirror, nil
	default:
		return &base, nil
	}
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

	if stopReason, ok := data["stop_reason"].(string); ok {
		result.StopReason = &stopReason
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
	if modelUsage, ok := data["modelUsage"].(map[string]any); ok {
		result.ModelUsage = modelUsage
	}
	if permissionDenials, ok := data["permission_denials"].([]any); ok {
		result.PermissionDenials = permissionDenials
	}
	if errorsValue, ok := data["errors"].([]any); ok {
		errors := make([]string, 0, len(errorsValue))
		for _, item := range errorsValue {
			if s, ok := item.(string); ok {
				errors = append(errors, s)
			}
		}
		result.Errors = errors
	}
	// api_error_status: HTTP status of failing API call (CLI v2.1.110+).
	if status, ok := toInt(data["api_error_status"]); ok {
		result.APIErrorStatus = &status
	}
	// deferred_tool_use: present when a PreToolUse hook returned permissionDecision="defer".
	if deferred, ok := data["deferred_tool_use"].(map[string]any); ok {
		dtu := &shared.DeferredToolUse{}
		if v, ok := deferred["id"].(string); ok {
			dtu.ID = v
		}
		if v, ok := deferred["name"].(string); ok {
			dtu.Name = v
		}
		if v, ok := deferred["input"].(map[string]any); ok {
			dtu.Input = v
		}
		result.DeferredToolUse = dtu
	}
	if uuid, ok := data["uuid"].(string); ok {
		result.UUID = &uuid
	}

	debugLog("[SDK-Parser] ✅ ResultMessage parsed: subtype=%s, session_id=%s, is_error=%v",
		result.Subtype, result.SessionID, result.IsError)
	return result, nil
}

// parseRateLimitEvent parses a rate_limit_event from raw JSON data.
func (p *Parser) parseRateLimitEvent(data map[string]any) (*shared.RateLimitEvent, error) {
	infoMap, ok := data["rate_limit_info"].(map[string]any)
	if !ok {
		return nil, shared.NewMessageParseError("rate_limit_event missing rate_limit_info field", data)
	}
	status, ok := infoMap["status"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("rate_limit_event missing status field", data)
	}
	uuid, ok := data["uuid"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("rate_limit_event missing uuid field", data)
	}
	sessionID, ok := data["session_id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("rate_limit_event missing session_id field", data)
	}

	info := shared.RateLimitInfo{
		Status: shared.RateLimitStatus(status),
		Raw:    infoMap,
	}
	if resetsAt, ok := toInt64(infoMap["resetsAt"]); ok {
		info.ResetsAt = &resetsAt
	}
	if rateLimitType, ok := infoMap["rateLimitType"].(string); ok {
		value := shared.RateLimitType(rateLimitType)
		info.RateLimitType = &value
	}
	if utilization, ok := infoMap["utilization"].(float64); ok {
		info.Utilization = &utilization
	}
	if overageStatus, ok := infoMap["overageStatus"].(string); ok {
		value := shared.RateLimitStatus(overageStatus)
		info.OverageStatus = &value
	}
	if overageResetsAt, ok := toInt64(infoMap["overageResetsAt"]); ok {
		info.OverageResetsAt = &overageResetsAt
	}
	if overageDisabledReason, ok := infoMap["overageDisabledReason"].(string); ok {
		info.OverageDisabledReason = &overageDisabledReason
	}

	return &shared.RateLimitEvent{
		MessageType:   shared.MessageTypeRateLimitEvent,
		RateLimitInfo: info,
		UUID:          uuid,
		SessionID:     sessionID,
	}, nil
}

func parseTaskUsage(data map[string]any, raw map[string]any) (shared.TaskUsage, error) {
	totalTokens, ok := toInt(data["total_tokens"])
	if !ok {
		return shared.TaskUsage{}, shared.NewMessageParseError("task usage missing total_tokens field", raw)
	}
	toolUses, ok := toInt(data["tool_uses"])
	if !ok {
		return shared.TaskUsage{}, shared.NewMessageParseError("task usage missing tool_uses field", raw)
	}
	durationMS, ok := toInt(data["duration_ms"])
	if !ok {
		return shared.TaskUsage{}, shared.NewMessageParseError("task usage missing duration_ms field", raw)
	}
	return shared.TaskUsage{
		TotalTokens: totalTokens,
		ToolUses:    toolUses,
		DurationMS:  durationMS,
	}, nil
}

func stringPtr(data map[string]any, key string) *string {
	if value, ok := data[key].(string); ok {
		return &value
	}
	return nil
}

func mapValue(data map[string]any, key string) map[string]any {
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	return nil
}

func toInt(value any) (int, bool) {
	if f, ok := value.(float64); ok {
		return int(f), true
	}
	if i, ok := value.(int); ok {
		return i, true
	}
	return 0, false
}

func toInt64(value any) (int64, bool) {
	if f, ok := value.(float64); ok {
		return int64(f), true
	}
	if i, ok := value.(int64); ok {
		return i, true
	}
	if i, ok := value.(int); ok {
		return int64(i), true
	}
	return 0, false
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
	case shared.ContentBlockTypeServerToolUse:
		return p.parseServerToolUseBlock(data)
	case shared.ContentBlockTypeAdvisorToolResult:
		return p.parseServerToolResultBlock(data)
	default:
		// Skip unknown content block types to match Python SDK behavior.
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

func (p *Parser) parseServerToolUseBlock(data map[string]any) (shared.ContentBlock, error) {
	id, ok := data["id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("server_tool_use block missing id field", data)
	}
	name, ok := data["name"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("server_tool_use block missing name field", data)
	}
	input, _ := data["input"].(map[string]any)
	if input == nil {
		input = make(map[string]any)
	}
	return &shared.ServerToolUseBlock{
		Type:  shared.ContentBlockTypeServerToolUse,
		ID:    id,
		Name:  name,
		Input: input,
	}, nil
}

func (p *Parser) parseServerToolResultBlock(data map[string]any) (shared.ContentBlock, error) {
	toolUseID, ok := data["tool_use_id"].(string)
	if !ok {
		return nil, shared.NewMessageParseError("advisor_tool_result block missing tool_use_id field", data)
	}
	content, _ := data["content"].(map[string]any)
	if content == nil {
		content = make(map[string]any)
	}
	return &shared.ServerToolResultBlock{
		Type:      shared.ContentBlockTypeAdvisorToolResult,
		ToolUseID: toolUseID,
		Content:   content,
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
