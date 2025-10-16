package shared

import (
	"context"
)

// HookEvent represents the type of hook event.
type HookEvent string

const (
	HookEventPreToolUse       HookEvent = "PreToolUse"
	HookEventPostToolUse      HookEvent = "PostToolUse"
	HookEventUserPromptSubmit HookEvent = "UserPromptSubmit"
	HookEventStop             HookEvent = "Stop"
	HookEventSubagentStop     HookEvent = "SubagentStop"
	HookEventPreCompact       HookEvent = "PreCompact"
)

// BaseHookInput contains common fields present across many hook events.
type BaseHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
}

// PreToolUseHookInput represents input data for PreToolUse hook events.
type PreToolUseHookInput struct {
	BaseHookInput
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
}

// PostToolUseHookInput represents input data for PostToolUse hook events.
type PostToolUseHookInput struct {
	BaseHookInput
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	ToolResponse  any            `json:"tool_response"`
}

// UserPromptSubmitHookInput represents input data for UserPromptSubmit hook events.
type UserPromptSubmitHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Prompt        string `json:"prompt"`
}

// StopHookInput represents input data for Stop hook events.
type StopHookInput struct {
	BaseHookInput
	HookEventName  string `json:"hook_event_name"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// SubagentStopHookInput represents input data for SubagentStop hook events.
type SubagentStopHookInput struct {
	BaseHookInput
	HookEventName  string `json:"hook_event_name"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// PreCompactHookInput represents input data for PreCompact hook events.
type PreCompactHookInput struct {
	BaseHookInput
	HookEventName      string  `json:"hook_event_name"`
	Trigger            string  `json:"trigger"` // "manual" or "auto"
	CustomInstructions *string `json:"custom_instructions,omitempty"`
}

// HookInput is a union type for all hook inputs.
// Use type assertion to access specific fields based on hook_event_name.
type HookInput = map[string]any

// PreToolUseHookSpecificOutput represents hook-specific output for PreToolUse events.
type PreToolUseHookSpecificOutput struct {
	HookEventName            string         `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`       // "allow", "deny", or "ask"
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"` 
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
}

// PostToolUseHookSpecificOutput represents hook-specific output for PostToolUse events.
type PostToolUseHookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// UserPromptSubmitHookSpecificOutput represents hook-specific output for UserPromptSubmit events.
type UserPromptSubmitHookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// SessionStartHookSpecificOutput represents hook-specific output for SessionStart events.
type SessionStartHookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

// HookSpecificOutput is a union type for hook-specific outputs.
type HookSpecificOutput = map[string]any

// AsyncHookJSONOutput represents async hook output that defers hook execution.
type AsyncHookJSONOutput struct {
	Async        bool `json:"async"`
	AsyncTimeout *int `json:"asyncTimeout,omitempty"`
}

// SyncHookJSONOutput represents synchronous hook output with control and decision fields.
type SyncHookJSONOutput struct {
	// Common control fields
	Continue       *bool  `json:"continue,omitempty"`
	SuppressOutput *bool  `json:"suppressOutput,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`

	// Decision fields
	Decision      string `json:"decision,omitempty"` // "block"
	SystemMessage string `json:"systemMessage,omitempty"`
	Reason        string `json:"reason,omitempty"`

	// Hook-specific outputs
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookJSONOutput is the output type for hook callbacks.
// It can be either async or sync output.
type HookJSONOutput = map[string]any

// HookContext provides context information for hook callbacks.
type HookContext struct {
	Context context.Context
	Signal  any // Future: abort signal support
}

// HookCallback is the function signature for hook callbacks.
// Parameters:
//   - input: Hook input data with discriminated unions based on hook_event_name
//   - toolUseID: Optional tool use identifier
//   - ctx: Hook context with abort signal support (currently placeholder)
//
// Returns:
//   - HookJSONOutput: Hook output with control and decision fields
//   - error: Error if hook execution fails
type HookCallback func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error)

// HookMatcher configures hook callbacks for specific patterns.
type HookMatcher struct {
	Matcher string         `json:"matcher"`
	Hooks   []HookCallback `json:"-"` // Not serialized, handled by SDK
}

// Permission decision constants
const (
	PermissionDecisionAllow = "allow"
	PermissionDecisionDeny  = "deny"
	PermissionDecisionAsk   = "ask"
)

// Helper functions to create hook outputs

// NewPreToolUseOutput creates a PreToolUse hook output with permission decision.
func NewPreToolUseOutput(decision, reason string, updatedInput map[string]any) HookJSONOutput {
	output := make(HookJSONOutput)
	
	hookSpecific := map[string]any{
		"hookEventName":       "PreToolUse",
		"permissionDecision":  decision,
	}
	
	if reason != "" {
		output["reason"] = reason
		hookSpecific["permissionDecisionReason"] = reason
	}
	
	if updatedInput != nil {
		hookSpecific["updatedInput"] = updatedInput
	}
	
	output["hookSpecificOutput"] = hookSpecific
	return output
}

// NewPostToolUseOutput creates a PostToolUse hook output with additional context.
func NewPostToolUseOutput(additionalContext string) HookJSONOutput {
	output := make(HookJSONOutput)
	
	if additionalContext != "" {
		hookSpecific := map[string]any{
			"hookEventName":     "PostToolUse",
			"additionalContext": additionalContext,
		}
		output["hookSpecificOutput"] = hookSpecific
	}
	
	return output
}

// NewBlockingOutput creates a hook output that blocks execution.
func NewBlockingOutput(systemMessage, reason string) HookJSONOutput {
	output := make(HookJSONOutput)
	output["decision"] = "block"
	
	if systemMessage != "" {
		output["systemMessage"] = systemMessage
	}
	
	if reason != "" {
		output["reason"] = reason
	}
	
	return output
}

// NewStopOutput creates a hook output that stops execution with a reason.
func NewStopOutput(stopReason string) HookJSONOutput {
	output := make(HookJSONOutput)
	continueVal := false
	output["continue"] = &continueVal
	
	if stopReason != "" {
		output["stopReason"] = stopReason
	}
	
	return output
}

// NewAsyncOutput creates a hook output that defers execution.
func NewAsyncOutput(timeout *int) HookJSONOutput {
	output := make(HookJSONOutput)
	output["async"] = true
	
	if timeout != nil {
		output["asyncTimeout"] = *timeout
	}
	
	return output
}
