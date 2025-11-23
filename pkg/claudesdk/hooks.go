package claudesdk

// Re-export hook types from internal/shared for public API
import (
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// Hook event types
type HookEvent = shared.HookEvent

const (
	HookEventPreToolUse       = shared.HookEventPreToolUse
	HookEventPostToolUse      = shared.HookEventPostToolUse
	HookEventUserPromptSubmit = shared.HookEventUserPromptSubmit
	HookEventStop             = shared.HookEventStop
	HookEventSubagentStop     = shared.HookEventSubagentStop
	HookEventPreCompact       = shared.HookEventPreCompact
)

// Hook input types
type BaseHookInput = shared.BaseHookInput
type PreToolUseHookInput = shared.PreToolUseHookInput
type PostToolUseHookInput = shared.PostToolUseHookInput
type UserPromptSubmitHookInput = shared.UserPromptSubmitHookInput
type StopHookInput = shared.StopHookInput
type SubagentStopHookInput = shared.SubagentStopHookInput
type PreCompactHookInput = shared.PreCompactHookInput
type HookInput = shared.HookInput

// Hook output types
type PreToolUseHookSpecificOutput = shared.PreToolUseHookSpecificOutput
type PostToolUseHookSpecificOutput = shared.PostToolUseHookSpecificOutput
type UserPromptSubmitHookSpecificOutput = shared.UserPromptSubmitHookSpecificOutput
type SessionStartHookSpecificOutput = shared.SessionStartHookSpecificOutput
type HookSpecificOutput = shared.HookSpecificOutput
type AsyncHookJSONOutput = shared.AsyncHookJSONOutput
type SyncHookJSONOutput = shared.SyncHookJSONOutput
type HookJSONOutput = shared.HookJSONOutput

// Hook context and callbacks
type HookContext = shared.HookContext
type HookCallback = shared.HookCallback
type HookMatcher = shared.HookMatcher

// Permission decision constants
const (
	PermissionDecisionAllow = shared.PermissionDecisionAllow
	PermissionDecisionDeny  = shared.PermissionDecisionDeny
	PermissionDecisionAsk   = shared.PermissionDecisionAsk
)

// Helper functions
var (
	NewPreToolUseOutput  = shared.NewPreToolUseOutput
	NewPostToolUseOutput = shared.NewPostToolUseOutput
	NewBlockingOutput    = shared.NewBlockingOutput
	NewStopOutput        = shared.NewStopOutput
	NewAsyncOutput       = shared.NewAsyncOutput
)
