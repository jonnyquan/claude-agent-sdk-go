package claudesdk

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/sessions"
)

// shadowed helpers so projectsDirForOptions stays one tight block.
var (
	filepathJoin   = filepath.Join
	osGetenv       = os.Getenv
	osUserHomeDir  = os.UserHomeDir
)

// InMemorySessionStore is an in-memory SessionStore implementation for
// testing and development. Stores entries in a map; not suitable for
// production — data is lost when the process exits.
type InMemorySessionStore = sessions.InMemorySessionStore

// NewInMemorySessionStore creates a new InMemorySessionStore.
var NewInMemorySessionStore = sessions.NewInMemorySessionStore

// ProjectKeyForDirectory derives the SessionStore ProjectKey for a directory.
// Defaults to the current working directory.
var ProjectKeyForDirectory = sessions.ProjectKeyForDirectory

// FilePathToSessionKey derives a SessionKey from an absolute transcript file path.
var FilePathToSessionKey = sessions.FilePathToSessionKey

// FoldSessionSummary folds a batch of appended entries into the running
// summary for a session. Stores call this from inside Append() to maintain
// an incremental SessionSummaryEntry sidecar.
var FoldSessionSummary = sessions.FoldSessionSummary

// SummaryEntryToSDKInfo converts a SessionSummaryEntry to SDKSessionInfo.
var SummaryEntryToSDKInfo = sessions.SummaryEntryToSDKInfo

// TranscriptMirrorBatcher accumulates transcript_mirror frames and flushes
// them to a SessionStore.
type TranscriptMirrorBatcher = sessions.TranscriptMirrorBatcher

// MirrorBatcherConfig configures TranscriptMirrorBatcher behavior.
type MirrorBatcherConfig = sessions.MirrorBatcherConfig

// MirrorErrorCallback is invoked when a flush fails after exhausting retries.
type MirrorErrorCallback = sessions.MirrorErrorCallback

// NewTranscriptMirrorBatcher constructs a TranscriptMirrorBatcher.
var NewTranscriptMirrorBatcher = sessions.NewTranscriptMirrorBatcher

// MaterializedResume is the result of materializing a SessionStore-backed
// resume into a temp config dir.
type MaterializedResume = sessions.MaterializedResume

// MaterializeResumeSession loads a session from store and writes it to a
// temp dir laid out like ~/.claude/.
var MaterializeResumeSession = sessions.MaterializeResumeSession

// ApplyMaterializedOptions returns a copy of options repointed at a
// materialized temp config dir.
var ApplyMaterializedOptions = sessions.ApplyMaterializedOptions

// ValidateSessionStoreOptions returns an error for invalid SessionStore option
// combinations. Call before subprocess spawn so misconfiguration fails fast.
var ValidateSessionStoreOptions = sessions.ValidateSessionStoreOptions

// ImportSessionToStore replays a local on-disk session transcript into a
// SessionStore. See sessions.ImportSessionToStore for details.
var ImportSessionToStore = sessions.ImportSessionToStore

// ImportSessionOptions configures ImportSessionToStore behavior.
type ImportSessionOptions = sessions.ImportSessionOptions

// RunSessionStoreConformance asserts the 14 SessionStore behavioral
// contracts. Call from a Go test with a factory that returns a fresh store
// for each contract.
//
// Mirrors Python SDK's claude_agent_sdk.testing.run_session_store_conformance.
var RunSessionStoreConformance = sessions.RunSessionStoreConformance

// ConformanceOptions configures RunSessionStoreConformance.
type ConformanceOptions = sessions.ConformanceOptions

// projectsDirForOptions returns the absolute projects directory the
// subprocess will write transcripts under — typically the materialized
// CLAUDE_CONFIG_DIR + "/projects", or $HOME/.claude/projects when no
// resume materialization happened.
func projectsDirForOptions(opts *Options, m *MaterializedResume) string {
	if m != nil {
		return filepathJoin(m.ConfigDir, "projects")
	}
	if opts != nil && opts.ExtraEnv != nil {
		if dir, ok := opts.ExtraEnv["CLAUDE_CONFIG_DIR"]; ok && dir != "" {
			return filepathJoin(dir, "projects")
		}
	}
	if dir := osGetenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepathJoin(dir, "projects")
	}
	if home, err := osUserHomeDir(); err == nil {
		return filepathJoin(home, ".claude", "projects")
	}
	return ""
}

// ListSessions returns stored Claude sessions, sorted by last modified descending.
func ListSessions(directory string, limit *int, offset int, includeWorktrees bool) []SDKSessionInfo {
	return sessions.ListSessions(directory, limit, offset, includeWorktrees)
}

// GetSessionInfo reads metadata for a single stored session.
func GetSessionInfo(sessionID string, directory string) *SDKSessionInfo {
	return sessions.GetSessionInfo(sessionID, directory)
}

// GetSessionMessages reconstructs the visible conversation chain for a session.
func GetSessionMessages(sessionID string, directory string, limit *int, offset int) []SessionMessage {
	return sessions.GetSessionMessages(sessionID, directory, limit, offset)
}

// RenameSession appends a custom-title entry to a stored session transcript.
func RenameSession(sessionID, title, directory string) error {
	return sessions.RenameSession(sessionID, title, directory)
}

// TagSession appends a tag entry to a stored session transcript.
func TagSession(sessionID string, tag *string, directory string) error {
	return sessions.TagSession(sessionID, tag, directory)
}

// DeleteSession removes a stored session transcript file.
func DeleteSession(sessionID, directory string) error {
	return sessions.DeleteSession(sessionID, directory)
}

// ForkSession forks a stored transcript into a new session with remapped UUIDs.
func ForkSession(sessionID, directory string, upToMessageID, title *string) (*ForkSessionResult, error) {
	return sessions.ForkSession(sessionID, directory, upToMessageID, title)
}

// ListSubagents lists subagent IDs for a given session.
func ListSubagents(sessionID, directory string) []string {
	return sessions.ListSubagents(sessionID, directory)
}

// GetSubagentMessages reads a subagent's conversation messages.
func GetSubagentMessages(sessionID, agentID, directory string, limit *int, offset int) []SessionMessage {
	return sessions.GetSubagentMessages(sessionID, agentID, directory, limit, offset)
}

// ---------------------------------------------------------------------------
// SessionStore-backed counterparts.
// ---------------------------------------------------------------------------

// ListSessionsFromStore lists sessions from a SessionStore.
func ListSessionsFromStore(ctx context.Context, store SessionStore, directory string, limit *int, offset int) ([]SDKSessionInfo, error) {
	return sessions.ListSessionsFromStore(ctx, store, directory, limit, offset)
}

// GetSessionInfoFromStore reads metadata for a single session from a SessionStore.
func GetSessionInfoFromStore(ctx context.Context, store SessionStore, sessionID, directory string) (*SDKSessionInfo, error) {
	return sessions.GetSessionInfoFromStore(ctx, store, sessionID, directory)
}

// GetSessionMessagesFromStore reads a session's conversation messages from a SessionStore.
func GetSessionMessagesFromStore(ctx context.Context, store SessionStore, sessionID, directory string, limit *int, offset int) ([]SessionMessage, error) {
	return sessions.GetSessionMessagesFromStore(ctx, store, sessionID, directory, limit, offset)
}

// ListSubagentsFromStore lists subagent IDs for a session from a SessionStore.
func ListSubagentsFromStore(ctx context.Context, store SessionStore, sessionID, directory string) ([]string, error) {
	return sessions.ListSubagentsFromStore(ctx, store, sessionID, directory)
}

// GetSubagentMessagesFromStore reads a subagent's conversation messages from a SessionStore.
func GetSubagentMessagesFromStore(ctx context.Context, store SessionStore, sessionID, agentID, directory string, limit *int, offset int) ([]SessionMessage, error) {
	return sessions.GetSubagentMessagesFromStore(ctx, store, sessionID, agentID, directory, limit, offset)
}

// RenameSessionViaStore appends a custom-title entry to a SessionStore.
func RenameSessionViaStore(ctx context.Context, store SessionStore, sessionID, title, directory string) error {
	return sessions.RenameSessionViaStore(ctx, store, sessionID, title, directory)
}

// TagSessionViaStore appends a tag entry to a SessionStore. Pass tag=nil to clear.
func TagSessionViaStore(ctx context.Context, store SessionStore, sessionID string, tag *string, directory string) error {
	return sessions.TagSessionViaStore(ctx, store, sessionID, tag, directory)
}

// DeleteSessionViaStore deletes a session from a SessionStore. No-op when
// the adapter doesn't implement Delete.
func DeleteSessionViaStore(ctx context.Context, store SessionStore, sessionID, directory string) error {
	return sessions.DeleteSessionViaStore(ctx, store, sessionID, directory)
}
