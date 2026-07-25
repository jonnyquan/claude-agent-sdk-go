package shared

import (
	"runtime"
	"strings"
	"testing"
)

func TestIsBatchScriptPath(t *testing.T) {
	// OS-independent classification, so these cases run on POSIX CI too.
	batch := []string{
		`C:\Users\me\AppData\Roaming\npm\claude.cmd`,
		`C:\tools\claude.bat`,
		"claude.cmd",
		".cmd",
		// Trailing dots and spaces are stripped at Win32 path resolution.
		"claude.cmd.",
		"claude.CMD ",
		// A last-dot scan covers the whole component, stream spec included.
		`C:\tools\claude:evil.cmd`,
		// An NTFS stream spec still opens its base file.
		`C:\tools\claude.cmd:stream`,
		// Drive-relative prefix rides in the same component.
		"C:claude.cmd",
		// Any component counts: normalization can make this resolve to the
		// batch file itself.
		`C:\tools\claude.cmd\..\..\x`,
	}
	for _, path := range batch {
		if !IsBatchScriptPath(path) {
			t.Errorf("IsBatchScriptPath(%q) = false, want true", path)
		}
	}

	native := []string{
		`C:\Program Files\claude\claude.exe`,
		"/usr/local/bin/claude",
		"claude",
		"claude.exe",
		// ".command" must not match ".cmd" — suffix compare is on the full ext.
		"claude.command",
		"batch/claude",
	}
	for _, path := range native {
		if IsBatchScriptPath(path) {
			t.Errorf("IsBatchScriptPath(%q) = true, want false", path)
		}
	}
}

func TestIsNativeWindowsExecutable(t *testing.T) {
	yes := []string{`C:\x\claude.exe`, "claude.EXE", "claude.com", "claude.exe. "}
	for _, path := range yes {
		if !IsNativeWindowsExecutable(path) {
			t.Errorf("IsNativeWindowsExecutable(%q) = false, want true", path)
		}
	}
	// PATHEXT resolution can hand back "claude.exe.cmd"; only the real final
	// extension counts, so this must not be treated as native.
	no := []string{"claude", "claude.cmd", "claude.exe.cmd", `C:\claude.exe\wrapper`}
	for _, path := range no {
		if IsNativeWindowsExecutable(path) {
			t.Errorf("IsNativeWindowsExecutable(%q) = true, want false", path)
		}
	}
}

func TestWindowsCmdMetacharacters(t *testing.T) {
	if got := WindowsCmdMetacharacters("plain-session-title"); got != nil {
		t.Errorf("expected clean value, got %q", got)
	}
	// Sorted and deduplicated.
	got := WindowsCmdMetacharacters(`a&b|c&d`)
	want := []string{"&", "|"}
	if strings.Join(got, "") != strings.Join(want, "") {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := WindowsCmdMetacharacters("line\nbreak"); len(got) != 1 {
		t.Errorf("expected newline to be rejected, got %q", got)
	}
}

func TestRejectWindowsCmdMetacharactersIsPOSIXNoOp(t *testing.T) {
	err := RejectWindowsCmdMetacharacters("resume", `evil & calc.exe`)
	if runtime.GOOS == WindowsOS {
		if err == nil {
			t.Error("expected rejection on Windows")
		}
		return
	}
	if err != nil {
		t.Errorf("POSIX behavior must be unchanged, got %v", err)
	}
}

func TestRejectWindowsBatchCLIIsPOSIXNoOp(t *testing.T) {
	err := RejectWindowsBatchCLI(`C:\npm\claude.cmd`)
	if runtime.GOOS == WindowsOS {
		if err == nil {
			t.Error("expected refusal on Windows")
		}
		return
	}
	if err != nil {
		t.Errorf("POSIX behavior must be unchanged, got %v", err)
	}
}
