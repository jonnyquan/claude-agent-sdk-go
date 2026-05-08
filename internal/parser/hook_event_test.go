package parser

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestParseHookEventStarted mirrors Python's
// `test_parse_hook_event_message` — `system/hook_started` parses into
// HookEventMessage with hook_event_name extracted from the wire payload.
func TestParseHookEventStarted(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":       "system",
		"subtype":    "hook_started",
		"hook_event": "PreToolUse",
		"hook_name":  "PreToolUse",
		"session_id": "sess-123",
		"uuid":       "uuid-456",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "ls"},
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	hem, ok := msg.(*shared.HookEventMessage)
	if !ok {
		t.Fatalf("expected *HookEventMessage, got %T", msg)
	}
	if hem.Subtype != "hook_started" {
		t.Errorf("Subtype = %q, want hook_started", hem.Subtype)
	}
	if hem.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q, want PreToolUse", hem.HookEventName)
	}
	if hem.SessionID == nil || *hem.SessionID != "sess-123" {
		t.Errorf("SessionID mismatch: %v", hem.SessionID)
	}
	if hem.UUID == nil || *hem.UUID != "uuid-456" {
		t.Errorf("UUID mismatch: %v", hem.UUID)
	}
	if got, _ := hem.Data["tool_name"].(string); got != "Bash" {
		t.Errorf("Data tool_name = %q, want Bash", got)
	}
}

// TestParseHookEventResponse mirrors Python's
// `test_parse_hook_event_message_response` — `system/hook_response`
// parses into HookEventMessage with output/exit_code/outcome preserved
// in Data.
func TestParseHookEventResponse(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":       "system",
		"subtype":    "hook_response",
		"hook_event": "PostToolUse",
		"hook_name":  "PostToolUse",
		"session_id": "sess-123",
		"uuid":       "uuid-789",
		"output":     "",
		"exit_code":  float64(0),
		"outcome":    "success",
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	hem := msg.(*shared.HookEventMessage)
	if hem.Subtype != "hook_response" {
		t.Errorf("Subtype = %q, want hook_response", hem.Subtype)
	}
	if hem.HookEventName != "PostToolUse" {
		t.Errorf("HookEventName = %q, want PostToolUse", hem.HookEventName)
	}
	if hem.Data["output"] != "" {
		t.Errorf("Data output mismatch: %v", hem.Data["output"])
	}
	if hem.Data["outcome"] != "success" {
		t.Errorf("Data outcome mismatch: %v", hem.Data["outcome"])
	}
}

// TestParseHookEventMinimal mirrors Python's
// `test_parse_hook_event_message_minimal` — hook events without session
// id, uuid, or hook_event still parse (hook_name is the fallback).
func TestParseHookEventMinimal(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":      "system",
		"subtype":   "hook_started",
		"hook_name": "Stop",
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	hem := msg.(*shared.HookEventMessage)
	if hem.Subtype != "hook_started" {
		t.Errorf("Subtype = %q, want hook_started", hem.Subtype)
	}
	// Python falls back from hook_event → hook_name → hook_event_name;
	// missing hook_event but hook_name="Stop" should yield "Stop".
	if hem.HookEventName != "Stop" {
		t.Errorf("HookEventName = %q, want Stop (hook_name fallback)", hem.HookEventName)
	}
	if hem.SessionID != nil {
		t.Errorf("SessionID should be nil, got %v", *hem.SessionID)
	}
	if hem.UUID != nil {
		t.Errorf("UUID should be nil, got %v", *hem.UUID)
	}
}

// TestHookEventMessageIsSystemMessage verifies the embedded SystemMessage
// works correctly — Python's HookEventMessage subclasses SystemMessage so
// existing isinstance(msg, SystemMessage) checks continue to match.
func TestHookEventMessageIsSystemMessage(t *testing.T) {
	parser := New()
	data := map[string]any{
		"type":       "system",
		"subtype":    "hook_started",
		"hook_event": "PreToolUse",
	}
	msg, err := parser.ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	hem, ok := msg.(*shared.HookEventMessage)
	if !ok {
		t.Fatalf("expected *HookEventMessage")
	}
	// The embedded SystemMessage should be accessible — verify via its
	// promoted field.
	if hem.SystemMessage.Subtype != "hook_started" {
		t.Errorf("embedded SystemMessage.Subtype = %q, want hook_started", hem.SystemMessage.Subtype)
	}
}
