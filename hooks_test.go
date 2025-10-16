package claudecode

import (
	"context"
	"testing"
)

func TestHookTypes(t *testing.T) {
	t.Run("HookEvent constants", func(t *testing.T) {
		events := []HookEvent{
			HookEventPreToolUse,
			HookEventPostToolUse,
			HookEventUserPromptSubmit,
			HookEventStop,
			HookEventSubagentStop,
			HookEventPreCompact,
		}
		
		for _, event := range events {
			if event == "" {
				t.Errorf("HookEvent should not be empty")
			}
		}
	})
	
	t.Run("Permission decision constants", func(t *testing.T) {
		decisions := []string{
			PermissionDecisionAllow,
			PermissionDecisionDeny,
			PermissionDecisionAsk,
		}
		
		expected := []string{"allow", "deny", "ask"}
		
		for i, decision := range decisions {
			if decision != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], decision)
			}
		}
	})
}

func TestHookInputTypes(t *testing.T) {
	t.Run("PreToolUseHookInput", func(t *testing.T) {
		input := PreToolUseHookInput{
			BaseHookInput: BaseHookInput{
				SessionID:      "test-session",
				TranscriptPath: "/path/to/transcript",
				Cwd:            "/working/dir",
			},
			HookEventName: "PreToolUse",
			ToolName:      "Bash",
			ToolInput: map[string]any{
				"command": "echo hello",
			},
		}
		
		if input.SessionID != "test-session" {
			t.Errorf("Expected session_id 'test-session', got %s", input.SessionID)
		}
		
		if input.ToolName != "Bash" {
			t.Errorf("Expected tool_name 'Bash', got %s", input.ToolName)
		}
	})
	
	t.Run("PostToolUseHookInput", func(t *testing.T) {
		input := PostToolUseHookInput{
			BaseHookInput: BaseHookInput{
				SessionID: "test-session",
			},
			HookEventName: "PostToolUse",
			ToolName:      "Write",
			ToolInput: map[string]any{
				"file_path": "test.txt",
			},
			ToolResponse: "Success",
		}
		
		if input.ToolResponse != "Success" {
			t.Errorf("Expected tool_response 'Success', got %v", input.ToolResponse)
		}
	})
}

func TestHookOutputHelpers(t *testing.T) {
	t.Run("NewPreToolUseOutput", func(t *testing.T) {
		output := NewPreToolUseOutput(PermissionDecisionAllow, "Test reason", nil)
		
		if output == nil {
			t.Fatal("Output should not be nil")
		}
		
		hookSpecific, ok := output["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatal("hookSpecificOutput should be a map")
		}
		
		decision, _ := hookSpecific["permissionDecision"].(string)
		if decision != PermissionDecisionAllow {
			t.Errorf("Expected decision 'allow', got %s", decision)
		}
		
		reason, _ := output["reason"].(string)
		if reason != "Test reason" {
			t.Errorf("Expected reason 'Test reason', got %s", reason)
		}
	})
	
	t.Run("NewPostToolUseOutput", func(t *testing.T) {
		output := NewPostToolUseOutput("Additional context here")
		
		if output == nil {
			t.Fatal("Output should not be nil")
		}
		
		hookSpecific, ok := output["hookSpecificOutput"].(map[string]any)
		if !ok {
			t.Fatal("hookSpecificOutput should be present")
		}
		
		context, _ := hookSpecific["additionalContext"].(string)
		if context != "Additional context here" {
			t.Errorf("Expected context 'Additional context here', got %s", context)
		}
	})
	
	t.Run("NewBlockingOutput", func(t *testing.T) {
		output := NewBlockingOutput("System message", "Block reason")
		
		decision, _ := output["decision"].(string)
		if decision != "block" {
			t.Errorf("Expected decision 'block', got %s", decision)
		}
		
		systemMsg, _ := output["systemMessage"].(string)
		if systemMsg != "System message" {
			t.Errorf("Expected systemMessage 'System message', got %s", systemMsg)
		}
		
		reason, _ := output["reason"].(string)
		if reason != "Block reason" {
			t.Errorf("Expected reason 'Block reason', got %s", reason)
		}
	})
	
	t.Run("NewStopOutput", func(t *testing.T) {
		output := NewStopOutput("Critical error detected")
		
		continuePtr, ok := output["continue"].(*bool)
		if !ok || continuePtr == nil {
			t.Fatal("continue should be a bool pointer")
		}
		
		if *continuePtr != false {
			t.Errorf("Expected continue to be false, got %v", *continuePtr)
		}
		
		stopReason, _ := output["stopReason"].(string)
		if stopReason != "Critical error detected" {
			t.Errorf("Expected stopReason 'Critical error detected', got %s", stopReason)
		}
	})
	
	t.Run("NewAsyncOutput", func(t *testing.T) {
		timeout := 5000
		output := NewAsyncOutput(&timeout)
		
		async, _ := output["async"].(bool)
		if !async {
			t.Error("Expected async to be true")
		}
		
		asyncTimeout, _ := output["asyncTimeout"].(int)
		if asyncTimeout != 5000 {
			t.Errorf("Expected asyncTimeout 5000, got %d", asyncTimeout)
		}
	})
}

func TestHookCallback(t *testing.T) {
	t.Run("HookCallback signature", func(t *testing.T) {
		// Test that we can define a hook callback
		callback := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
			toolName, _ := input["tool_name"].(string)
			if toolName == "Bash" {
				return NewPreToolUseOutput(PermissionDecisionAllow, "Approved", nil), nil
			}
			return make(HookJSONOutput), nil
		}
		
		// Simulate calling the callback
		input := HookInput{
			"tool_name": "Bash",
			"tool_input": map[string]any{
				"command": "echo test",
			},
		}
		
		ctx := HookContext{
			Context: context.Background(),
		}
		
		output, err := callback(input, nil, ctx)
		if err != nil {
			t.Fatalf("Callback should not return error: %v", err)
		}
		
		if output == nil {
			t.Fatal("Output should not be nil")
		}
	})
}

func TestHookMatcher(t *testing.T) {
	t.Run("HookMatcher creation", func(t *testing.T) {
		callback := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
			return make(HookJSONOutput), nil
		}
		
		matcher := HookMatcher{
			Matcher: "Bash",
			Hooks:   []HookCallback{callback},
		}
		
		if matcher.Matcher != "Bash" {
			t.Errorf("Expected matcher 'Bash', got %s", matcher.Matcher)
		}
		
		if len(matcher.Hooks) != 1 {
			t.Errorf("Expected 1 hook, got %d", len(matcher.Hooks))
		}
	})
}

func TestWithHooksOption(t *testing.T) {
	t.Run("WithHook adds hook to options", func(t *testing.T) {
		callback := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
			return make(HookJSONOutput), nil
		}
		
		matcher := HookMatcher{
			Matcher: "Bash",
			Hooks:   []HookCallback{callback},
		}
		
		opts := NewOptions(
			WithHook(HookEventPreToolUse, matcher),
		)
		
		if opts.Hooks == nil {
			t.Fatal("Hooks should be initialized")
		}
		
		preToolUseHooks, ok := opts.Hooks[string(HookEventPreToolUse)]
		if !ok {
			t.Fatal("PreToolUse hooks should be present")
		}
		
		if len(preToolUseHooks) != 1 {
			t.Errorf("Expected 1 hook, got %d", len(preToolUseHooks))
		}
	})
	
	t.Run("WithHooks adds multiple hooks", func(t *testing.T) {
		callback1 := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
			return make(HookJSONOutput), nil
		}
		
		callback2 := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
			return make(HookJSONOutput), nil
		}
		
		hooks := map[string][]HookMatcher{
			string(HookEventPreToolUse): {
				{Matcher: "Bash", Hooks: []HookCallback{callback1}},
			},
			string(HookEventPostToolUse): {
				{Matcher: "Write", Hooks: []HookCallback{callback2}},
			},
		}
		
		opts := NewOptions(WithHooks(hooks))
		
		if len(opts.Hooks) != 2 {
			t.Errorf("Expected 2 hook types, got %d", len(opts.Hooks))
		}
	})
}
