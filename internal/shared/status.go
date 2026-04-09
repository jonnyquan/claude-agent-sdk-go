package shared

// McpToolAnnotations represents tool-level annotations returned by MCP status APIs.
type McpToolAnnotations struct {
	ReadOnly      *bool `json:"readOnly,omitempty"`
	Destructive   *bool `json:"destructive,omitempty"`
	OpenWorld     *bool `json:"openWorld,omitempty"`
	ReadOnlyHint  *bool `json:"readOnlyHint,omitempty"`
	OpenWorldHint *bool `json:"openWorldHint,omitempty"`
}

// McpToolInfo describes an MCP tool advertised by a server.
type McpToolInfo struct {
	Name        string              `json:"name"`
	Description *string             `json:"description,omitempty"`
	Annotations *McpToolAnnotations `json:"annotations,omitempty"`
}

// McpServerInfo describes a connected MCP server.
type McpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// McpServerConnectionStatus is the connection status string for an MCP server.
type McpServerConnectionStatus string

const (
	McpServerStatusConnected McpServerConnectionStatus = "connected"
	McpServerStatusFailed    McpServerConnectionStatus = "failed"
	McpServerStatusNeedsAuth McpServerConnectionStatus = "needs-auth"
	McpServerStatusPending   McpServerConnectionStatus = "pending"
	McpServerStatusDisabled  McpServerConnectionStatus = "disabled"
)

// McpServerStatusConfig is the serialized server config shape returned in MCP status responses.
// It is a superset of the stdio/sse/http/sdk/claudeai-proxy variants.
type McpServerStatusConfig struct {
	Type    string            `json:"type"`
	Command *string           `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     *string           `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Name    *string           `json:"name,omitempty"`
	ID      *string           `json:"id,omitempty"`
}

// McpServerStatus describes the live status of a configured MCP server.
type McpServerStatus struct {
	Name       string                    `json:"name"`
	Status     McpServerConnectionStatus `json:"status"`
	ServerInfo *McpServerInfo            `json:"serverInfo,omitempty"`
	Error      *string                   `json:"error,omitempty"`
	Config     *McpServerStatusConfig    `json:"config,omitempty"`
	Scope      *string                   `json:"scope,omitempty"`
	Tools      []McpToolInfo             `json:"tools,omitempty"`
}

// McpStatusResponse wraps server statuses returned by get_mcp_status.
type McpStatusResponse struct {
	McpServers []McpServerStatus `json:"mcpServers"`
}

// ContextUsageCategory is a single usage bucket from get_context_usage.
type ContextUsageCategory struct {
	Name       string `json:"name"`
	Tokens     int    `json:"tokens"`
	Color      string `json:"color"`
	IsDeferred *bool  `json:"isDeferred,omitempty"`
}

// ContextUsageMemoryFile describes a loaded memory file and its token cost.
type ContextUsageMemoryFile struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Tokens int    `json:"tokens"`
}

// ContextUsageMcpTool describes an MCP tool's contribution to context usage.
type ContextUsageMcpTool struct {
	Name       string `json:"name"`
	ServerName string `json:"serverName"`
	Tokens     int    `json:"tokens"`
	IsLoaded   *bool  `json:"isLoaded,omitempty"`
}

// ContextUsageAgent describes an agent definition's contribution to context usage.
type ContextUsageAgent struct {
	AgentType string `json:"agentType"`
	Source    string `json:"source"`
	Tokens    int    `json:"tokens"`
}

// ContextUsageNamedTokens is a generic named token-count entry used by several sections.
type ContextUsageNamedTokens struct {
	Name   string `json:"name"`
	Tokens int    `json:"tokens"`
}

// ContextUsageResponse describes the current context window usage breakdown.
type ContextUsageResponse struct {
	Categories           []ContextUsageCategory    `json:"categories"`
	TotalTokens          int                       `json:"totalTokens"`
	MaxTokens            int                       `json:"maxTokens"`
	RawMaxTokens         int                       `json:"rawMaxTokens"`
	Percentage           float64                   `json:"percentage"`
	Model                string                    `json:"model"`
	IsAutoCompactEnabled bool                      `json:"isAutoCompactEnabled"`
	MemoryFiles          []ContextUsageMemoryFile  `json:"memoryFiles"`
	McpTools             []ContextUsageMcpTool     `json:"mcpTools"`
	Agents               []ContextUsageAgent       `json:"agents"`
	GridRows             [][]map[string]any        `json:"gridRows"`
	AutoCompactThreshold *int                      `json:"autoCompactThreshold,omitempty"`
	DeferredBuiltinTools []ContextUsageNamedTokens `json:"deferredBuiltinTools,omitempty"`
	SystemTools          []ContextUsageNamedTokens `json:"systemTools,omitempty"`
	SystemPromptSections []ContextUsageNamedTokens `json:"systemPromptSections,omitempty"`
	SlashCommands        map[string]any            `json:"slashCommands,omitempty"`
	Skills               map[string]any            `json:"skills,omitempty"`
	MessageBreakdown     map[string]any            `json:"messageBreakdown,omitempty"`
	APIUsage             map[string]any            `json:"apiUsage,omitempty"`
}
