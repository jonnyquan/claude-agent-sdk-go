package sessions

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// ProjectKeyForDirectory derives the SessionStore ProjectKey for a directory.
//
// Defaults to the current working directory. Uses the same realpath + NFC
// normalization + djb2-hashed sanitization the CLI uses for project directory
// names, so keys match between local-disk transcripts and store-mirrored
// transcripts even on filesystems that decompose Unicode (macOS HFS+).
func ProjectKeyForDirectory(directory string) string {
	if directory == "" {
		directory = "."
	}
	abs := canonicalizePath(directory)
	return sanitizePath(abs)
}

// FilePathToSessionKey derives a SessionKey from an absolute transcript file path.
//
// Main transcripts: <projectsDir>/<projectKey>/<sessionID>.jsonl
// Subagent transcripts: <projectsDir>/<projectKey>/<sessionID>/subagents/agent-<id>.jsonl
//
// Returns nil if filePath is not under projectsDir or has an unrecognized shape.
func FilePathToSessionKey(filePath, projectsDir string) *shared.SessionKey {
	rel, err := relPath(projectsDir, filePath)
	if err != nil || rel == "" {
		return nil
	}

	parts := splitPath(rel)
	if len(parts) == 0 || parts[0] == ".." {
		return nil
	}
	if len(parts) < 2 {
		return nil
	}

	projectKey := parts[0]
	second := parts[1]

	// Main transcript: <projectKey>/<sessionID>.jsonl
	if len(parts) == 2 && strings.HasSuffix(second, ".jsonl") {
		return &shared.SessionKey{
			ProjectKey: projectKey,
			SessionID:  strings.TrimSuffix(second, ".jsonl"),
		}
	}

	// Subagent transcript: <projectKey>/<sessionID>/subagents/.../agent-<id>.jsonl
	if len(parts) >= 4 {
		subpathParts := append([]string{}, parts[2:]...)
		last := subpathParts[len(subpathParts)-1]
		if strings.HasSuffix(last, ".jsonl") {
			subpathParts[len(subpathParts)-1] = strings.TrimSuffix(last, ".jsonl")
		}
		return &shared.SessionKey{
			ProjectKey: projectKey,
			SessionID:  second,
			Subpath:    strings.Join(subpathParts, "/"),
		}
	}

	return nil
}

// InMemorySessionStore is an in-memory SessionStore implementation for testing
// and development. Stores entries in a map keyed by a composite
// "projectKey/sessionID[/subpath]" string. Not suitable for production — data
// is lost when the process exits.
type InMemorySessionStore struct {
	mu        sync.Mutex
	store     map[string][]shared.SessionStoreEntry
	mtimes    map[string]int64
	summaries map[summaryKey]shared.SessionSummaryEntry
	lastMtime int64
}

type summaryKey struct {
	projectKey string
	sessionID  string
}

// NewInMemorySessionStore creates a new in-memory SessionStore.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		store:     map[string][]shared.SessionStoreEntry{},
		mtimes:    map[string]int64{},
		summaries: map[summaryKey]shared.SessionSummaryEntry{},
	}
}

func (s *InMemorySessionStore) keyToString(key shared.SessionKey) string {
	parts := []string{key.ProjectKey, key.SessionID}
	if key.Subpath != "" {
		parts = append(parts, key.Subpath)
	}
	return strings.Join(parts, "/")
}

// nextMtime returns a strictly monotonically increasing storage write time
// in Unix epoch milliseconds. Real backends get this property for free from
// commit ordering.
func (s *InMemorySessionStore) nextMtime() int64 {
	now := time.Now().UnixMilli()
	if now <= s.lastMtime {
		now = s.lastMtime + 1
	}
	s.lastMtime = now
	return now
}

// Append mirrors a batch of transcript entries.
func (s *InMemorySessionStore) Append(_ context.Context, key shared.SessionKey, entries []shared.SessionStoreEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := s.keyToString(key)
	s.store[k] = append(s.store[k], entries...)
	now := s.nextMtime()

	// Maintain the per-session summary sidecar incrementally so
	// ListSessionSummaries() never re-reads. Subagent subpaths don't
	// contribute to the main session's summary.
	if key.Subpath == "" {
		sk := summaryKey{projectKey: key.ProjectKey, sessionID: key.SessionID}
		var prev *shared.SessionSummaryEntry
		if existing, ok := s.summaries[sk]; ok {
			cp := existing
			prev = &cp
		}
		folded := FoldSessionSummary(prev, key, entries)
		// Stamp the sidecar with this adapter's storage write time — the
		// SAME clock ListSessions exposes below.
		folded.Mtime = now
		s.summaries[sk] = folded
	}
	s.mtimes[k] = now
	return nil
}

// Load loads a full session for resume.
func (s *InMemorySessionStore) Load(_ context.Context, key shared.SessionKey) ([]shared.SessionStoreEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, ok := s.store[s.keyToString(key)]
	if !ok {
		return nil, nil
	}
	cp := make([]shared.SessionStoreEntry, len(entries))
	copy(cp, entries)
	return cp, nil
}

// ListSessions lists sessions for a projectKey.
func (s *InMemorySessionStore) ListSessions(_ context.Context, projectKey string) ([]shared.SessionStoreListEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := []shared.SessionStoreListEntry{}
	prefix := projectKey + "/"
	for k := range s.store {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := k[len(prefix):]
		// Only include main transcripts (no subpath, so no second '/').
		if !strings.Contains(rest, "/") {
			results = append(results, shared.SessionStoreListEntry{
				SessionID: rest,
				Mtime:     s.mtimes[k],
			})
		}
	}
	return results, nil
}

// ListSessionSummaries returns incremental summaries for all sessions in one call.
func (s *InMemorySessionStore) ListSessionSummaries(_ context.Context, projectKey string) ([]shared.SessionSummaryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := []shared.SessionSummaryEntry{}
	for sk, summary := range s.summaries {
		if sk.projectKey == projectKey {
			results = append(results, summary)
		}
	}
	return results, nil
}

// Delete deletes a session.
//
// Deleting a main-transcript key (no Subpath) cascades to all subkeys under
// that session so subagent transcripts aren't orphaned. A targeted delete with
// an explicit Subpath removes only that one entry.
func (s *InMemorySessionStore) Delete(_ context.Context, key shared.SessionKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.keyToString(key)
	delete(s.store, k)
	delete(s.mtimes, k)
	if key.Subpath == "" {
		delete(s.summaries, summaryKey{projectKey: key.ProjectKey, sessionID: key.SessionID})
		prefix := key.ProjectKey + "/" + key.SessionID + "/"
		for storeKey := range s.store {
			if strings.HasPrefix(storeKey, prefix) {
				delete(s.store, storeKey)
				delete(s.mtimes, storeKey)
			}
		}
	}
	return nil
}

// ListSubkeys lists all subpath keys under a session.
func (s *InMemorySessionStore) ListSubkeys(_ context.Context, key shared.SessionListSubkeysKey) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := key.ProjectKey + "/" + key.SessionID + "/"
	results := []string{}
	for k := range s.store {
		if strings.HasPrefix(k, prefix) {
			results = append(results, k[len(prefix):])
		}
	}
	return results, nil
}

// GetEntries is a test helper that returns all entries for a key (empty slice
// if absent).
func (s *InMemorySessionStore) GetEntries(key shared.SessionKey) []shared.SessionStoreEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := s.store[s.keyToString(key)]
	cp := make([]shared.SessionStoreEntry, len(entries))
	copy(cp, entries)
	return cp
}

// Size returns the number of stored sessions (main transcripts only).
func (s *InMemorySessionStore) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for k := range s.store {
		first := strings.IndexByte(k, '/')
		if first == -1 {
			continue
		}
		// No additional '/' after the first one — that's a main transcript.
		if !strings.Contains(k[first+1:], "/") {
			count++
		}
	}
	return count
}

// Clear removes all stored data.
func (s *InMemorySessionStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = map[string][]shared.SessionStoreEntry{}
	s.mtimes = map[string]int64{}
	s.summaries = map[summaryKey]shared.SessionSummaryEntry{}
	s.lastMtime = 0
}
