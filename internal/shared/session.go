package shared

// SDKSessionInfo contains lightweight metadata for a stored session transcript.
type SDKSessionInfo struct {
	SessionID    string  `json:"session_id"`
	Summary      string  `json:"summary"`
	LastModified int64   `json:"last_modified"`
	FileSize     *int64  `json:"file_size,omitempty"`
	CustomTitle  *string `json:"custom_title,omitempty"`
	FirstPrompt  *string `json:"first_prompt,omitempty"`
	GitBranch    *string `json:"git_branch,omitempty"`
	Cwd          *string `json:"cwd,omitempty"`
	Tag          *string `json:"tag,omitempty"`
	CreatedAt    *int64  `json:"created_at,omitempty"`
}

// SessionMessage represents a top-level user or assistant message from a session transcript.
type SessionMessage struct {
	Type            string         `json:"type"`
	UUID            string         `json:"uuid"`
	SessionID       string         `json:"session_id"`
	Message         map[string]any `json:"message"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
}

// ForkSessionResult is returned from fork_session operations.
type ForkSessionResult struct {
	SessionID string `json:"session_id"`
}
