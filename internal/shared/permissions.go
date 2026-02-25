package shared

import (
	"context"
)

// PermissionUpdateType represents the type of permission update.
type PermissionUpdateType string

const (
	PermissionUpdateTypeAddRules         PermissionUpdateType = "addRules"
	PermissionUpdateTypeReplaceRules     PermissionUpdateType = "replaceRules"
	PermissionUpdateTypeRemoveRules      PermissionUpdateType = "removeRules"
	PermissionUpdateTypeSetMode          PermissionUpdateType = "setMode"
	PermissionUpdateTypeAddDirectories   PermissionUpdateType = "addDirectories"
	PermissionUpdateTypeRemoveDirectories PermissionUpdateType = "removeDirectories"
)

// PermissionDestination specifies where the permission update applies.
type PermissionDestination string

const (
	PermissionDestinationSession         PermissionDestination = "session"
	PermissionDestinationUserSettings    PermissionDestination = "userSettings"
	PermissionDestinationProjectSettings PermissionDestination = "projectSettings"
	PermissionDestinationLocalSettings   PermissionDestination = "localSettings"
)

// PermissionBehavior represents the behavior for a permission decision.
type PermissionBehavior string

const (
	PermissionBehaviorAllow PermissionBehavior = "allow"
	PermissionBehaviorDeny  PermissionBehavior = "deny"
	PermissionBehaviorAsk   PermissionBehavior = "ask"
)

// PermissionRule represents a rule for tool permissions.
type PermissionRule struct {
	ToolName    string  `json:"toolName"`
	RuleContent *string `json:"ruleContent"`
}

// PermissionUpdate represents a permission update request.
type PermissionUpdate struct {
	Type        PermissionUpdateType   `json:"type"`
	Destination *PermissionDestination `json:"destination,omitempty"`
	Rules       []PermissionRule       `json:"rules,omitempty"`
	Behavior    *string                `json:"behavior,omitempty"`
	Mode        *string                `json:"mode,omitempty"`
	Directories []string               `json:"directories,omitempty"`
}

// ToolPermissionContext provides context for tool permission callbacks.
type ToolPermissionContext struct {
	Context     context.Context
	Signal      any                 // Future: abort signal support
	Suggestions []PermissionUpdate  // Permission suggestions from CLI
}

// PermissionResultAllow represents an allow permission result.
type PermissionResultAllow struct {
	Behavior           string             `json:"behavior"` // "allow"
	UpdatedInput       any                `json:"updatedInput,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updatedPermissions,omitempty"`
}

// PermissionResultDeny represents a deny permission result.
type PermissionResultDeny struct {
	Behavior  string `json:"behavior"` // "deny"
	Message   string `json:"message,omitempty"`
	Interrupt bool   `json:"interrupt,omitempty"`
}

// PermissionResult is a union type for permission results.
// Use type assertion to determine if it's Allow or Deny.
type PermissionResult interface {
	isPermissionResult()
}

func (p *PermissionResultAllow) isPermissionResult() {}
func (p *PermissionResultDeny) isPermissionResult()  {}

// CanUseToolCallback is the function signature for tool permission callbacks.
// Parameters:
//   - toolName: Name of the tool being requested
//   - toolInput: Input parameters for the tool
//   - ctx: Context with permission suggestions from CLI
//
// Returns:
//   - PermissionResult: Allow or Deny result
//   - error: Error if callback execution fails
type CanUseToolCallback func(toolName string, toolInput map[string]any, ctx ToolPermissionContext) (PermissionResult, error)

// Helper functions to create permission results

// NewPermissionAllow creates a permission allow result.
func NewPermissionAllow(updatedInput any, updatedPermissions []PermissionUpdate) *PermissionResultAllow {
	return &PermissionResultAllow{
		Behavior:           "allow",
		UpdatedInput:       updatedInput,
		UpdatedPermissions: updatedPermissions,
	}
}

// NewPermissionDeny creates a permission deny result.
func NewPermissionDeny(message string, interrupt bool) *PermissionResultDeny {
	return &PermissionResultDeny{
		Behavior:  "deny",
		Message:   message,
		Interrupt: interrupt,
	}
}

// NewPermissionRule creates a permission rule.
func NewPermissionRule(toolName string, ruleContent *string) PermissionRule {
	return PermissionRule{
		ToolName:    toolName,
		RuleContent: ruleContent,
	}
}

// NewPermissionUpdate creates a permission update with the given type.
func NewPermissionUpdate(updateType PermissionUpdateType) *PermissionUpdate {
	return &PermissionUpdate{
		Type: updateType,
	}
}

// WithDestination sets the destination for the permission update.
func (p *PermissionUpdate) WithDestination(dest PermissionDestination) *PermissionUpdate {
	p.Destination = &dest
	return p
}

// WithRules sets the rules for the permission update.
func (p *PermissionUpdate) WithRules(rules []PermissionRule) *PermissionUpdate {
	p.Rules = rules
	return p
}

// WithBehavior sets the behavior for the permission update.
func (p *PermissionUpdate) WithBehavior(behavior string) *PermissionUpdate {
	p.Behavior = &behavior
	return p
}

// WithMode sets the mode for the permission update.
func (p *PermissionUpdate) WithMode(mode string) *PermissionUpdate {
	p.Mode = &mode
	return p
}

// WithDirectories sets the directories for the permission update.
func (p *PermissionUpdate) WithDirectories(dirs []string) *PermissionUpdate {
	p.Directories = dirs
	return p
}
