package query

import (
	"context"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

func TestHookProcessor_BuildInitializeConfig(t *testing.T) {
	ctx := context.Background()
	
	// Create a test hook callback
	testHook := func(input shared.HookInput, toolUseID *string, ctx shared.HookContext) (shared.HookJSONOutput, error) {
		return shared.NewPreToolUseOutput(shared.PermissionDecisionAllow, "", nil), nil
	}
	
	// Create options with hooks
	options := shared.NewOptions()
	options.Hooks = map[string][]any{
		string(shared.HookEventPreToolUse): {
			shared.HookMatcher{
				Matcher: "Bash",
				Hooks:   []shared.HookCallback{testHook},
			},
		},
	}
	
	hp := NewHookProcessor(ctx, options)
	
	config := hp.BuildInitializeConfig()
	
	if config == nil {
		t.Fatal("Expected non-nil config")
	}
	
	preToolUseConfig, exists := config[string(shared.HookEventPreToolUse)]
	if !exists {
		t.Fatal("Expected PreToolUse config")
	}
	
	if len(preToolUseConfig) != 1 {
		t.Fatalf("Expected 1 matcher config, got %d", len(preToolUseConfig))
	}
	
	if preToolUseConfig[0].Matcher != "Bash" {
		t.Errorf("Expected matcher 'Bash', got %s", preToolUseConfig[0].Matcher)
	}
}

func TestHookProcessor_ProcessHookCallback(t *testing.T) {
	ctx := context.Background()
	
	called := false
	expectedInput := map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "echo test"},
	}
	
	// Create a test hook that sets the called flag
	testHook := func(input shared.HookInput, toolUseID *string, ctx shared.HookContext) (shared.HookJSONOutput, error) {
		called = true
		
		// Verify input
		if toolName, ok := input["tool_name"].(string); !ok || toolName != "Bash" {
			t.Errorf("Expected tool_name 'Bash', got %v", input["tool_name"])
		}
		
		return shared.NewPreToolUseOutput(shared.PermissionDecisionAllow, "Test approved", nil), nil
	}
	
	hp := NewHookProcessor(ctx, shared.NewOptions())
	
	// Manually register callback
	callbackID := hp.generateCallbackID()
	hp.hookCallbacks[callbackID] = testHook
	
	// Create hook callback request
	request := &shared.HookCallbackRequest{
		Subtype:    shared.ControlSubtypeHookCallback,
		CallbackID: callbackID,
		Input:      expectedInput,
		ToolUseID:  nil,
	}
	
	// Process callback
	output, err := hp.ProcessHookCallback(request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if !called {
		t.Error("Hook callback was not called")
	}
	
	if output == nil {
		t.Fatal("Expected non-nil output")
	}
	
	// Check hook specific output
	hookSpecific, ok := output["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("Expected hookSpecificOutput in output")
	}
	
	if decision, ok := hookSpecific["permissionDecision"].(string); !ok || decision != shared.PermissionDecisionAllow {
		t.Errorf("Expected permissionDecision 'allow', got %v", hookSpecific["permissionDecision"])
	}
}

func TestHookProcessor_ProcessCanUseTool(t *testing.T) {
	ctx := context.Background()
	
	called := false
	
	// Create permission callback
	permCallback := func(toolName string, toolInput map[string]any, ctx shared.ToolPermissionContext) (shared.PermissionResult, error) {
		called = true
		
		if toolName != "Bash" {
			t.Errorf("Expected toolName 'Bash', got %s", toolName)
		}
		
		if command, ok := toolInput["command"].(string); !ok || command != "rm -rf /" {
			t.Errorf("Expected command 'rm -rf /', got %v", toolInput["command"])
		}
		
		// Deny dangerous command
		return shared.NewPermissionDeny("Dangerous command", false), nil
	}
	
	hp := NewHookProcessor(ctx, shared.NewOptions())
	hp.SetCanUseToolCallback(permCallback)
	
	// Create can use tool request
	request := &shared.CanUseToolRequest{
		Subtype:  shared.ControlSubtypeCanUseTool,
		ToolName: "Bash",
		Input: map[string]any{
			"command": "rm -rf /",
		},
	}
	
	// Process request
	response, err := hp.ProcessCanUseTool(request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if !called {
		t.Error("Permission callback was not called")
	}
	
	if response.Behavior != "deny" {
		t.Errorf("Expected behavior 'deny', got %s", response.Behavior)
	}
	
	if response.Message != "Dangerous command" {
		t.Errorf("Expected message 'Dangerous command', got %s", response.Message)
	}
}

func TestHookProcessor_ProcessCanUseTool_Allow(t *testing.T) {
	ctx := context.Background()
	
	permCallback := func(toolName string, toolInput map[string]any, ctx shared.ToolPermissionContext) (shared.PermissionResult, error) {
		// Allow safe command
		return shared.NewPermissionAllow(toolInput, nil), nil
	}
	
	hp := NewHookProcessor(ctx, shared.NewOptions())
	hp.SetCanUseToolCallback(permCallback)
	
	request := &shared.CanUseToolRequest{
		Subtype:  shared.ControlSubtypeCanUseTool,
		ToolName: "Bash",
		Input: map[string]any{
			"command": "echo hello",
		},
	}
	
	response, err := hp.ProcessCanUseTool(request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if response.Behavior != "allow" {
		t.Errorf("Expected behavior 'allow', got %s", response.Behavior)
	}
	
	// Should return updated input (or original if nil)
	if response.UpdatedInput == nil {
		t.Error("Expected updatedInput in response")
	}
}

func TestHookProcessor_NoCallback(t *testing.T) {
	ctx := context.Background()
	hp := NewHookProcessor(ctx, shared.NewOptions())
	
	request := &shared.HookCallbackRequest{
		Subtype:    shared.ControlSubtypeHookCallback,
		CallbackID: "nonexistent",
		Input:      map[string]any{},
	}
	
	_, err := hp.ProcessHookCallback(request)
	if err == nil {
		t.Error("Expected error for nonexistent callback")
	}
}

func TestHookProcessor_NoPermissionCallback(t *testing.T) {
	ctx := context.Background()
	hp := NewHookProcessor(ctx, shared.NewOptions())
	
	request := &shared.CanUseToolRequest{
		Subtype:  shared.ControlSubtypeCanUseTool,
		ToolName: "Bash",
		Input:    map[string]any{},
	}
	
	_, err := hp.ProcessCanUseTool(request)
	if err == nil {
		t.Error("Expected error when no permission callback is set")
	}
}
