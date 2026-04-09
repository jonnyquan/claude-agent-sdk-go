package claudesdk

import (
	"context"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// Message represents any message type in the conversation.
type Message = shared.Message

// ContentBlock represents a content block within a message.
type ContentBlock = shared.ContentBlock

// UserMessage represents a message from the user.
type UserMessage = shared.UserMessage

// AssistantMessage represents a message from the assistant.
type AssistantMessage = shared.AssistantMessage

// SystemMessage represents a system prompt message.
type SystemMessage = shared.SystemMessage
type TaskStartedMessage = shared.TaskStartedMessage
type TaskProgressMessage = shared.TaskProgressMessage
type TaskNotificationMessage = shared.TaskNotificationMessage
type TaskNotificationStatus = shared.TaskNotificationStatus
type TaskUsage = shared.TaskUsage

// ResultMessage represents a result or status message.
type ResultMessage = shared.ResultMessage

// TextBlock represents a text content block.
type TextBlock = shared.TextBlock

// ThinkingBlock represents a thinking content block.
type ThinkingBlock = shared.ThinkingBlock

// ToolUseBlock represents a tool usage content block.
type ToolUseBlock = shared.ToolUseBlock

// ToolResultBlock represents a tool result content block.
type ToolResultBlock = shared.ToolResultBlock

// StreamEvent represents a stream event for partial message updates.
type StreamEvent = shared.StreamEvent
type RateLimitStatus = shared.RateLimitStatus
type RateLimitType = shared.RateLimitType
type RateLimitInfo = shared.RateLimitInfo
type RateLimitEvent = shared.RateLimitEvent
type McpToolAnnotations = shared.McpToolAnnotations
type McpToolInfo = shared.McpToolInfo
type McpServerInfo = shared.McpServerInfo
type McpServerConnectionStatus = shared.McpServerConnectionStatus
type McpServerStatusConfig = shared.McpServerStatusConfig
type McpServerStatus = shared.McpServerStatus
type McpStatusResponse = shared.McpStatusResponse
type ContextUsageCategory = shared.ContextUsageCategory
type ContextUsageMemoryFile = shared.ContextUsageMemoryFile
type ContextUsageMcpTool = shared.ContextUsageMcpTool
type ContextUsageAgent = shared.ContextUsageAgent
type ContextUsageNamedTokens = shared.ContextUsageNamedTokens
type ContextUsageResponse = shared.ContextUsageResponse
type SDKSessionInfo = shared.SDKSessionInfo
type SessionMessage = shared.SessionMessage
type ForkSessionResult = shared.ForkSessionResult

// Note: ImageBlock removed - not part of Python SDK ContentBlock types

// AgentDefinition represents a custom agent configuration.
type AgentDefinition = shared.AgentDefinition

// StreamMessage represents a message in the streaming protocol.
type StreamMessage = shared.StreamMessage

// MessageIterator provides iteration over messages.
type MessageIterator = shared.MessageIterator

// Re-export message type constants
const (
	MessageTypeUser                 = shared.MessageTypeUser
	MessageTypeAssistant            = shared.MessageTypeAssistant
	MessageTypeSystem               = shared.MessageTypeSystem
	MessageTypeResult               = shared.MessageTypeResult
	MessageTypeStreamEvent          = shared.MessageTypeStreamEvent
	MessageTypeRateLimitEvent       = shared.MessageTypeRateLimitEvent
	McpServerStatusConnected        = shared.McpServerStatusConnected
	McpServerStatusFailed           = shared.McpServerStatusFailed
	McpServerStatusNeedsAuth        = shared.McpServerStatusNeedsAuth
	McpServerStatusPending          = shared.McpServerStatusPending
	McpServerStatusDisabled         = shared.McpServerStatusDisabled
	TaskNotificationStatusCompleted = shared.TaskNotificationStatusCompleted
	TaskNotificationStatusFailed    = shared.TaskNotificationStatusFailed
	TaskNotificationStatusStopped   = shared.TaskNotificationStatusStopped
	RateLimitStatusAllowed          = shared.RateLimitStatusAllowed
	RateLimitStatusAllowedWarning   = shared.RateLimitStatusAllowedWarning
	RateLimitStatusRejected         = shared.RateLimitStatusRejected
	RateLimitTypeFiveHour           = shared.RateLimitTypeFiveHour
	RateLimitTypeSevenDay           = shared.RateLimitTypeSevenDay
	RateLimitTypeSevenDayOpus       = shared.RateLimitTypeSevenDayOpus
	RateLimitTypeSevenDaySonnet     = shared.RateLimitTypeSevenDaySonnet
	RateLimitTypeOverage            = shared.RateLimitTypeOverage
)

// Re-export content block type constants
const (
	ContentBlockTypeText       = shared.ContentBlockTypeText
	ContentBlockTypeThinking   = shared.ContentBlockTypeThinking
	ContentBlockTypeToolUse    = shared.ContentBlockTypeToolUse
	ContentBlockTypeToolResult = shared.ContentBlockTypeToolResult
	// Note: ContentBlockTypeImage removed - not part of Python SDK
)

// AssistantMessageError represents error types for assistant messages
type AssistantMessageError = shared.AssistantMessageError

// Re-export assistant message error constants
const (
	AssistantMessageErrorAuthenticationFailed = shared.AssistantMessageErrorAuthenticationFailed
	AssistantMessageErrorBillingError         = shared.AssistantMessageErrorBillingError
	AssistantMessageErrorRateLimit            = shared.AssistantMessageErrorRateLimit
	AssistantMessageErrorInvalidRequest       = shared.AssistantMessageErrorInvalidRequest
	AssistantMessageErrorServerError          = shared.AssistantMessageErrorServerError
	AssistantMessageErrorUnknown              = shared.AssistantMessageErrorUnknown
)

// Transport abstracts the communication layer with Claude Code CLI.
// This interface stays in main package because it's used by client code.
type Transport interface {
	Connect(ctx context.Context) error
	SendMessage(ctx context.Context, message StreamMessage) error
	ReceiveMessages(ctx context.Context) (<-chan Message, <-chan error)
	Interrupt(ctx context.Context) error
	Close() error
	RewindFiles(ctx context.Context, userMessageID string) error
	GetMCPStatus(ctx context.Context) (map[string]any, error)
	GetContextUsage(ctx context.Context) (map[string]any, error)
	ReconnectMCPServer(ctx context.Context, serverName string) error
	ToggleMCPServer(ctx context.Context, serverName string, enabled bool) error
	StopTask(ctx context.Context, taskID string) error
	SetPermissionMode(ctx context.Context, mode string) error
	SetModel(ctx context.Context, model *string) error
	GetServerInfo() map[string]any
}
