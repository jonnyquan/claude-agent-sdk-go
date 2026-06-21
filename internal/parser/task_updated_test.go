package parser

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// These tests mirror the Python SDK's task_updated message_parser tests to keep
// the two SDKs at 1:1 behavioral parity.

func TestParseTaskUpdatedMessageTerminal(t *testing.T) {
	p := setupParserTest(t)
	data := map[string]any{
		"type":       "system",
		"subtype":    "task_updated",
		"task_id":    "task-abc",
		"patch":      map[string]any{"status": "completed", "end_time": float64(1780405729183)},
		"uuid":       "uuid-4",
		"session_id": "session-1",
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu, ok := msg.(*shared.TaskUpdatedMessage)
	if !ok {
		t.Fatalf("expected *TaskUpdatedMessage, got %T", msg)
	}
	if tu.TaskID != "task-abc" {
		t.Errorf("task_id = %q, want task-abc", tu.TaskID)
	}
	if tu.Status != shared.TaskUpdatedStatusCompleted {
		t.Errorf("status = %q, want completed", tu.Status)
	}
	if tu.UUID == nil || *tu.UUID != "uuid-4" {
		t.Errorf("uuid = %v, want uuid-4", tu.UUID)
	}
	if tu.SessionID == nil || *tu.SessionID != "session-1" {
		t.Errorf("session_id = %v, want session-1", tu.SessionID)
	}
	if tu.Patch["status"] != "completed" {
		t.Errorf("patch[status] = %v, want completed", tu.Patch["status"])
	}
	if !shared.IsTerminalTaskStatus(string(tu.Status)) {
		t.Errorf("status %q should be terminal", tu.Status)
	}
}

func TestParseTaskUpdatedMessageMinimal(t *testing.T) {
	// Only task_id and patch (no uuid/session_id) — parsing must never fail.
	p := setupParserTest(t)
	data := map[string]any{
		"type":    "system",
		"subtype": "task_updated",
		"task_id": "b1m21w89v",
		"patch":   map[string]any{"status": "completed", "end_time": float64(1780405729183)},
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu := msg.(*shared.TaskUpdatedMessage)
	if tu.TaskID != "b1m21w89v" {
		t.Errorf("task_id = %q", tu.TaskID)
	}
	if tu.Status != shared.TaskUpdatedStatusCompleted {
		t.Errorf("status = %q, want completed", tu.Status)
	}
	if tu.UUID != nil {
		t.Errorf("uuid = %v, want nil", tu.UUID)
	}
	if tu.SessionID != nil {
		t.Errorf("session_id = %v, want nil", tu.SessionID)
	}
}

func TestParseTaskUpdatedMessageNonTerminalStatuses(t *testing.T) {
	for _, status := range []string{"pending", "running", "paused"} {
		t.Run(status, func(t *testing.T) {
			p := setupParserTest(t)
			data := map[string]any{
				"type":    "system",
				"subtype": "task_updated",
				"task_id": "task-abc",
				"patch":   map[string]any{"status": status},
			}
			msg, err := p.ParseMessage(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tu := msg.(*shared.TaskUpdatedMessage)
			if string(tu.Status) != status {
				t.Errorf("status = %q, want %q", tu.Status, status)
			}
			if shared.IsTerminalTaskStatus(status) {
				t.Errorf("status %q should not be terminal", status)
			}
		})
	}
}

func TestParseTaskUpdatedMessageNoPatch(t *testing.T) {
	// No patch parses with an empty patch and empty status.
	p := setupParserTest(t)
	data := map[string]any{
		"type":    "system",
		"subtype": "task_updated",
		"task_id": "task-abc",
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu := msg.(*shared.TaskUpdatedMessage)
	if len(tu.Patch) != 0 {
		t.Errorf("patch = %v, want empty", tu.Patch)
	}
	if tu.Status != "" {
		t.Errorf("status = %q, want empty", tu.Status)
	}
}

func TestParseTaskUpdatedMessagePatchWithoutStatus(t *testing.T) {
	// A patch lacking "status" is preserved verbatim; status is empty.
	p := setupParserTest(t)
	data := map[string]any{
		"type":    "system",
		"subtype": "task_updated",
		"task_id": "task-abc",
		"patch":   map[string]any{"end_time": float64(1780405729183)},
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu := msg.(*shared.TaskUpdatedMessage)
	if tu.Patch["end_time"] != float64(1780405729183) {
		t.Errorf("patch not preserved: %v", tu.Patch)
	}
	if tu.Status != "" {
		t.Errorf("status = %q, want empty", tu.Status)
	}
}

func TestParseTaskUpdatedMessageNonDictPatch(t *testing.T) {
	// A non-dict (or missing) patch never raises; patch falls back to {}.
	for _, patch := range []any{"completed", []any{"completed"}, float64(42), nil} {
		p := setupParserTest(t)
		data := map[string]any{
			"type":    "system",
			"subtype": "task_updated",
			"task_id": "task-abc",
			"patch":   patch,
		}
		msg, err := p.ParseMessage(data)
		if err != nil {
			t.Fatalf("patch=%v: unexpected error: %v", patch, err)
		}
		tu := msg.(*shared.TaskUpdatedMessage)
		if len(tu.Patch) != 0 {
			t.Errorf("patch=%v: got %v, want empty", patch, tu.Patch)
		}
		if tu.Status != "" {
			t.Errorf("patch=%v: status = %q, want empty", patch, tu.Status)
		}
	}
}

func TestParseTaskUpdatedMessageTerminalStatuses(t *testing.T) {
	// task_updated reports the raw "killed" (not the "stopped" form the CLI maps
	// to on task_notification).
	for _, status := range []string{"completed", "failed", "killed"} {
		t.Run(status, func(t *testing.T) {
			p := setupParserTest(t)
			data := map[string]any{
				"type":    "system",
				"subtype": "task_updated",
				"task_id": "task-abc",
				"patch":   map[string]any{"status": status},
			}
			msg, err := p.ParseMessage(data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tu := msg.(*shared.TaskUpdatedMessage)
			if string(tu.Status) != status {
				t.Errorf("status = %q, want %q", tu.Status, status)
			}
			if !shared.IsTerminalTaskStatus(status) {
				t.Errorf("status %q should be terminal", status)
			}
		})
	}
}

func TestParseTaskUpdatedKilledIsTerminal(t *testing.T) {
	// A task stopped via TaskStop reports status="killed" and is terminal; in
	// some kill paths no task_notification is emitted, so this is the only
	// terminal signal.
	p := setupParserTest(t)
	data := map[string]any{
		"type":    "system",
		"subtype": "task_updated",
		"task_id": "bs2r8eew4",
		"patch":   map[string]any{"status": "killed", "end_time": float64(1780405729183)},
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu := msg.(*shared.TaskUpdatedMessage)
	if tu.Status != shared.TaskUpdatedStatusKilled {
		t.Errorf("status = %q, want killed", tu.Status)
	}
	if !shared.IsTerminalTaskStatus(string(tu.Status)) {
		t.Errorf("killed should be terminal")
	}
}

func TestTaskUpdatedBackwardCompatSystemMessage(t *testing.T) {
	// Backward-compat: TaskUpdatedMessage still carries the base SystemMessage
	// fields and reports the system message type.
	p := setupParserTest(t)
	data := map[string]any{
		"type":       "system",
		"subtype":    "task_updated",
		"task_id":    "t1",
		"patch":      map[string]any{"status": "failed"},
		"uuid":       "u1",
		"session_id": "s1",
	}
	msg, err := p.ParseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tu := msg.(*shared.TaskUpdatedMessage)
	if tu.Subtype != "task_updated" {
		t.Errorf("subtype = %q, want task_updated", tu.Subtype)
	}
	if tu.Type() != shared.MessageTypeSystem {
		t.Errorf("Type() = %q, want %q", tu.Type(), shared.MessageTypeSystem)
	}
}
