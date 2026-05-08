package sessions

import "testing"

// TestExtractPrompt_StringContentSimple verifies the basic happy path.
func TestExtractPrompt_StringContentSimple(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Hello, world",
		},
	}
	got, fallback := extractPrompt(entry)
	if got != "Hello, world" {
		t.Errorf("expected 'Hello, world', got %q", got)
	}
	if fallback != "" {
		t.Errorf("expected empty fallback, got %q", fallback)
	}
}

// TestExtractPrompt_NewlineReplacedWithSpace mirrors Python's
// `raw.replace("\n", " ")` — multi-line prompts collapse to single line.
func TestExtractPrompt_NewlineReplacedWithSpace(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": "line one\nline two\nline three",
		},
	}
	got, _ := extractPrompt(entry)
	want := "line one line two line three"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// TestExtractPrompt_TruncationUsesUnicodeEllipsis verifies the rune-count
// truncation produces the Unicode "…" character (Python parity), not "...".
func TestExtractPrompt_TruncationUsesUnicodeEllipsis(t *testing.T) {
	long := ""
	for i := 0; i < 250; i++ {
		long += "x"
	}
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": long,
		},
	}
	got, _ := extractPrompt(entry)
	if len(got) == 0 {
		t.Fatalf("got empty prompt")
	}
	// Last rune should be the Unicode ellipsis, not "..."
	runes := []rune(got)
	if runes[len(runes)-1] != '…' {
		t.Errorf("expected trailing Unicode ellipsis '…', got %q", string(runes[len(runes)-1]))
	}
	if len(runes) != 201 { // 200 chars + 1 ellipsis
		t.Errorf("expected 201 runes (200 + ellipsis), got %d", len(runes))
	}
}

// TestExtractPrompt_TrailingWhitespaceTrimmedBeforeEllipsis mirrors
// Python's `.rstrip()` before appending the ellipsis.
func TestExtractPrompt_TrailingWhitespaceTrimmedBeforeEllipsis(t *testing.T) {
	long := ""
	for i := 0; i < 195; i++ {
		long += "a"
	}
	long += "     "                                    // trailing spaces inside the 200-char window
	long += "remainder that pushes past the threshold" // pushes past 200
	entry := map[string]any{
		"type":    "user",
		"message": map[string]any{"content": long},
	}
	got, _ := extractPrompt(entry)
	runes := []rune(got)
	// The 196th–200th runes are spaces; rstrip must remove them so the
	// ellipsis attaches directly to the last 'a'.
	if len(runes) < 2 {
		t.Fatalf("got too short result: %q", got)
	}
	if runes[len(runes)-2] != 'a' {
		t.Errorf("expected last char before ellipsis to be 'a', got %q", string(runes[len(runes)-2]))
	}
	if runes[len(runes)-1] != '…' {
		t.Errorf("expected trailing ellipsis, got %q", string(runes[len(runes)-1]))
	}
}

// TestExtractPrompt_FirstTextBlockWins mirrors Python's per-block iteration:
// the first text block that yields a non-empty stripped result is returned;
// later blocks are not concatenated.
func TestExtractPrompt_FirstTextBlockWins(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "first block"},
				map[string]any{"type": "text", "text": "second block"},
			},
		},
	}
	got, _ := extractPrompt(entry)
	// Pre-fix, Go joined the two with "\n" giving "first block\nsecond block".
	// Python returns just "first block".
	if got != "first block" {
		t.Errorf("expected only first block, got %q", got)
	}
}

// TestExtractPrompt_SkipCommandReturnsFallback verifies slash-command
// messages return as fallback rather than the prompt.
func TestExtractPrompt_SkipCommandReturnsFallback(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": "<command-name>commit</command-name>",
		},
	}
	prompt, fallback := extractPrompt(entry)
	if prompt != "" {
		t.Errorf("expected empty prompt for command message, got %q", prompt)
	}
	if fallback != "commit" {
		t.Errorf("expected fallback 'commit', got %q", fallback)
	}
}

// TestExtractPrompt_SkipsToolResult mirrors Python's "tool_result" filter.
func TestExtractPrompt_SkipsToolResult(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_result", "tool_use_id": "x", "content": "..."},
				map[string]any{"type": "text", "text": "should be ignored"},
			},
		},
	}
	got, _ := extractPrompt(entry)
	if got != "" {
		t.Errorf("expected empty prompt when tool_result present, got %q", got)
	}
}

// TestExtractPrompt_SkipsIsMetaAndCompactSummary mirrors Python's
// `isMeta`/`isCompactSummary` filters.
func TestExtractPrompt_SkipsIsMetaAndCompactSummary(t *testing.T) {
	for _, key := range []string{"isMeta", "isCompactSummary"} {
		entry := map[string]any{
			"type":    "user",
			key:       true,
			"message": map[string]any{"content": "should be skipped"},
		}
		got, _ := extractPrompt(entry)
		if got != "" {
			t.Errorf("%s=true entry should yield empty prompt, got %q", key, got)
		}
	}
}
