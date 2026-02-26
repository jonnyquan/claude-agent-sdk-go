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
				Matcher: stringPtr("Bash"),
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

	if preToolUseConfig[0].Matcher == nil || *preToolUseConfig[0].Matcher != "Bash" {
		t.Errorf("Expected matcher 'Bash', got %v", preToolUseConfig[0].Matcher)
	}
}

func TestHookProcessor_BuildInitializeConfig_NilMatcher(t *testing.T) {
	ctx := context.Background()

	testHook := func(input shared.HookInput, toolUseID *string, ctx shared.HookContext) (shared.HookJSONOutput, error) {
		return shared.NewPreToolUseOutput(shared.PermissionDecisionAllow, "", nil), nil
	}

	options := shared.NewOptions()
	options.Hooks = map[string][]any{
		string(shared.HookEventPreToolUse): {
			shared.HookMatcher{
				Matcher: nil,
				Hooks:   []shared.HookCallback{testHook},
			},
		},
	}

	hp := NewHookProcessor(ctx, options)
	config := hp.BuildInitializeConfig()
	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	preToolUseConfig := config[string(shared.HookEventPreToolUse)]
	if len(preToolUseConfig) != 1 {
		t.Fatalf("Expected 1 matcher config, got %d", len(preToolUseConfig))
	}
	if preToolUseConfig[0].Matcher != nil {
		t.Fatalf("expected nil matcher, got %v", preToolUseConfig[0].Matcher)
	}
}

func TestHookProcessor_BuildInitializeConfig_IncludesMatcherWithoutCallbacks(t *testing.T) {
	ctx := context.Background()

	options := shared.NewOptions()
	options.Hooks = map[string][]any{
		string(shared.HookEventPreToolUse): {
			shared.HookMatcher{
				Matcher: stringPtr("Bash"),
				Hooks:   nil,
			},
		},
	}

	hp := NewHookProcessor(ctx, options)
	config := hp.BuildInitializeConfig()
	if config == nil {
		t.Fatal("expected non-nil config")
	}

	preToolUseConfig, exists := config[string(shared.HookEventPreToolUse)]
	if !exists {
		t.Fatal("expected PreToolUse config")
	}
	if len(preToolUseConfig) != 1 {
		t.Fatalf("expected 1 matcher config, got %d", len(preToolUseConfig))
	}
	if preToolUseConfig[0].Matcher == nil || *preToolUseConfig[0].Matcher != "Bash" {
		t.Fatalf("expected matcher 'Bash', got %v", preToolUseConfig[0].Matcher)
	}
	if len(preToolUseConfig[0].HookCallbackIDs) != 0 {
		t.Fatalf("expected no callback IDs, got %v", preToolUseConfig[0].HookCallbackIDs)
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
		inputMap, ok := input.(map[string]any)
		if !ok {
			t.Fatalf("expected map input, got %T", input)
		}
		if toolName, ok := inputMap["tool_name"].(string); !ok || toolName != "Bash" {
			t.Errorf("Expected tool_name 'Bash', got %v", inputMap["tool_name"])
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

func stringPtr(s string) *string { return &s }

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

func TestHookProcessor_ConvertsPermissionSuggestions(t *testing.T) {
	ctx := context.Background()

	sawSuggestions := false
	permCallback := func(toolName string, toolInput map[string]any, ctx shared.ToolPermissionContext) (shared.PermissionResult, error) {
		if len(ctx.Suggestions) != 1 {
			t.Fatalf("expected 1 suggestion, got %d", len(ctx.Suggestions))
		}

		suggestion := ctx.Suggestions[0]
		if suggestion.Type != shared.PermissionUpdateTypeAddRules {
			t.Fatalf("expected addRules suggestion type, got %s", suggestion.Type)
		}
		if suggestion.Destination == nil || *suggestion.Destination != shared.PermissionDestinationSession {
			t.Fatalf("expected destination session, got %#v", suggestion.Destination)
		}
		if len(suggestion.Rules) != 1 || suggestion.Rules[0].ToolName != "Bash" {
			t.Fatalf("unexpected suggestion rules: %#v", suggestion.Rules)
		}

		sawSuggestions = true
		return shared.NewPermissionAllow(toolInput, nil), nil
	}

	hp := NewHookProcessor(ctx, shared.NewOptions())
	hp.SetCanUseToolCallback(permCallback)

	request := &shared.CanUseToolRequest{
		Subtype:  shared.ControlSubtypeCanUseTool,
		ToolName: "Bash",
		Input:    map[string]any{"command": "echo ok"},
		PermissionSuggestions: []any{
			map[string]any{
				"type":        "addRules",
				"destination": "session",
				"behavior":    "allow",
				"rules": []any{
					map[string]any{
						"toolName":    "Bash",
						"ruleContent": "echo *",
					},
				},
			},
		},
	}

	_, err := hp.ProcessCanUseTool(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sawSuggestions {
		t.Fatal("expected callback to receive converted suggestions")
	}
}

func TestHookProcessor_BuildInitializeConfig_DoesNotCrossBindSameCallbackAcrossMatchers(t *testing.T) {
	ctx := context.Background()

	sharedCallback := func(input shared.HookInput, toolUseID *string, ctx shared.HookContext) (shared.HookJSONOutput, error) {
		return shared.NewPreToolUseOutput(shared.PermissionDecisionAllow, "", nil), nil
	}

	options := shared.NewOptions()
	options.Hooks = map[string][]any{
		string(shared.HookEventPreToolUse): {
			shared.HookMatcher{
				Matcher: stringPtr("Bash"),
				Hooks:   []shared.HookCallback{sharedCallback},
			},
			shared.HookMatcher{
				Matcher: stringPtr("Edit"),
				Hooks:   []shared.HookCallback{sharedCallback},
			},
		},
	}

	hp := NewHookProcessor(ctx, options)
	config := hp.BuildInitializeConfig()
	if config == nil {
		t.Fatal("expected initialize config")
	}

	preToolUse := config[string(shared.HookEventPreToolUse)]
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 matcher configs, got %d", len(preToolUse))
	}
	if len(preToolUse[0].HookCallbackIDs) != 1 {
		t.Fatalf("expected first matcher to have 1 callback id, got %d", len(preToolUse[0].HookCallbackIDs))
	}
	if len(preToolUse[1].HookCallbackIDs) != 1 {
		t.Fatalf("expected second matcher to have 1 callback id, got %d", len(preToolUse[1].HookCallbackIDs))
	}
	if preToolUse[0].HookCallbackIDs[0] == preToolUse[1].HookCallbackIDs[0] {
		t.Fatalf("expected distinct callback IDs per matcher, got shared id %q", preToolUse[0].HookCallbackIDs[0])
	}
}
