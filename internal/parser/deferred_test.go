package parser

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestParseResultMessage_DeferredToolUse mirrors Python SDK's
// `test_parse_result_message_with_deferred_tool_use` — when a result
// carries a `deferred_tool_use` block (a PreToolUse hook returned
// `permissionDecision: "defer"`), the parser should populate
// `ResultMessage.DeferredToolUse` with id/name/input.
func TestParseResultMessage_DeferredToolUse(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     float64(1200),
		"duration_api_ms": float64(900),
		"is_error":        false,
		"num_turns":       float64(1),
		"session_id":      "session_123",
		"deferred_tool_use": map[string]any{
			"id":    "toolu_01abc",
			"name":  "Bash",
			"input": map[string]any{"command": "rm -rf /tmp/scratch"},
		},
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	rm, ok := msg.(*shared.ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}
	if rm.DeferredToolUse == nil {
		t.Fatal("expected DeferredToolUse to be populated")
	}
	if rm.DeferredToolUse.ID != "toolu_01abc" {
		t.Errorf("DeferredToolUse.ID = %q, want toolu_01abc", rm.DeferredToolUse.ID)
	}
	if rm.DeferredToolUse.Name != "Bash" {
		t.Errorf("DeferredToolUse.Name = %q, want Bash", rm.DeferredToolUse.Name)
	}
	got, _ := rm.DeferredToolUse.Input["command"].(string)
	if got != "rm -rf /tmp/scratch" {
		t.Errorf("DeferredToolUse.Input.command = %q, want rm -rf /tmp/scratch", got)
	}
}

// TestParseResultMessage_APIErrorStatus mirrors Python's
// `test_parse_result_message_with_api_error_status` — the HTTP status of
// a failing API call is surfaced on the result.
func TestParseResultMessage_APIErrorStatus(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":             "result",
		"subtype":          "success",
		"duration_ms":      float64(500),
		"duration_api_ms":  float64(400),
		"is_error":         true,
		"num_turns":        float64(1),
		"session_id":       "session_xyz",
		"api_error_status": float64(429),
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	rm, ok := msg.(*shared.ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}
	if rm.APIErrorStatus == nil {
		t.Fatal("expected APIErrorStatus to be populated")
	}
	if *rm.APIErrorStatus != 429 {
		t.Errorf("APIErrorStatus = %d, want 429", *rm.APIErrorStatus)
	}
}

// TestParseResultMessage_NoDeferredToolUse verifies the field stays nil
// when absent (Python parity: optional field).
func TestParseResultMessage_NoDeferredToolUse(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     float64(100),
		"duration_api_ms": float64(80),
		"is_error":        false,
		"num_turns":       float64(1),
		"session_id":      "session_no_deferred",
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	rm := msg.(*shared.ResultMessage)
	if rm.DeferredToolUse != nil {
		t.Errorf("expected DeferredToolUse=nil, got %+v", rm.DeferredToolUse)
	}
	if rm.APIErrorStatus != nil {
		t.Errorf("expected APIErrorStatus=nil, got %v", *rm.APIErrorStatus)
	}
}
