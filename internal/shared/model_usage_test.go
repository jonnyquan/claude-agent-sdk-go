package shared

import "testing"

func TestParseModelUsage(t *testing.T) {
	raw := map[string]any{
		"claude-opus-4-7": map[string]any{
			"inputTokens":              float64(100),
			"outputTokens":             float64(200),
			"cacheReadInputTokens":     float64(10),
			"cacheCreationInputTokens": float64(5),
			"webSearchRequests":        float64(2),
			"costUSD":                  1.25,
			"contextWindow":            float64(200000),
			"maxOutputTokens":          float64(64000),
			"canonicalModel":           "claude-opus-4-7",
			"provider":                 "firstParty",
			// A field the SDK does not know yet must stay reachable.
			"futureField": "kept",
		},
		// A non-object entry is skipped rather than failing the whole result.
		"broken": "not-an-object",
	}

	usage := ParseModelUsage(raw)
	if len(usage) != 1 {
		t.Fatalf("len(usage) = %d, want 1 (malformed entry skipped)", len(usage))
	}
	entry := usage["claude-opus-4-7"]
	if entry.InputTokens != 100 || entry.OutputTokens != 200 {
		t.Errorf("token counts = %d/%d, want 100/200", entry.InputTokens, entry.OutputTokens)
	}
	if entry.CostUSD != 1.25 {
		t.Errorf("CostUSD = %v, want 1.25", entry.CostUSD)
	}
	if entry.CanonicalModel != "claude-opus-4-7" || entry.Provider != "firstParty" {
		t.Errorf("canonicalModel/provider = %q/%q", entry.CanonicalModel, entry.Provider)
	}
	if entry.Raw["futureField"] != "kept" {
		t.Errorf("Raw did not preserve unknown fields: %v", entry.Raw)
	}

	// Optional fields are genuinely optional.
	partial := ParseModelUsage(map[string]any{"m": map[string]any{"inputTokens": float64(1)}})
	if got := partial["m"]; got.CanonicalModel != "" || got.Provider != "" || got.OutputTokens != 0 {
		t.Errorf("expected zero values for absent fields, got %+v", got)
	}

	if ParseModelUsage("not-an-object") != nil {
		t.Error("non-object modelUsage must yield nil")
	}
	if ParseModelUsage(nil) != nil {
		t.Error("absent modelUsage must yield nil")
	}
}

func TestIsAbortedTerminalReason(t *testing.T) {
	for _, reason := range []string{TerminalReasonAbortedStreaming, TerminalReasonAbortedTools} {
		if !IsAbortedTerminalReason(reason) {
			t.Errorf("IsAbortedTerminalReason(%q) = false, want true", reason)
		}
	}
	for _, reason := range []string{TerminalReasonCompleted, TerminalReasonMaxTurns, "", "unknown"} {
		if IsAbortedTerminalReason(reason) {
			t.Errorf("IsAbortedTerminalReason(%q) = true, want false", reason)
		}
	}
}
