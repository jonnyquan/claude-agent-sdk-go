package parsing

import (
	"strings"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

const resultPrefix = `{"type":"result","subtype":"success","duration_ms":1,"duration_api_ms":1,` +
	`"is_error":false,"num_turns":1,"session_id":"s1"`

func parseOne(t *testing.T, line string) shared.Message {
	t.Helper()
	messages, err := New().ProcessLine(line)
	if err != nil {
		t.Fatalf("ProcessLine(%.80s) failed: %v", line, err)
	}
	if len(messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(messages))
	}
	return messages[0]
}

func TestResultMessageTerminalReason(t *testing.T) {
	msg := parseOne(t, resultPrefix+`,"terminal_reason":"aborted_streaming"}`)
	result, ok := msg.(*shared.ResultMessage)
	if !ok {
		t.Fatalf("got %T, want *shared.ResultMessage", msg)
	}
	if result.TerminalReason == nil || *result.TerminalReason != shared.TerminalReasonAbortedStreaming {
		t.Fatalf("TerminalReason = %v, want aborted_streaming", result.TerminalReason)
	}
	if !shared.IsAbortedTerminalReason(*result.TerminalReason) {
		t.Error("aborted_streaming should report as an aborted turn")
	}

	// Absent on older CLIs and on results that bypassed the query loop.
	plain := parseOne(t, resultPrefix+`}`).(*shared.ResultMessage)
	if plain.TerminalReason != nil {
		t.Errorf("TerminalReason = %v, want nil when unreported", *plain.TerminalReason)
	}
}

func TestResultMessageTypedModelUsageAndAPIErrorStatus(t *testing.T) {
	msg := parseOne(t, resultPrefix+
		`,"api_error_status":429`+
		`,"modelUsage":{"claude-opus-4-7":{"inputTokens":7,"costUSD":0.5,"provider":"bedrock"}}}`)
	result := msg.(*shared.ResultMessage)

	if result.APIErrorStatus == nil || *result.APIErrorStatus != 429 {
		t.Errorf("APIErrorStatus = %v, want 429", result.APIErrorStatus)
	}
	entry, ok := result.ModelUsage["claude-opus-4-7"]
	if !ok {
		t.Fatalf("ModelUsage missing entry: %v", result.ModelUsage)
	}
	if entry.InputTokens != 7 || entry.CostUSD != 0.5 || entry.Provider != "bedrock" {
		t.Errorf("unexpected ModelUsage entry: %+v", entry)
	}
}

func TestProcessLineSkipsNonJSONWithoutPoisoningTheNextLine(t *testing.T) {
	parser := New()

	// Non-JSON output some CLI builds write to stdout carries no message and
	// must not be accumulated (#347).
	messages, err := parser.ProcessLine("[SandboxDebug] starting sandbox")
	if err != nil {
		t.Fatalf("non-JSON line should be skipped, got error: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("got %d messages from a non-JSON line, want 0", len(messages))
	}

	// The next real message must still parse.
	if _, ok := parseOne(t, `{"type":"system","subtype":"status"}`).(*shared.SystemMessage); !ok {
		t.Error("a following system message should parse cleanly")
	}
}

func TestProcessLineReportsCorruptJSONInsteadOfDroppingIt(t *testing.T) {
	// The transport frames whole lines, so a line that looks like JSON but does
	// not parse is corrupt — no later data can complete it.
	_, err := New().ProcessLine(`{"type":"result","truncated":`)
	if err == nil {
		t.Fatal("expected a decode error for a truncated JSON line")
	}
	if _, ok := err.(*shared.JSONDecodeError); !ok {
		t.Fatalf("got %T, want *shared.JSONDecodeError", err)
	}
}

func TestProcessLineBoundsASingleMessage(t *testing.T) {
	oversized := "{" + strings.Repeat("x", MaxBufferSize+1)
	_, err := New().ProcessLine(oversized)
	if err == nil {
		t.Fatal("expected an error for an over-cap line")
	}
	if !strings.Contains(err.Error(), "buffer") {
		t.Errorf("error should name the buffer limit, got %q", err.Error())
	}
}

func TestProcessLineHandlesMultipleMessagesAndBlanks(t *testing.T) {
	messages, err := New().ProcessLine(
		`{"type":"system","subtype":"a"}` + "\n\n" + `{"type":"system","subtype":"b"}`)
	if err != nil {
		t.Fatalf("ProcessLine failed: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(messages))
	}
}

func TestParseHookEventMessage(t *testing.T) {
	// Emitted only when IncludeHookEvents is enabled. The CLI has spelled the
	// event-name key several ways across versions; all must be accepted.
	for _, key := range []string{"hook_event", "hook_name", "hook_event_name"} {
		line := `{"type":"system","subtype":"hook_started","` + key +
			`":"PreToolUse","session_id":"s1","uuid":"u1"}`
		msg, ok := parseOne(t, line).(*shared.HookEventMessage)
		if !ok {
			t.Fatalf("%s: expected *shared.HookEventMessage", key)
		}
		if msg.HookEventName != "PreToolUse" {
			t.Errorf("%s: HookEventName = %q, want PreToolUse", key, msg.HookEventName)
		}
		if msg.SessionID == nil || *msg.SessionID != "s1" {
			t.Errorf("%s: SessionID not populated", key)
		}
	}

	// hook_response is the completion phase of the same lifecycle.
	if _, ok := parseOne(t, `{"type":"system","subtype":"hook_response","hook_event":"Stop"}`).(*shared.HookEventMessage); !ok {
		t.Error("hook_response should parse as *shared.HookEventMessage")
	}
}

func TestParseMirrorErrorMessage(t *testing.T) {
	// SDK-synthesized when a SessionStore.Append fails. The dropped batch is
	// not retried, so this is the consumer's only signal.
	line := `{"type":"system","subtype":"mirror_error","error":"s3 timeout",` +
		`"key":{"project_key":"p1","session_id":"s1","subpath":"sub"}}`
	msg, ok := parseOne(t, line).(*shared.MirrorErrorMessage)
	if !ok {
		t.Fatalf("expected *shared.MirrorErrorMessage")
	}
	if msg.Error != "s3 timeout" {
		t.Errorf("Error = %q, want s3 timeout", msg.Error)
	}
	if msg.Key == nil || msg.Key.ProjectKey != "p1" || msg.Key.SessionID != "s1" || msg.Key.Subpath != "sub" {
		t.Errorf("Key not populated: %+v", msg.Key)
	}

	// A mirror_error without a key must still parse.
	bare, ok := parseOne(t, `{"type":"system","subtype":"mirror_error","error":"boom"}`).(*shared.MirrorErrorMessage)
	if !ok || bare.Key != nil {
		t.Errorf("keyless mirror_error should parse with a nil Key, got %+v", bare)
	}
}

func TestParseServerToolBlocks(t *testing.T) {
	line := `{"type":"assistant","message":{"model":"m","content":[` +
		`{"type":"server_tool_use","id":"t1","name":"web_search","input":{"q":"x"}},` +
		`{"type":"advisor_tool_result","tool_use_id":"t1","content":{"ok":true}}]}}`
	msg := parseOne(t, line).(*shared.AssistantMessage)
	if len(msg.Content) != 2 {
		t.Fatalf("got %d blocks, want 2 (server tool blocks must not be dropped)", len(msg.Content))
	}
	use, ok := msg.Content[0].(*shared.ServerToolUseBlock)
	if !ok || use.ID != "t1" || use.Name != "web_search" {
		t.Errorf("unexpected first block: %+v", msg.Content[0])
	}
	res, ok := msg.Content[1].(*shared.ServerToolResultBlock)
	if !ok || res.ToolUseID != "t1" {
		t.Errorf("unexpected second block: %+v", msg.Content[1])
	}

	// A block type from a newer CLI is still skipped, not fatal.
	fwd := parseOne(t, `{"type":"assistant","message":{"model":"m","content":[{"type":"future_block"}]}}`).(*shared.AssistantMessage)
	if len(fwd.Content) != 0 {
		t.Errorf("unknown block types should be skipped, got %v", fwd.Content)
	}
}
