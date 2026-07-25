package shared

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// CanUseToolShadowedLogger receives the advisory emitted when a CanUseTool
// callback is set but some tool calls are auto-approved before it runs. It
// defaults to the standard logger; set it to nil to silence the advisory
// entirely (the Go equivalent of filtering Python's CanUseToolShadowedWarning).
//
// Not synchronized: set it during program setup, before the first Connect.
var CanUseToolShadowedLogger func(message string) = func(message string) {
	log.Print(message)
}

// warnedShadowMessages dedupes the advisory per distinct message for the life of
// the process, matching Python's default warning filter ("once per message per
// process") rather than emitting on every Connect.
var warnedShadowMessages sync.Map

// wholeToolAllowed returns the tool an AllowedTools entry allows outright, and
// whether it allows one at all.
//
// Mirrors the CLI's rule parser: an entry allows a whole tool when it has no
// "(...)" specifier ("Read"), or when the specifier is empty or a lone wildcard
// ("Read()", "Read(*)"). A real specifier ("Bash(ls:*)") only allows matching
// invocations. Malformed entries fall back to the whole string as a tool name in
// the CLI, so they match nothing and are ignored here.
func wholeToolAllowed(entry string) (string, bool) {
	if strings.TrimSpace(entry) == "" {
		return "", false
	}
	openIndex := strings.Index(entry, "(")
	if openIndex == -1 {
		return entry, true
	}
	if openIndex == 0 || !strings.HasSuffix(entry, ")") {
		return "", false
	}
	spec := entry[openIndex+1 : len(entry)-1]
	if spec == "" || spec == "*" {
		return entry[:openIndex], true
	}
	return "", false
}

// CanUseToolShadowedMessage returns the advisory for these options, or "" when
// nothing visibly shadows the callback.
func CanUseToolShadowedMessage(permissionMode *PermissionMode, allowedTools []string) string {
	if permissionMode != nil && *permissionMode == PermissionModeBypassPermissions {
		return "CanUseTool will not be invoked: PermissionMode 'bypassPermissions' " +
			"auto-approves every tool call (except explicit deny rules) before the " +
			"callback is consulted. To gate every tool call, use a PreToolUse hook instead."
	}

	// Dedupe while preserving order: redundant configs like ["Read", "Read()"]
	// resolve to the same tool and must not report it twice.
	seen := make(map[string]struct{}, len(allowedTools))
	var shadowed []string
	for _, entry := range allowedTools {
		tool, ok := wholeToolAllowed(entry)
		if !ok {
			continue
		}
		if _, dup := seen[tool]; dup {
			continue
		}
		seen[tool] = struct{}{}
		shadowed = append(shadowed, tool)
	}
	if len(shadowed) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"CanUseTool will not be invoked for: %s. An AllowedTools entry that allows "+
			"a whole tool auto-approves it before the callback is consulted. To gate "+
			"every tool call, use a PreToolUse hook; or narrow the entry so calls fall "+
			"through to CanUseTool. Allow rules from settings files can also shadow the "+
			"callback but are not visible here.",
		strings.Join(shadowed, ", "),
	)
}

// WarnIfCanUseToolShadowed emits the advisory when a CanUseTool callback is set
// alongside options that visibly shadow it. Called once per query/connect.
//
// Advisory only (never an error): shadowing can be intentional, e.g. a callback
// used solely for tools outside AllowedTools. Each distinct message is emitted
// once per process, so two callers with the same shadowed config together
// produce one advisory rather than one each.
func WarnIfCanUseToolShadowed(options *Options) {
	if options == nil || options.CanUseTool == nil {
		return
	}

	// Skills "all" makes the transport append a bare "Skill" to the effective
	// AllowedTools, so it shadows the callback just like a hand-written entry.
	// An explicit skill list appends Skill(name) specifiers, which do not.
	allowedTools := options.AllowedTools
	if options.Skills != nil && options.Skills.All {
		hasSkill := false
		for _, entry := range allowedTools {
			if entry == "Skill" {
				hasSkill = true
				break
			}
		}
		if !hasSkill {
			allowedTools = append(append([]string(nil), allowedTools...), "Skill")
		}
	}

	message := CanUseToolShadowedMessage(options.PermissionMode, allowedTools)
	if message == "" {
		return
	}
	if _, alreadyWarned := warnedShadowMessages.LoadOrStore(message, struct{}{}); alreadyWarned {
		return
	}
	if logger := CanUseToolShadowedLogger; logger != nil {
		logger(message)
	}
}
