package claudecode

import (
	"context"
	"testing"
)

func TestPermissionTypes(t *testing.T) {
	t.Run("PermissionUpdateType constants", func(t *testing.T) {
		types := []PermissionUpdateType{
			PermissionUpdateTypeAddRules,
			PermissionUpdateTypeReplaceRules,
			PermissionUpdateTypeRemoveRules,
			PermissionUpdateTypeSetMode,
			PermissionUpdateTypeAddDirectories,
			PermissionUpdateTypeRemoveDirectories,
		}
		
		expected := []string{
			"addRules",
			"replaceRules",
			"removeRules",
			"setMode",
			"addDirectories",
			"removeDirectories",
		}
		
		for i, pType := range types {
			if string(pType) != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], string(pType))
			}
		}
	})
	
	t.Run("PermissionDestination constants", func(t *testing.T) {
		destinations := []PermissionDestination{
			PermissionDestinationSession,
			PermissionDestinationSettings,
		}
		
		expected := []string{"session", "settings"}
		
		for i, dest := range destinations {
			if string(dest) != expected[i] {
				t.Errorf("Expected %s, got %s", expected[i], string(dest))
			}
		}
	})
}

func TestPermissionRule(t *testing.T) {
	t.Run("NewPermissionRule", func(t *testing.T) {
		rule := NewPermissionRule("Bash", "allow all")
		
		if rule.ToolName != "Bash" {
			t.Errorf("Expected toolName 'Bash', got %s", rule.ToolName)
		}
		
		if rule.RuleContent != "allow all" {
			t.Errorf("Expected ruleContent 'allow all', got %s", rule.RuleContent)
		}
	})
}

func TestPermissionUpdate(t *testing.T) {
	t.Run("NewPermissionUpdate basic", func(t *testing.T) {
		update := NewPermissionUpdate(PermissionUpdateTypeAddRules)
		
		if update.Type != PermissionUpdateTypeAddRules {
			t.Errorf("Expected type 'addRules', got %s", update.Type)
		}
	})
	
	t.Run("PermissionUpdate with builder methods", func(t *testing.T) {
		dest := PermissionDestinationSession
		behavior := "allow"
		rules := []PermissionRule{
			NewPermissionRule("Bash", "allow echo commands"),
			NewPermissionRule("Write", "allow writes to /tmp"),
		}
		
		update := NewPermissionUpdate(PermissionUpdateTypeAddRules).
			WithDestination(dest).
			WithBehavior(behavior).
			WithRules(rules)
		
		if update.Type != PermissionUpdateTypeAddRules {
			t.Errorf("Expected type 'addRules', got %s", update.Type)
		}
		
		if update.Destination == nil || *update.Destination != dest {
			t.Errorf("Expected destination 'session', got %v", update.Destination)
		}
		
		if update.Behavior == nil || *update.Behavior != behavior {
			t.Errorf("Expected behavior 'allow', got %v", update.Behavior)
		}
		
		if len(update.Rules) != 2 {
			t.Errorf("Expected 2 rules, got %d", len(update.Rules))
		}
	})
	
	t.Run("PermissionUpdate with mode", func(t *testing.T) {
		mode := "acceptEdits"
		update := NewPermissionUpdate(PermissionUpdateTypeSetMode).
			WithMode(mode)
		
		if update.Mode == nil || *update.Mode != mode {
			t.Errorf("Expected mode 'acceptEdits', got %v", update.Mode)
		}
	})
	
	t.Run("PermissionUpdate with directories", func(t *testing.T) {
		dirs := []string{"/path/to/dir1", "/path/to/dir2"}
		update := NewPermissionUpdate(PermissionUpdateTypeAddDirectories).
			WithDirectories(dirs)
		
		if len(update.Directories) != 2 {
			t.Errorf("Expected 2 directories, got %d", len(update.Directories))
		}
		
		if update.Directories[0] != "/path/to/dir1" {
			t.Errorf("Expected first directory '/path/to/dir1', got %s", update.Directories[0])
		}
	})
}

func TestPermissionResults(t *testing.T) {
	t.Run("NewPermissionAllow", func(t *testing.T) {
		updatedInput := map[string]any{
			"command": "echo 'safe command'",
		}
		
		updates := []PermissionUpdate{
			*NewPermissionUpdate(PermissionUpdateTypeAddRules).
				WithRules([]PermissionRule{
					NewPermissionRule("Bash", "allow echo"),
				}),
		}
		
		result := NewPermissionAllow(updatedInput, updates)
		
		if result.Behavior != "allow" {
			t.Errorf("Expected behavior 'allow', got %s", result.Behavior)
		}
		
		if result.UpdatedInput == nil {
			t.Error("UpdatedInput should not be nil")
		}
		
		if len(result.UpdatedPermissions) != 1 {
			t.Errorf("Expected 1 permission update, got %d", len(result.UpdatedPermissions))
		}
	})
	
	t.Run("NewPermissionDeny", func(t *testing.T) {
		result := NewPermissionDeny("Command is too dangerous", true)
		
		if result.Behavior != "deny" {
			t.Errorf("Expected behavior 'deny', got %s", result.Behavior)
		}
		
		if result.Message != "Command is too dangerous" {
			t.Errorf("Expected message 'Command is too dangerous', got %s", result.Message)
		}
		
		if !result.Interrupt {
			t.Error("Expected interrupt to be true")
		}
	})
	
	t.Run("PermissionResult interface", func(t *testing.T) {
		var result PermissionResult
		
		result = NewPermissionAllow(nil, nil)
		if result == nil {
			t.Error("NewPermissionAllow should return non-nil PermissionResult")
		}
		
		result = NewPermissionDeny("error", false)
		if result == nil {
			t.Error("NewPermissionDeny should return non-nil PermissionResult")
		}
	})
}

func TestToolPermissionContext(t *testing.T) {
	t.Run("ToolPermissionContext creation", func(t *testing.T) {
		suggestions := []PermissionUpdate{
			*NewPermissionUpdate(PermissionUpdateTypeAddRules).
				WithRules([]PermissionRule{
					NewPermissionRule("Bash", "suggested rule"),
				}),
		}
		
		ctx := ToolPermissionContext{
			Context:     context.Background(),
			Suggestions: suggestions,
		}
		
		if ctx.Context == nil {
			t.Error("Context should not be nil")
		}
		
		if len(ctx.Suggestions) != 1 {
			t.Errorf("Expected 1 suggestion, got %d", len(ctx.Suggestions))
		}
	})
}

func TestCanUseToolCallback(t *testing.T) {
	t.Run("CanUseToolCallback signature", func(t *testing.T) {
		callback := func(toolName string, toolInput map[string]any, ctx ToolPermissionContext) (PermissionResult, error) {
			if toolName == "Bash" {
				command, _ := toolInput["command"].(string)
				if command == "rm -rf /" {
					return NewPermissionDeny("Dangerous command blocked", false), nil
				}
				return NewPermissionAllow(nil, nil), nil
			}
			return NewPermissionAllow(nil, nil), nil
		}
		
		// Test allow case
		ctx := ToolPermissionContext{
			Context: context.Background(),
		}
		
		result, err := callback("Bash", map[string]any{"command": "echo hello"}, ctx)
		if err != nil {
			t.Fatalf("Callback should not return error: %v", err)
		}
		
		allowResult, ok := result.(*PermissionResultAllow)
		if !ok {
			t.Fatal("Result should be PermissionResultAllow")
		}
		
		if allowResult.Behavior != "allow" {
			t.Errorf("Expected behavior 'allow', got %s", allowResult.Behavior)
		}
		
		// Test deny case
		result, err = callback("Bash", map[string]any{"command": "rm -rf /"}, ctx)
		if err != nil {
			t.Fatalf("Callback should not return error: %v", err)
		}
		
		denyResult, ok := result.(*PermissionResultDeny)
		if !ok {
			t.Fatal("Result should be PermissionResultDeny")
		}
		
		if denyResult.Behavior != "deny" {
			t.Errorf("Expected behavior 'deny', got %s", denyResult.Behavior)
		}
		
		if denyResult.Message != "Dangerous command blocked" {
			t.Errorf("Expected message 'Dangerous command blocked', got %s", denyResult.Message)
		}
	})
}
