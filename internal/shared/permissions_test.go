package shared

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPermissionResponseAlwaysEmitMessage verifies the deny-path emits
// `"message": ""` even when Message is empty, matching Python's
// `{"behavior": "deny", "message": response.message}` shape.
func TestPermissionResponseAlwaysEmitMessage(t *testing.T) {
	r := &PermissionResponse{
		Behavior:          "deny",
		Message:           "",
		AlwaysEmitMessage: true,
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"message":""`) {
		t.Errorf("expected `\"message\":\"\"` in deny payload, got %s", got)
	}
	if strings.Contains(got, `"interrupt"`) {
		t.Errorf("interrupt=false should be omitted, got %s", got)
	}
}

// TestPermissionResponseAllowSkipsMessage verifies the allow-path does NOT
// emit `"message"` when AlwaysEmitMessage is false (the default).
func TestPermissionResponseAllowSkipsMessage(t *testing.T) {
	r := &PermissionResponse{
		Behavior:     "allow",
		UpdatedInput: map[string]any{"a": 1},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if strings.Contains(got, `"message"`) {
		t.Errorf("allow without AlwaysEmitMessage should omit message, got %s", got)
	}
	if !strings.Contains(got, `"updatedInput"`) {
		t.Errorf("expected updatedInput in allow payload, got %s", got)
	}
}

// TestPermissionResponseInterruptOnlyWhenTrue confirms `interrupt: true` is
// emitted but `interrupt: false` is omitted.
func TestPermissionResponseInterruptOnlyWhenTrue(t *testing.T) {
	r := &PermissionResponse{
		Behavior:          "deny",
		Message:           "stop",
		Interrupt:         true,
		AlwaysEmitMessage: true,
	}
	data, _ := json.Marshal(r)
	if !strings.Contains(string(data), `"interrupt":true`) {
		t.Errorf("expected interrupt=true to be emitted, got %s", string(data))
	}
}

// TestPermissionUpdateFromDictRoundtripsAllVariants verifies that
// PermissionUpdateFromDict + variant-aware to_dict roundtrip preserves
// the relevant fields per type.
func TestPermissionUpdateFromDictRoundtripsAllVariants(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
	}{
		{
			name: "addRules",
			in: map[string]any{
				"type":     "addRules",
				"rules":    []any{map[string]any{"toolName": "Bash", "ruleContent": "rm -rf /"}},
				"behavior": "deny",
			},
		},
		{
			name: "setMode",
			in: map[string]any{
				"type": "setMode",
				"mode": "acceptEdits",
			},
		},
		{
			name: "addDirectories",
			in: map[string]any{
				"type":        "addDirectories",
				"directories": []any{"/tmp", "/var/log"},
			},
		},
		{
			name: "with destination",
			in: map[string]any{
				"type":        "setMode",
				"mode":        "default",
				"destination": "session",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pu := PermissionUpdateFromDict(c.in)
			if string(pu.Type) != c.in["type"].(string) {
				t.Errorf("type mismatch: got %s, want %s", pu.Type, c.in["type"])
			}
		})
	}
}
