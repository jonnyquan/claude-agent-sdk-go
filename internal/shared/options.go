package shared

import "fmt"

const (
	// DefaultMaxThinkingTokens is the default maximum number of thinking tokens.
	DefaultMaxThinkingTokens = 8000
)

// PermissionMode represents the different permission handling modes.
type PermissionMode string

const (
	// PermissionModeDefault is the standard permission handling mode.
	PermissionModeDefault PermissionMode = "default"
	// PermissionModeAcceptEdits automatically accepts all edit permissions.
	PermissionModeAcceptEdits PermissionMode = "acceptEdits"
	// PermissionModePlan enables plan mode for task execution.
	PermissionModePlan PermissionMode = "plan"
	// PermissionModeBypassPermissions bypasses all permission checks.
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// SdkBeta represents SDK beta feature identifiers.
// See https://docs.anthropic.com/en/api/beta-headers
type SdkBeta string

const (
	// SdkBetaContext1M enables extended context window (1M tokens).
	SdkBetaContext1M SdkBeta = "context-1m-2025-08-07"
)

// ToolsPreset represents a preset configuration for available tools.
type ToolsPreset struct {
	Type   string `json:"type"`   // Always "preset"
	Preset string `json:"preset"` // Preset name, e.g., "claude_code"
}

// ToolsOption can be either a list of tool names or a preset.
// Use nil for default behavior, empty slice to disable all tools,
// slice with tool names to specify which tools are available,
// or ToolsPreset for preset configuration.
type ToolsOption interface{}

// Options configures the Claude Code SDK behavior.
type Options struct {
	// Tool Control
	Tools           ToolsOption `json:"tools,omitempty"` // Base set of available tools
	AllowedTools    []string    `json:"allowed_tools,omitempty"`
	DisallowedTools []string    `json:"disallowed_tools,omitempty"`

	// System Prompts & Model
	SystemPrompt       *string   `json:"system_prompt,omitempty"`
	AppendSystemPrompt *string   `json:"append_system_prompt,omitempty"`
	Model              *string   `json:"model,omitempty"`
	FallbackModel      *string   `json:"fallback_model,omitempty"`
	MaxThinkingTokens  int       `json:"max_thinking_tokens,omitempty"`
	Betas              []SdkBeta `json:"betas,omitempty"` // Beta features to enable

	// Permission & Safety System
	PermissionMode           *PermissionMode `json:"permission_mode,omitempty"`
	PermissionPromptToolName *string         `json:"permission_prompt_tool_name,omitempty"`

	// Session & State Management
	ContinueConversation   bool     `json:"continue_conversation,omitempty"`
	Resume                 *string  `json:"resume,omitempty"`
	MaxTurns               int      `json:"max_turns,omitempty"`
	MaxBudgetUSD           *float64 `json:"max_budget_usd,omitempty"` // Budget limit in USD for API costs
	Settings               *string  `json:"settings,omitempty"`
	IncludePartialMessages bool    `json:"include_partial_messages,omitempty"`
	ForkSession            bool    `json:"fork_session,omitempty"`

	// File System & Context
	Cwd            *string  `json:"cwd,omitempty"`
	AddDirs        []string `json:"add_dirs,omitempty"`
	User           *string  `json:"user,omitempty"`
	SettingSources []string `json:"setting_sources,omitempty"`

	// MCP Integration
	McpServers    map[string]McpServerConfig `json:"mcp_servers,omitempty"`
	MaxBufferSize *int                       `json:"max_buffer_size,omitempty"`

	// Hooks for intercepting and controlling SDK behavior
	// Key: HookEvent type (e.g., "PreToolUse", "PostToolUse")
	// Value: List of hook matchers with their callbacks
	Hooks map[string][]any `json:"hooks,omitempty"`

	// Extensibility
	ExtraArgs map[string]*string `json:"extra_args,omitempty"`

	// ExtraEnv specifies additional environment variables for the subprocess.
	// These are merged with the system environment variables.
	ExtraEnv map[string]string `json:"extra_env,omitempty"`

	// CLI Path (for testing and custom installations)
	CLIPath *string `json:"cli_path,omitempty"`

	// Agents configuration for custom workflows.
	Agents map[string]AgentDefinition `json:"agents,omitempty"`

	// Plugins configuration for custom plugins.
	Plugins []PluginConfig `json:"plugins,omitempty"`

	// OutputFormat for structured outputs (JSON Schema validation)
	// Example: map[string]interface{}{"type": "json_schema", "schema": {...}}
	OutputFormat map[string]interface{} `json:"output_format,omitempty"`

	// Sandbox configuration for bash command isolation.
	// Filesystem and network restrictions are derived from permission rules (Read/Edit/WebFetch),
	// not from these sandbox settings.
	Sandbox *SandboxSettings `json:"sandbox,omitempty"`

	// EnableFileCheckpointing enables file checkpointing to track file changes during the session.
	// When enabled, files can be rewound to their state at any user message using Client.RewindFiles().
	EnableFileCheckpointing bool `json:"enable_file_checkpointing,omitempty"`
}

// AgentDefinition configures a named agent available to the CLI.
type AgentDefinition struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       *string  `json:"model,omitempty"`
}

// PluginType represents the type of plugin.
type PluginType string

const (
	// PluginTypeLocal represents a local filesystem plugin.
	PluginTypeLocal PluginType = "local"
)

// PluginConfig represents plugin configuration.
type PluginConfig struct {
	Type PluginType `json:"type"`
	Path string     `json:"path"`
}

// SandboxNetworkConfig configures network settings for sandbox.
type SandboxNetworkConfig struct {
	// AllowUnixSockets specifies Unix socket paths accessible in sandbox (e.g., SSH agents).
	AllowUnixSockets []string `json:"allowUnixSockets,omitempty"`
	// AllowAllUnixSockets allows all Unix sockets (less secure).
	AllowAllUnixSockets bool `json:"allowAllUnixSockets,omitempty"`
	// AllowLocalBinding allows binding to localhost ports (macOS only).
	AllowLocalBinding bool `json:"allowLocalBinding,omitempty"`
	// HTTPProxyPort is the HTTP proxy port if bringing your own proxy.
	HTTPProxyPort int `json:"httpProxyPort,omitempty"`
	// SOCKSProxyPort is the SOCKS5 proxy port if bringing your own proxy.
	SOCKSProxyPort int `json:"socksProxyPort,omitempty"`
}

// SandboxIgnoreViolations specifies violations to ignore in sandbox.
type SandboxIgnoreViolations struct {
	// File paths for which violations should be ignored.
	File []string `json:"file,omitempty"`
	// Network hosts for which violations should be ignored.
	Network []string `json:"network,omitempty"`
}

// SandboxSettings configures sandbox behavior for bash command isolation.
//
// This controls how Claude Code sandboxes bash commands for filesystem
// and network isolation.
//
// Important: Filesystem and network restrictions are configured via permission
// rules, not via these sandbox settings:
//   - Filesystem read restrictions: Use Read deny rules
//   - Filesystem write restrictions: Use Edit allow/deny rules
//   - Network restrictions: Use WebFetch allow/deny rules
type SandboxSettings struct {
	// Enabled enables bash sandboxing (macOS/Linux only). Default: false
	Enabled bool `json:"enabled,omitempty"`
	// AutoAllowBashIfSandboxed auto-approves bash commands when sandboxed. Default: true
	AutoAllowBashIfSandboxed bool `json:"autoAllowBashIfSandboxed,omitempty"`
	// ExcludedCommands lists commands that should run outside the sandbox (e.g., ["git", "docker"])
	ExcludedCommands []string `json:"excludedCommands,omitempty"`
	// AllowUnsandboxedCommands allows commands to bypass sandbox via dangerouslyDisableSandbox.
	// When false, all commands must run sandboxed (or be in excludedCommands). Default: true
	AllowUnsandboxedCommands bool `json:"allowUnsandboxedCommands,omitempty"`
	// Network configures network settings for sandbox.
	Network *SandboxNetworkConfig `json:"network,omitempty"`
	// IgnoreViolations specifies violations to ignore.
	IgnoreViolations *SandboxIgnoreViolations `json:"ignoreViolations,omitempty"`
	// EnableWeakerNestedSandbox enables weaker sandbox for unprivileged Docker environments
	// (Linux only). Reduces security. Default: false
	EnableWeakerNestedSandbox bool `json:"enableWeakerNestedSandbox,omitempty"`
}

// McpServerType represents the type of MCP server.
type McpServerType string

const (
	// McpServerTypeStdio represents a stdio-based MCP server.
	McpServerTypeStdio McpServerType = "stdio"
	// McpServerTypeSSE represents a Server-Sent Events MCP server.
	McpServerTypeSSE McpServerType = "sse"
	// McpServerTypeHTTP represents an HTTP-based MCP server.
	McpServerTypeHTTP McpServerType = "http"
	// McpServerTypeSDK represents an in-process SDK MCP server.
	McpServerTypeSDK McpServerType = "sdk"
)

// McpServerConfig represents MCP server configuration.
type McpServerConfig interface {
	GetType() McpServerType
}

// McpStdioServerConfig configures an MCP stdio server.
type McpStdioServerConfig struct {
	Type    McpServerType     `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// GetType returns the server type for McpStdioServerConfig.
func (c *McpStdioServerConfig) GetType() McpServerType {
	return McpServerTypeStdio
}

// McpSSEServerConfig configures an MCP Server-Sent Events server.
type McpSSEServerConfig struct {
	Type    McpServerType     `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// GetType returns the server type for McpSSEServerConfig.
func (c *McpSSEServerConfig) GetType() McpServerType {
	return McpServerTypeSSE
}

// McpHTTPServerConfig configures an MCP HTTP server.
type McpHTTPServerConfig struct {
	Type    McpServerType     `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// GetType returns the server type for McpHTTPServerConfig.
func (c *McpHTTPServerConfig) GetType() McpServerType {
	return McpServerTypeHTTP
}

// McpSDKServer represents an in-process MCP server instance.
// This is an interface to avoid circular dependencies.
type McpSDKServer interface {
	Name() string
	Version() string
	HandleJSONRPC(ctx interface{}, request []byte) ([]byte, error)
}

// McpSdkServerConfig configures an in-process SDK MCP server.
type McpSdkServerConfig struct {
	Type     McpServerType `json:"type"`
	Name     string        `json:"name"`
	Instance McpSDKServer  `json:"-"` // Not serialized, internal use only
}

// GetType returns the server type for McpSdkServerConfig.
func (c *McpSdkServerConfig) GetType() McpServerType {
	return McpServerTypeSDK
}

// Validate checks the options for valid values and constraints.
func (o *Options) Validate() error {
	// Validate MaxThinkingTokens
	if o.MaxThinkingTokens < 0 {
		return fmt.Errorf("MaxThinkingTokens must be non-negative, got %d", o.MaxThinkingTokens)
	}

	// Validate MaxTurns
	if o.MaxTurns < 0 {
		return fmt.Errorf("MaxTurns must be non-negative, got %d", o.MaxTurns)
	}

	// Validate tool conflicts (same tool in both allowed and disallowed)
	allowedSet := make(map[string]bool)
	for _, tool := range o.AllowedTools {
		allowedSet[tool] = true
	}

	for _, tool := range o.DisallowedTools {
		if allowedSet[tool] {
			return fmt.Errorf("tool '%s' cannot be in both AllowedTools and DisallowedTools", tool)
		}
	}

	if o.MaxBufferSize != nil && *o.MaxBufferSize <= 0 {
		return fmt.Errorf("MaxBufferSize must be positive, got %d", *o.MaxBufferSize)
	}

	// Validate plugins
	for i, plugin := range o.Plugins {
		if plugin.Type != PluginTypeLocal {
			return fmt.Errorf("plugin[%d]: unsupported plugin type: %s", i, plugin.Type)
		}
		if plugin.Path == "" {
			return fmt.Errorf("plugin[%d]: path cannot be empty", i)
		}
	}

	return nil
}

// NewOptions creates Options with default values.
func NewOptions() *Options {
	return &Options{
		AllowedTools:      []string{},
		DisallowedTools:   []string{},
		MaxThinkingTokens: DefaultMaxThinkingTokens,
		AddDirs:           []string{},
		McpServers:        make(map[string]McpServerConfig),
		Agents:            make(map[string]AgentDefinition),
		Plugins:           []PluginConfig{},
		Hooks:             make(map[string][]any),
		ExtraArgs:         make(map[string]*string),
		ExtraEnv:          make(map[string]string),
	}
}
