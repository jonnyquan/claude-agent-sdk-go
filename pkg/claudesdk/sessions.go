package claudesdk

import (
	"github.com/jonnyquan/claude-agent-sdk-go/internal/sessions"
)

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
