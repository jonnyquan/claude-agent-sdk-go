package claudesdk

import (
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// Options contains configuration for Claude Code CLI interactions.
type Options = shared.Options

// PermissionMode defines the permission handling mode.
type PermissionMode = shared.PermissionMode

// McpServerType defines the type of MCP server.
type McpServerType = shared.McpServerType

// McpServerConfig represents an MCP server configuration.
type McpServerConfig = shared.McpServerConfig

// McpStdioServerConfig represents a stdio MCP server configuration.
type McpStdioServerConfig = shared.McpStdioServerConfig

// McpSSEServerConfig represents an SSE MCP server configuration.
type McpSSEServerConfig = shared.McpSSEServerConfig

// McpHTTPServerConfig represents an HTTP MCP server configuration.
type McpHTTPServerConfig = shared.McpHTTPServerConfig

// McpSdkServerConfig represents an SDK MCP server configuration.
type McpSdkServerConfig = shared.McpSdkServerConfig

// SdkBeta represents SDK beta feature identifiers.
type SdkBeta = shared.SdkBeta

// ToolsPreset represents a preset configuration for available tools.
type ToolsPreset = shared.ToolsPreset

// ToolsOption can be either a list of tool names or a preset.
type ToolsOption = shared.ToolsOption

// PluginType defines the type of plugin.
type PluginType = shared.PluginType

// PluginConfig represents plugin configuration.
type PluginConfig = shared.PluginConfig

// Sandbox configuration types
type SandboxSettings = shared.SandboxSettings
type SandboxNetworkConfig = shared.SandboxNetworkConfig
type SandboxIgnoreViolations = shared.SandboxIgnoreViolations

// Re-export constants
const (
	PermissionModeDefault           = shared.PermissionModeDefault
	PermissionModeAcceptEdits       = shared.PermissionModeAcceptEdits
	PermissionModePlan              = shared.PermissionModePlan
	PermissionModeBypassPermissions = shared.PermissionModeBypassPermissions
	McpServerTypeStdio              = shared.McpServerTypeStdio
	McpServerTypeSSE                = shared.McpServerTypeSSE
	McpServerTypeHTTP               = shared.McpServerTypeHTTP
	McpServerTypeSDK                = shared.McpServerTypeSDK
	PluginTypeLocal                 = shared.PluginTypeLocal
	// Beta features
	SdkBetaContext1M = shared.SdkBetaContext1M
)

// Option configures Options using the functional options pattern.
type Option func(*Options)

// WithTools sets the base set of available tools.
// Pass nil for default behavior, empty slice to disable all tools,
// slice with tool names to specify which tools are available,
// or ToolsPreset for preset configuration (e.g., claude_code).
//
// Examples:
//
//	WithTools([]string{"Read", "Edit", "Bash"})  // Specific tools only
//	WithTools([]string{})                        // Disable all built-in tools
//	WithTools(ToolsPreset{Type: "preset", Preset: "claude_code"}) // Use preset
func WithTools(tools ToolsOption) Option {
	return func(o *Options) {
		o.Tools = tools
	}
}

// WithAllowedTools sets the allowed tools list.
func WithAllowedTools(tools ...string) Option {
	return func(o *Options) {
		o.AllowedTools = tools
	}
}

// WithDisallowedTools sets the disallowed tools list.
func WithDisallowedTools(tools ...string) Option {
	return func(o *Options) {
		o.DisallowedTools = tools
	}
}

// WithSystemPrompt sets the system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(o *Options) {
		o.SystemPrompt = &prompt
	}
}

// WithAppendSystemPrompt sets the append system prompt.
func WithAppendSystemPrompt(prompt string) Option {
	return func(o *Options) {
		o.AppendSystemPrompt = &prompt
	}
}

// WithModel sets the model to use.
func WithModel(model string) Option {
	return func(o *Options) {
		o.Model = &model
	}
}

// WithBetas enables Anthropic API beta features.
// See https://docs.anthropic.com/en/api/beta-headers for available betas.
//
// Example:
//
//	WithBetas(SdkBetaContext1M) // Enable extended context window
func WithBetas(betas ...SdkBeta) Option {
	return func(o *Options) {
		o.Betas = betas
	}
}

// WithMaxThinkingTokens sets the maximum thinking tokens.
func WithMaxThinkingTokens(tokens int) Option {
	return func(o *Options) {
		o.MaxThinkingTokens = tokens
	}
}

// WithPermissionMode sets the permission mode.
func WithPermissionMode(mode PermissionMode) Option {
	return func(o *Options) {
		o.PermissionMode = &mode
	}
}

// WithPermissionPromptToolName sets the permission prompt tool name.
func WithPermissionPromptToolName(toolName string) Option {
	return func(o *Options) {
		o.PermissionPromptToolName = &toolName
	}
}

// WithContinueConversation enables conversation continuation.
func WithContinueConversation(continueConversation bool) Option {
	return func(o *Options) {
		o.ContinueConversation = continueConversation
	}
}

// WithResume sets the session ID to resume.
func WithResume(sessionID string) Option {
	return func(o *Options) {
		o.Resume = &sessionID
	}
}

// WithCwd sets the working directory.
func WithCwd(cwd string) Option {
	return func(o *Options) {
		o.Cwd = &cwd
	}
}

// WithAddDirs adds directories to the context.
func WithAddDirs(dirs ...string) Option {
	return func(o *Options) {
		o.AddDirs = dirs
	}
}

// WithIncludePartialMessages controls whether partial assistant messages are emitted.
func WithIncludePartialMessages(include bool) Option {
	return func(o *Options) {
		o.IncludePartialMessages = include
	}
}

// WithForkSession controls whether resumed sessions fork to a new session ID.
func WithForkSession(fork bool) Option {
	return func(o *Options) {
		o.ForkSession = fork
	}
}

// WithSettingSources sets the configuration sources the CLI should load.
func WithSettingSources(sources ...string) Option {
	return func(o *Options) {
		if len(sources) == 0 {
			o.SettingSources = nil
			return
		}
		copied := make([]string, len(sources))
		copy(copied, sources)
		o.SettingSources = copied
	}
}

// WithAgents configures custom agents available to the CLI.
func WithAgents(agents map[string]AgentDefinition) Option {
	return func(o *Options) {
		if agents == nil {
			o.Agents = nil
			return
		}
		copied := make(map[string]AgentDefinition, len(agents))
		for name, def := range agents {
			copied[name] = def
		}
		o.Agents = copied
	}
}

// WithPlugins configures custom plugins for the CLI.
func WithPlugins(plugins ...PluginConfig) Option {
	return func(o *Options) {
		if len(plugins) == 0 {
			o.Plugins = nil
			return
		}
		copied := make([]PluginConfig, len(plugins))
		copy(copied, plugins)
		o.Plugins = copied
	}
}

// WithOutputFormat sets the output format for structured outputs.
// Example: WithOutputFormat(map[string]interface{}{"type": "json_schema", "schema": mySchema})
func WithOutputFormat(format map[string]interface{}) Option {
	return func(o *Options) {
		o.OutputFormat = format
	}
}

// WithUser sets the user under which the CLI should run (platform dependent).
func WithUser(user string) Option {
	return func(o *Options) {
		if user == "" {
			o.User = nil
			return
		}
		o.User = &user
	}
}

// WithMaxBufferSize configures the maximum buffer size for CLI stdout.
// A non-positive size clears the override.
func WithMaxBufferSize(size int) Option {
	return func(o *Options) {
		if size <= 0 {
			o.MaxBufferSize = nil
			return
		}
		sizeCopy := size
		o.MaxBufferSize = &sizeCopy
	}
}

// WithMcpServers sets the MCP server configurations.
func WithMcpServers(servers map[string]McpServerConfig) Option {
	return func(o *Options) {
		o.McpServers = servers
	}
}

// WithMaxTurns sets the maximum number of conversation turns.
func WithMaxTurns(turns int) Option {
	return func(o *Options) {
		o.MaxTurns = turns
	}
}

// WithMaxBudgetUSD sets the maximum budget in USD for API costs.
// When the cumulative cost exceeds this budget, the CLI will stop execution
// and return an error_max_budget_usd result.
//
// Note: The actual cost may slightly exceed the budget by up to one API call's worth,
// as budget checking happens after each API call completes.
func WithMaxBudgetUSD(budget float64) Option {
	return func(o *Options) {
		o.MaxBudgetUSD = &budget
	}
}

// WithSettings sets the settings file path or JSON string.
func WithSettings(settings string) Option {
	return func(o *Options) {
		o.Settings = &settings
	}
}

// WithSandbox configures sandbox settings for bash command isolation.
// When sandbox is provided, it will be merged into the settings passed to CLI.
func WithSandbox(sandbox *SandboxSettings) Option {
	return func(o *Options) {
		o.Sandbox = sandbox
	}
}

// WithEnableFileCheckpointing enables file checkpointing to track file changes during the session.
// When enabled, files can be rewound to their state at any user message using Client.RewindFiles().
func WithEnableFileCheckpointing(enable bool) Option {
	return func(o *Options) {
		o.EnableFileCheckpointing = enable
	}
}

// WithExtraArgs sets arbitrary CLI flags via ExtraArgs.
func WithExtraArgs(args map[string]*string) Option {
	return func(o *Options) {
		o.ExtraArgs = args
	}
}

// WithCLIPath sets a custom CLI path.
func WithCLIPath(path string) Option {
	return func(o *Options) {
		o.CLIPath = &path
	}
}

// WithEnv sets environment variables for the subprocess.
// Multiple calls to WithEnv or WithEnvVar merge the values.
// Later calls override earlier ones for the same key.
func WithEnv(env map[string]string) Option {
	return func(o *Options) {
		if o.ExtraEnv == nil {
			o.ExtraEnv = make(map[string]string)
		}
		// Merge pattern - idiomatic Go
		for k, v := range env {
			o.ExtraEnv[k] = v
		}
	}
}

// WithEnvVar sets a single environment variable for the subprocess.
// This is a convenience method for setting individual variables.
func WithEnvVar(key, value string) Option {
	return func(o *Options) {
		if o.ExtraEnv == nil {
			o.ExtraEnv = make(map[string]string)
		}
		o.ExtraEnv[key] = value
	}
}

// WithHooks configures hooks for intercepting SDK behavior.
// Hooks are organized by event type (e.g., "PreToolUse", "PostToolUse").
// Each event can have multiple matchers with their callback functions.
func WithHooks(hooks map[string][]HookMatcher) Option {
	return func(o *Options) {
		if o.Hooks == nil {
			o.Hooks = make(map[string][]any)
		}
		// Convert HookMatcher to any for storage in Options
		for event, matchers := range hooks {
			anyMatchers := make([]any, len(matchers))
			for i, m := range matchers {
				anyMatchers[i] = m
			}
			o.Hooks[event] = anyMatchers
		}
	}
}

// WithHook adds a single hook matcher for a specific event.
// This is a convenience method for adding individual hooks.
func WithHook(event HookEvent, matcher HookMatcher) Option {
	return func(o *Options) {
		if o.Hooks == nil {
			o.Hooks = make(map[string][]any)
		}
		eventKey := string(event)
		o.Hooks[eventKey] = append(o.Hooks[eventKey], matcher)
	}
}

const customTransportMarker = "custom_transport"

// WithTransport sets a custom transport for testing.
// Since Transport is not part of Options struct, this is handled in client creation.
func WithTransport(_ Transport) Option {
	return func(o *Options) {
		// This will be handled in client implementation
		// For now, we'll use a special marker in ExtraArgs
		if o.ExtraArgs == nil {
			o.ExtraArgs = make(map[string]*string)
		}
		marker := customTransportMarker
		o.ExtraArgs["__transport_marker__"] = &marker
	}
}

// NewOptions creates Options with default values using functional options pattern.
func NewOptions(opts ...Option) *Options {
	// Create options with defaults from shared package
	options := shared.NewOptions()

	// Apply functional options
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// IntPtr returns a pointer to the given int value.
// Useful for optional integer fields.
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr returns a pointer to the given float64 value.
// Useful for optional timeout fields in hooks.
func Float64Ptr(f float64) *float64 {
	return &f
}
