package claudecode

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

type PermissionDestination = shared.PermissionDestination

const (
	PermissionDestinationSession  = shared.PermissionDestinationSession
	PermissionDestinationSettings = shared.PermissionDestinationSettings
)

type PermissionRule = shared.PermissionRule
type PermissionUpdate = shared.PermissionUpdate
type ToolPermissionContext = shared.ToolPermissionContext
type PermissionResultAllow = shared.PermissionResultAllow
type PermissionResultDeny = shared.PermissionResultDeny
type PermissionResult = shared.PermissionResult
type CanUseToolCallback = shared.CanUseToolCallback

// Helper functions
var (
	NewPermissionAllow  = shared.NewPermissionAllow
	NewPermissionDeny   = shared.NewPermissionDeny
	NewPermissionRule   = shared.NewPermissionRule
	NewPermissionUpdate = shared.NewPermissionUpdate
)
