package shared

import (
	"strings"
	"testing"
)

func TestWholeToolAllowed(t *testing.T) {
	shadowing := map[string]string{
		"Read":     "Read",
		"Read()":   "Read",
		"Read(*)":  "Read",
		"Bash":     "Bash",
		"Bash( )":  "",
		"Bash(ls)": "",
	}
	for entry, want := range shadowing {
		got, ok := wholeToolAllowed(entry)
		if want == "" {
			if ok {
				t.Errorf("wholeToolAllowed(%q) = %q, want no whole-tool match", entry, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("wholeToolAllowed(%q) = (%q, %v), want (%q, true)", entry, got, ok, want)
		}
	}

	// Narrow specifiers and malformed entries never shadow.
	for _, entry := range []string{"Bash(ls:*)", "", "   ", "(Read)", "Read(", "Read(*"} {
		if got, ok := wholeToolAllowed(entry); ok {
			t.Errorf("wholeToolAllowed(%q) = %q, want no whole-tool match", entry, got)
		}
	}
}

func TestCanUseToolShadowedMessage(t *testing.T) {
	if msg := CanUseToolShadowedMessage(nil, []string{"Bash(ls:*)"}); msg != "" {
		t.Errorf("narrow specifier must not warn, got %q", msg)
	}

	bypass := PermissionModeBypassPermissions
	msg := CanUseToolShadowedMessage(&bypass, nil)
	if !strings.Contains(msg, "bypassPermissions") {
		t.Errorf("expected bypassPermissions advisory, got %q", msg)
	}

	// Redundant entries resolve to the same tool and must be reported once.
	msg = CanUseToolShadowedMessage(nil, []string{"Read", "Read()", "Read(*)", "Write"})
	if strings.Count(msg, "Read") != 1 {
		t.Errorf("expected Read reported once, got %q", msg)
	}
	if !strings.Contains(msg, "Write") {
		t.Errorf("expected Write reported, got %q", msg)
	}
}

func TestWarnIfCanUseToolShadowed(t *testing.T) {
	original := CanUseToolShadowedLogger
	t.Cleanup(func() { CanUseToolShadowedLogger = original })

	var emitted []string
	CanUseToolShadowedLogger = func(message string) { emitted = append(emitted, message) }

	// No callback: nothing to shadow.
	WarnIfCanUseToolShadowed(&Options{AllowedTools: []string{"Read"}})
	if len(emitted) != 0 {
		t.Fatalf("expected no advisory without CanUseTool, got %v", emitted)
	}

	var callback CanUseToolCallback = func(_ string, _ map[string]any, _ ToolPermissionContext) (PermissionResult, error) {
		return nil, nil
	}

	// Skills "all" injects a bare "Skill" into the effective AllowedTools, so
	// it shadows the callback just like a hand-written entry.
	WarnIfCanUseToolShadowed(&Options{CanUseTool: callback, Skills: SkillsAll()})
	if len(emitted) != 1 || !strings.Contains(emitted[0], "Skill") {
		t.Fatalf("expected Skill advisory, got %v", emitted)
	}

	// The same message is emitted once per process, not once per call.
	WarnIfCanUseToolShadowed(&Options{CanUseTool: callback, Skills: SkillsAll()})
	if len(emitted) != 1 {
		t.Fatalf("expected dedupe, got %v", emitted)
	}

	// An explicit skill list appends Skill(name) specifiers, which do not shadow.
	emitted = nil
	WarnIfCanUseToolShadowed(&Options{CanUseTool: callback, Skills: SkillsList("deploy")})
	if len(emitted) != 0 {
		t.Fatalf("explicit skill list must not warn, got %v", emitted)
	}
}
