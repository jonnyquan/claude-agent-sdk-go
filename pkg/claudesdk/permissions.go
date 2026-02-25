package claudesdk

// Re-export permission types from internal/shared for public API
import (
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// Permission types
type PermissionUpdateType = shared.PermissionUpdateType

const (
	PermissionUpdateTypeAddRules          = shared.PermissionUpdateTypeAddRules
	PermissionUpdateTypeReplaceRules      = shared.PermissionUpdateTypeReplaceRules
	PermissionUpdateTypeRemoveRules       = shared.PermissionUpdateTypeRemoveRules
	PermissionUpdateTypeSetMode           = shared.PermissionUpdateTypeSetMode
	PermissionUpdateTypeAddDirectories    = shared.PermissionUpdateTypeAddDirectories
	PermissionUpdateTypeRemoveDirectories = shared.PermissionUpdateTypeRemoveDirectories
)

// Note: PermissionDestination is re-exported in options.go

type PermissionBehavior = shared.PermissionBehavior

const (
	PermissionBehaviorAllow = shared.PermissionBehaviorAllow
	PermissionBehaviorDeny  = shared.PermissionBehaviorDeny
	PermissionBehaviorAsk   = shared.PermissionBehaviorAsk
)

type PermissionRule = shared.PermissionRule
type PermissionUpdate = shared.PermissionUpdate
type ToolPermissionContext = shared.ToolPermissionContext
type PermissionResultAllow = shared.PermissionResultAllow
type PermissionResultDeny = shared.PermissionResultDeny
type PermissionResult = shared.PermissionResult
type CanUseToolCallback = shared.CanUseToolCallback

// CanUseTool is an alias for CanUseToolCallback, matching the Python SDK name.
type CanUseTool = shared.CanUseToolCallback

// Helper functions
var (
	NewPermissionAllow  = shared.NewPermissionAllow
	NewPermissionDeny   = shared.NewPermissionDeny
	NewPermissionRule   = shared.NewPermissionRule
	NewPermissionUpdate = shared.NewPermissionUpdate
)
