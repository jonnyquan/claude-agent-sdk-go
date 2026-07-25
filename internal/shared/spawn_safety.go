package shared

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// WindowsOS is the runtime.GOOS value for Windows.
const WindowsOS = "windows"

// cmdExeMetacharacters lists the cmd.exe metacharacters, plus the quote
// character cmd.exe uses to toggle its quoting state, and "!", which expands
// like "%" when delayed expansion is enabled.
//
// Go's os/exec quotes arguments for the MSVCRT argv rules only (it adds quotes
// only around whitespace and quotes), so in a whitespace-free argument these
// characters reach a cmd.exe command line verbatim. See RejectWindowsBatchCLI
// and RejectWindowsCmdMetacharacters.
const cmdExeMetacharacters = `&|<>^%!"`

// IsBatchScriptPath reports whether cliPath names a .bat/.cmd batch script.
//
// OS-independent by design so it stays testable on POSIX CI while the code runs
// on Windows; the OS gate lives in RejectWindowsBatchCLI.
//
// Classifies EVERY path component, not only the final one. Win32 opens a path
// after lexical normalization — "." / ".." collapsing, repeated separators, and
// position-dependent trailing dot/space trimming (a middle ".. " or "..." stays
// a literal name while a final one trims to ".." or vanishes) — and any attempt
// to re-derive the effective final component here is a race against that
// ruleset: get one rule slightly wrong and a spelling such as
// `claude.cmd\...\..` resolves to claude.cmd on Windows while the simulation
// lands on some other name. Refusing whenever ANY component carries a batch
// extension closes that whole class outright, because every normalization trick
// still has to spell the .bat/.cmd component somewhere in the string. It costs
// nothing legitimate: no real claude.exe lives beneath a directory named like a
// batch file.
//
// Within a component, Win32 finds the extension with a last-dot scan over the
// WHOLE component, stream spec included — "claude:evil.cmd" has extension
// ".cmd" — while an NTFS stream spec also opens its base file
// ("claude.cmd:stream" opens claude.cmd), and a drive prefix ("C:claude.cmd")
// rides in the same component. Splitting each component on ":" covers all of
// these: colons cannot appear in real file names, so no legitimate segment is
// over-refused. Trailing dots and spaces, which Windows strips at path
// resolution, are trimmed per segment (the same normalization Rust's
// CVE-2024-24576 fix applies), and a bare ".cmd" counts as a batch extension
// (as Win32 PathFindExtension treats it).
func IsBatchScriptPath(cliPath string) bool {
	for _, component := range strings.Split(strings.ReplaceAll(cliPath, `\`, "/"), "/") {
		for _, segment := range strings.Split(component, ":") {
			name := strings.ToLower(strings.TrimRight(segment, ". "))
			if strings.HasSuffix(name, ".bat") || strings.HasSuffix(name, ".cmd") {
				return true
			}
		}
	}
	return false
}

// IsNativeWindowsExecutable reports whether cliPath's final component names an
// image CreateProcess runs directly (.exe / .com).
//
// Used only to decide which discovery result to prefer. It is not a security
// gate: every returned path still passes RejectWindowsBatchCLI before a spawn.
func IsNativeWindowsExecutable(cliPath string) bool {
	parts := strings.Split(strings.ReplaceAll(cliPath, `\`, "/"), "/")
	name := strings.ToLower(strings.TrimRight(parts[len(parts)-1], ". "))
	return strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".com")
}

// RejectWindowsBatchCLI refuses to execute a .bat/.cmd script as the CLI on
// Windows. Always nil off Windows.
//
// Windows has no shebang mechanism: CreateProcess runs batch scripts by
// silently rewriting the spawn into a `cmd.exe /c` invocation, and cmd.exe
// re-parses the whole command line at execution time. Go's os/exec quotes
// arguments for the MSVCRT argv rules only, not for cmd.exe, so cmd.exe
// metacharacters inside an argument value — for example a session title passed
// to --resume — reach cmd.exe unescaped and can execute injected commands.
// Reliable escaping for cmd.exe does not exist (%VAR% expands even inside
// double quotes), so spawning a batch script with runtime-provided arguments
// cannot be made safe. Refusing is the same remediation Node.js shipped for this
// vulnerability class (CVE-2024-27980, "BatBadBut").
//
// In practice this refuses npm's claude.cmd shim, which discovery returns only
// when no native claude.exe is discoverable. The alternatives in the error
// message avoid cmd.exe entirely.
func RejectWindowsBatchCLI(cliPath string) error {
	if runtime.GOOS != WindowsOS || !IsBatchScriptPath(cliPath) {
		return nil
	}
	return NewConnectionError(fmt.Sprintf(
		"refusing to execute batch script %q: Windows runs .bat/.cmd files via "+
			"cmd.exe, which can execute commands injected through CLI arguments, "+
			"and no reliable escaping for cmd.exe exists. Use a native claude "+
			"executable instead: install Claude Code natively "+
			"(irm https://claude.ai/install.ps1 | iex), point WithCLIPath(...) at "+
			"a claude.exe, or use a build that bundles claude.exe.",
		cliPath,
	), nil)
}

// WindowsCmdMetacharacters returns the sorted, deduplicated set of characters in
// value that are unsafe to pass on a Windows command line. Empty when value is
// clean. OS-independent so it stays testable on POSIX.
func WindowsCmdMetacharacters(value string) []string {
	seen := make(map[rune]struct{})
	for _, c := range value {
		if strings.ContainsRune(cmdExeMetacharacters, c) || c == '\r' || c == '\n' {
			seen[c] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	bad := make([]string, 0, len(seen))
	for c := range seen {
		bad = append(bad, string(c))
	}
	sort.Strings(bad)
	return bad
}

// RejectWindowsCmdMetacharacters is defense in depth for Windows: it rejects
// cmd.exe metacharacters in an option value. Always nil off Windows.
//
// With batch-script spawning refused (RejectWindowsBatchCLI) these characters
// are harmless, since os/exec quotes correctly for native executables. They are
// rejected anyway so that Resume / SessionID values, which applications commonly
// take from external input, stay inert even if a cmd.exe hop is ever
// reintroduced between the SDK and the CLI. No format is imposed beyond this
// (resume values may be arbitrary session titles, not only UUIDs), and POSIX
// behavior is unchanged.
func RejectWindowsCmdMetacharacters(optionName, value string) error {
	if runtime.GOOS != WindowsOS {
		return nil
	}
	bad := WindowsCmdMetacharacters(value)
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf(
		"%s value %q contains characters that are unsafe to pass on a Windows command line: %q",
		optionName, value, bad,
	)
}

// ValidateSpawnOptions checks the option values that end up as CLI argv tokens
// for Windows command-line injection risks. Called before the CLI is spawned.
func ValidateSpawnOptions(options *Options) error {
	if options == nil {
		return nil
	}
	if options.Resume != nil {
		if err := RejectWindowsCmdMetacharacters("resume", *options.Resume); err != nil {
			return err
		}
	}
	if options.SessionID != nil {
		if err := RejectWindowsCmdMetacharacters("session_id", *options.SessionID); err != nil {
			return err
		}
	}
	return nil
}
