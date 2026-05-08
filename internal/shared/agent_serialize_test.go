package shared

import (
	"reflect"
	"testing"
)

func TestSerializeAgentDefinition_IncludesAllNonNilFields(t *testing.T) {
	model := "claude-sonnet-4-7"
	memory := "project"
	initial := "Run setup"
	maxTurns := 25
	background := true
	pmode := PermissionModeAcceptEdits

	def := AgentDefinition{
		Description:     "test agent",
		Prompt:          "be helpful",
		Tools:           []string{"Read", "Write"},
		DisallowedTools: []string{"Bash"},
		Model:           &model,
		Skills:          []string{"pdf"},
		Memory:          &memory,
		McpServers:      []any{"server-a"},
		InitialPrompt:   &initial,
		MaxTurns:        &maxTurns,
		Background:      &background,
		Effort:          "high",
		PermissionMode:  &pmode,
	}

	got := SerializeAgentDefinition(def)
	want := map[string]any{
		"description":     "test agent",
		"prompt":          "be helpful",
		"tools":           []string{"Read", "Write"},
		"disallowedTools": []string{"Bash"},
		"model":           "claude-sonnet-4-7",
		"skills":          []string{"pdf"},
		"memory":          "project",
		"mcpServers":      []any{"server-a"},
		"initialPrompt":   "Run setup",
		"maxTurns":        25,
		"background":      true,
		"effort":          "high",
		"permissionMode":  string(PermissionModeAcceptEdits),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("serialized agent mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestSerializeAgentDefinition_SkipsAllNilFields(t *testing.T) {
	def := AgentDefinition{
		Description: "minimal",
		Prompt:      "be brief",
	}
	got := SerializeAgentDefinition(def)
	want := map[string]any{
		"description": "minimal",
		"prompt":      "be brief",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected only description+prompt, got %#v", got)
	}
}

func TestSerializeAgentDefinitions_NilOnEmpty(t *testing.T) {
	if got := SerializeAgentDefinitions(nil); got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
	if got := SerializeAgentDefinitions(map[string]AgentDefinition{}); got != nil {
		t.Fatalf("expected nil for empty map, got %#v", got)
	}
}
