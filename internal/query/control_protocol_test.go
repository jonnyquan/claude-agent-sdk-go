package query

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInitializeTimeoutDefaultsToSixtySeconds(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "")
	if got := initializeTimeout(); got != 60*time.Second {
		t.Fatalf("expected 60s default timeout, got %v", got)
	}
}

func TestInitializeTimeoutHonorsLargerEnvValue(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "120000")
	if got := initializeTimeout(); got != 120*time.Second {
		t.Fatalf("expected 120s timeout, got %v", got)
	}
}

func TestInitializeTimeoutClampsToMinimum(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "1000")
	if got := initializeTimeout(); got != 60*time.Second {
		t.Fatalf("expected clamped 60s timeout, got %v", got)
	}
}

func TestHandleCanUseToolWithoutHookProcessor(t *testing.T) {
	cp := NewControlProtocol(context.Background(), nil, func([]byte) error { return nil }, nil)

	_, err := cp.handleCanUseTool(map[string]any{
		"tool_name": "Bash",
		"input": map[string]any{
			"command": "echo hi",
		},
	})
	if err == nil {
		t.Fatal("expected error when can_use_tool callback is not configured")
	}
	if !strings.Contains(err.Error(), "canUseTool callback is not provided") {
		t.Fatalf("expected canUseTool callback error, got %v", err)
	}
}

func TestHandleHookCallbackWithoutHookProcessor(t *testing.T) {
	cp := NewControlProtocol(context.Background(), nil, func([]byte) error { return nil }, nil)

	_, err := cp.handleHookCallback(map[string]any{
		"callback_id": "hook_0",
		"input":       map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error when hook processor is not configured")
	}
	if !strings.Contains(err.Error(), "no hook callback found for ID: hook_0") {
		t.Fatalf("expected missing hook callback error, got %v", err)
	}
}
