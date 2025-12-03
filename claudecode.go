// Package claudecode provides backward compatibility for the Claude Code SDK for Go.
//
// This package re-exports the main SDK functionality for backward compatibility.
// New code should import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk" directly.
//
// Deprecated: Use github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk instead.
package claudecode

import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"

// Re-export main types and functions for backward compatibility

// Client represents a Claude Code client instance.
// Deprecated: Use claudesdk.Client instead.
type Client = claudesdk.Client

// MessageIterator represents an iterator over messages from Claude.
// Deprecated: Use claudesdk.MessageIterator instead.
type MessageIterator = claudesdk.MessageIterator

// Option represents a configuration option for the client or query.
// Deprecated: Use claudesdk.Option instead.
type Option = claudesdk.Option

// Message represents a message in the conversation.
// Deprecated: Use claudesdk.Message instead.
type Message = claudesdk.Message

// UserMessage represents a message from the user.
// Deprecated: Use claudesdk.UserMessage instead.
type UserMessage = claudesdk.UserMessage

// AssistantMessage represents a message from Claude.
// Deprecated: Use claudesdk.AssistantMessage instead.
type AssistantMessage = claudesdk.AssistantMessage

// ToolUseBlock represents a tool use request from Claude.
// Deprecated: Use claudesdk.ToolUseBlock instead.
type ToolUseBlock = claudesdk.ToolUseBlock

// ToolResultBlock represents the result of a tool execution.
// Deprecated: Use claudesdk.ToolResultBlock instead.
type ToolResultBlock = claudesdk.ToolResultBlock

// Additional types for compatibility

// Hook types
// Deprecated: Use claudesdk types instead.
type HookInput = claudesdk.HookInput
type HookContext = claudesdk.HookContext  
type HookJSONOutput = claudesdk.HookJSONOutput
type HookMatcher = claudesdk.HookMatcher
type HookEvent = claudesdk.HookEvent
type HookCallback = claudesdk.HookCallback

// Plugin types
// Deprecated: Use claudesdk types instead.
type PluginConfig = claudesdk.PluginConfig
type PluginType = claudesdk.PluginType

// Sandbox types
// Deprecated: Use claudesdk types instead.
type SandboxSettings = claudesdk.SandboxSettings
type SandboxNetworkConfig = claudesdk.SandboxNetworkConfig
type SandboxIgnoreViolations = claudesdk.SandboxIgnoreViolations

// Content types
// Deprecated: Use claudesdk types instead.
type TextBlock = claudesdk.TextBlock
type ResultMessage = claudesdk.ResultMessage
type ToolContent = claudesdk.ToolContent
type ToolDef = claudesdk.ToolDef
type ContentBlock = claudesdk.ContentBlock
type SystemMessage = claudesdk.SystemMessage

// Permission types
// Deprecated: Use claudesdk types instead.
type PermissionMode = claudesdk.PermissionMode

// MCP types
// Deprecated: Use claudesdk types instead.
type McpServerConfig = claudesdk.McpServerConfig

// Constants
// Deprecated: Use claudesdk constants instead.
const (
	PluginTypeLocal = claudesdk.PluginTypeLocal
	
	// Hook events
	HookEventPreToolUse       = claudesdk.HookEventPreToolUse
	HookEventPostToolUse      = claudesdk.HookEventPostToolUse
	HookEventUserPromptSubmit = claudesdk.HookEventUserPromptSubmit
	HookEventStop             = claudesdk.HookEventStop
	HookEventSubagentStop     = claudesdk.HookEventSubagentStop
	HookEventPreCompact       = claudesdk.HookEventPreCompact
	
	// Permission decisions
	PermissionDecisionAllow = claudesdk.PermissionDecisionAllow
	PermissionDecisionDeny  = claudesdk.PermissionDecisionDeny
	PermissionDecisionAsk   = claudesdk.PermissionDecisionAsk
	
	// Permission modes
	PermissionModeDefault          = claudesdk.PermissionModeDefault
	PermissionModeAcceptEdits      = claudesdk.PermissionModeAcceptEdits
	PermissionModeBypassPermissions = claudesdk.PermissionModeBypassPermissions
)

// Re-export main functions

// NewClient creates a new Claude Code client with the given options.
// Deprecated: Use claudesdk.NewClient instead.
var NewClient = claudesdk.NewClient

// WithClient provides Go-idiomatic resource management.
// Deprecated: Use claudesdk.WithClient instead.
var WithClient = claudesdk.WithClient

// Query performs a one-shot query to Claude Code with automatic cleanup.
// Deprecated: Use claudesdk.Query instead.
var Query = claudesdk.Query

// Re-export option constructors

// WithSystemPrompt sets the system prompt for the conversation.
// Deprecated: Use claudesdk.WithSystemPrompt instead.
var WithSystemPrompt = claudesdk.WithSystemPrompt

// WithCwd sets the current working directory for Claude Code.
// Deprecated: Use claudesdk.WithCwd instead.
var WithCwd = claudesdk.WithCwd

// WithModel sets the model to use for the conversation.
// Deprecated: Use claudesdk.WithModel instead.
var WithModel = claudesdk.WithModel

// WithMcpServers connects to MCP (Model Context Protocol) servers.
// Deprecated: Use claudesdk.WithMcpServers instead.
var WithMcpServers = claudesdk.WithMcpServers

// Additional option functions

// WithPlugins configures custom plugins.
// Deprecated: Use claudesdk.WithPlugins instead.
var WithPlugins = claudesdk.WithPlugins

// WithAllowedTools sets the allowed tools list.
// Deprecated: Use claudesdk.WithAllowedTools instead.
var WithAllowedTools = claudesdk.WithAllowedTools

// WithMaxBudgetUSD sets the maximum budget in USD for API costs.
// Deprecated: Use claudesdk.WithMaxBudgetUSD instead.
var WithMaxBudgetUSD = claudesdk.WithMaxBudgetUSD

// WithMaxThinkingTokens sets the maximum thinking tokens.
// Deprecated: Use claudesdk.WithMaxThinkingTokens instead.
var WithMaxThinkingTokens = claudesdk.WithMaxThinkingTokens

// WithResume resumes a previous session.
// Deprecated: Use claudesdk.WithResume instead.
var WithResume = claudesdk.WithResume

// WithPermissionMode sets the permission mode.
// Deprecated: Use claudesdk.WithPermissionMode instead.
var WithPermissionMode = claudesdk.WithPermissionMode

// WithMaxTurns sets the maximum number of conversation turns.
// Deprecated: Use claudesdk.WithMaxTurns instead.
var WithMaxTurns = claudesdk.WithMaxTurns

// WithEnv sets environment variables for the subprocess.
// Deprecated: Use claudesdk.WithEnv instead.
var WithEnv = claudesdk.WithEnv

// WithEnvVar sets a single environment variable.
// Deprecated: Use claudesdk.WithEnvVar instead.
var WithEnvVar = claudesdk.WithEnvVar

// WithSandbox configures sandbox settings.
// Deprecated: Use claudesdk.WithSandbox instead.
var WithSandbox = claudesdk.WithSandbox

// Additional utility functions

// NewOptions creates Options with default values.
// Deprecated: Use claudesdk.NewOptions instead.
var NewOptions = claudesdk.NewOptions

// CreateSDKMcpServer creates an in-process MCP server.
// Deprecated: Use claudesdk.CreateSDKMcpServer instead.
var CreateSDKMcpServer = claudesdk.CreateSDKMcpServer

// NewTextContent creates new text content.
// Deprecated: Use claudesdk.NewTextContent instead.
var NewTextContent = claudesdk.NewTextContent

// NewImageContent creates new image content.
// Deprecated: Use claudesdk.NewImageContent instead.
var NewImageContent = claudesdk.NewImageContent

// Tool creates a new tool definition.
// Deprecated: Use claudesdk.Tool instead.
var Tool = claudesdk.Tool

// Hook utility functions
// Deprecated: Use claudesdk functions instead.
var NewPreToolUseOutput = claudesdk.NewPreToolUseOutput
var NewPostToolUseOutput = claudesdk.NewPostToolUseOutput
var NewStopOutput = claudesdk.NewStopOutput

// Hook option functions
// Deprecated: Use claudesdk functions instead.
var WithHook = claudesdk.WithHook
var WithHooks = claudesdk.WithHooks

// Additional variables  

// ErrNoMoreMessages indicates the message iterator has no more messages.
// Deprecated: Use claudesdk.ErrNoMoreMessages instead.
var ErrNoMoreMessages = claudesdk.ErrNoMoreMessages

// Version constants
// Deprecated: Use claudesdk.Version and claudesdk.BundledCLIVersion instead.
const (
	Version           = claudesdk.Version
	BundledCLIVersion = claudesdk.BundledCLIVersion
)
