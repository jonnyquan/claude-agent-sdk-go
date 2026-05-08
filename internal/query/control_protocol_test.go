package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
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

func TestHandleHookCallbackAcceptsNonObjectInput(t *testing.T) {
	ctx := context.Background()
	hp := NewHookProcessor(ctx, shared.NewOptions())
	callbackID := hp.generateCallbackID()
	hp.hookCallbacks[callbackID] = func(input shared.HookInput, toolUseID *string, hookCtx shared.HookContext) (shared.HookJSONOutput, error) {
		if inputStr, ok := input.(string); !ok || inputStr != "raw-input" {
			t.Fatalf("expected raw string input, got %T (%v)", input, input)
		}
		return shared.HookJSONOutput{"continue": true}, nil
	}

	cp := NewControlProtocol(ctx, hp, func([]byte) error { return nil }, nil)
	resp, err := cp.handleHookCallback(map[string]any{
		"callback_id": callbackID,
		"input":       "raw-input",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if continueVal, ok := resp["continue"].(bool); !ok || !continueVal {
		t.Fatalf("expected continue=true response, got %#v", resp)
	}
}

func TestSendControlRequestReturnsEmptyMapWhenResponseBodyMissing(t *testing.T) {
	ctx := context.Background()
	var cp *ControlProtocol
	writeFn := func(data []byte) error {
		var req shared.ControlRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return err
		}
		resp := shared.ControlResponse{
			Type: shared.ControlTypeResponse,
			Response: shared.ResponsePayload{
				Subtype:   shared.ControlSubtypeSuccess,
				RequestID: req.RequestID,
				// Intentionally no Response payload.
			},
		}
		respBytes, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		return cp.handleControlResponse(respBytes)
	}

	cp = NewControlProtocol(ctx, nil, writeFn, nil)
	got, err := cp.sendControlRequest(
		map[string]any{"subtype": shared.ControlSubtypeMCPStatus},
		time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}
}

// TestHandleControlCancelCancelsInflightRequest verifies that a
// control_cancel_request from the CLI cancels the matching in-flight
// inbound control request's context. Mirrors Python SDK fix #751.
func TestHandleControlCancelCancelsInflightRequest(t *testing.T) {
	cp := NewControlProtocol(context.Background(), nil, func([]byte) error { return nil }, nil)

	// Manually register an in-flight request — emulates handleControlRequest
	// having spawned a long-running handler.
	reqID := "req_42"
	ctx, cancel := context.WithCancel(context.Background())
	cp.inflightMu.Lock()
	cp.inflightRequests[reqID] = cancel
	cp.inflightMu.Unlock()

	// Sanity check: context not cancelled.
	select {
	case <-ctx.Done():
		t.Fatal("context cancelled prematurely")
	default:
	}

	// Send the cancel request.
	cancelMsg, _ := json.Marshal(map[string]any{
		"type":       "control_cancel_request",
		"request_id": reqID,
	})
	if err := cp.handleControlCancel(cancelMsg); err != nil {
		t.Fatalf("handleControlCancel: %v", err)
	}

	// In-flight entry should be removed.
	cp.inflightMu.Lock()
	_, stillRegistered := cp.inflightRequests[reqID]
	cp.inflightMu.Unlock()
	if stillRegistered {
		t.Fatal("expected in-flight entry to be removed after cancel")
	}

	// Context should be cancelled.
	select {
	case <-ctx.Done():
		// expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled within deadline")
	}
}

// TestHandleControlCancelUnknownIDIsNoOp ensures cancelling a stale id is
// silently ignored (matches Python's `pop(cancel_id, None)` behavior).
func TestHandleControlCancelUnknownIDIsNoOp(t *testing.T) {
	cp := NewControlProtocol(context.Background(), nil, func([]byte) error { return nil }, nil)
	cancelMsg, _ := json.Marshal(map[string]any{
		"type":       "control_cancel_request",
		"request_id": "ghost",
	})
	if err := cp.handleControlCancel(cancelMsg); err != nil {
		t.Fatalf("expected nil error for unknown request_id, got %v", err)
	}
}

// TestHandleControlCancelMalformedReturnsError verifies bad JSON surfaces
// as an error rather than panicking.
func TestHandleControlCancelMalformedReturnsError(t *testing.T) {
	cp := NewControlProtocol(context.Background(), nil, func([]byte) error { return nil }, nil)
	if err := cp.handleControlCancel([]byte("{not valid json")); err == nil {
		t.Fatal("expected error from malformed control_cancel_request")
	}
}
