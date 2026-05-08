// Package sessions: SessionStore-backed counterparts to ListSessions /
// GetSessionInfo / GetSessionMessages / RenameSession / TagSession /
// DeleteSession / ForkSession.
//
// These mirror Python SDK's *_via_store / *_from_store async helpers and
// implement the Subagent listing/reading helpers as well.
package sessions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// osReadDir is a thin wrapper kept for testability and to keep walkAgentDir
// independent of whether the os package or a shim is used.
func osReadDir(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

// storeListLoadConcurrency is the upper bound on concurrent Load() calls
// issued by ListSessionsFromStore. Keeps large project listings from
// exhausting adapter connection pools or tripping backend rate limits.
const storeListLoadConcurrency = 16

// ListSessionsFromStore lists sessions from a SessionStore.
//
// Async, store-backed counterpart to ListSessions. When the store implements
// ListSessionSummaries, fetches them in one batch call and gap-fills missing
// or stale sidecars via per-session Load(). Otherwise falls back to one
// Load() per session (bounded at storeListLoadConcurrency).
func ListSessionsFromStore(
	ctx context.Context,
	store shared.SessionStore,
	directory string,
	limit *int,
	offset int,
) ([]shared.SDKSessionInfo, error) {
	projectPath := canonicalizePath(directory)
	if directory == "" {
		projectPath = canonicalizePath(".")
	}
	projectKey := sanitizePath(projectPath)

	hasListSessions := storeImplements(store, "ListSessions",
		func() error { _, err := store.ListSessions(ctx, projectKey); return err })

	// Fast path: incremental summaries.
	summaries, summariesErr := store.ListSessionSummaries(ctx, projectKey)
	if summariesErr == nil {
		var listing []shared.SessionStoreListEntry
		knownMtimes := map[string]int64{}
		if hasListSessions {
			ls, err := store.ListSessions(ctx, projectKey)
			if err == nil {
				listing = ls
				for _, e := range ls {
					knownMtimes[e.SessionID] = e.Mtime
				}
			}
		}

		type slot struct {
			mtime     int64
			sessionID string
			info      *shared.SDKSessionInfo
		}
		slots := []slot{}
		freshIDs := map[string]struct{}{}
		for _, s := range summaries {
			if hasListSessions {
				known, ok := knownMtimes[s.SessionID]
				if !ok {
					continue
				}
				if s.Mtime < known {
					// Stale sidecar — let gap-fill re-fold from source.
					continue
				}
			}
			info := SummaryEntryToSDKInfo(s, projectPath)
			if info == nil {
				freshIDs[s.SessionID] = struct{}{}
				continue
			}
			slots = append(slots, slot{mtime: s.Mtime, sessionID: s.SessionID, info: info})
			freshIDs[s.SessionID] = struct{}{}
		}
		if hasListSessions {
			for _, e := range listing {
				if _, ok := freshIDs[e.SessionID]; !ok {
					slots = append(slots, slot{mtime: e.Mtime, sessionID: e.SessionID, info: nil})
				}
			}
		}

		sort.SliceStable(slots, func(i, j int) bool { return slots[i].mtime > slots[j].mtime })
		page := slots
		if offset > 0 && offset < len(page) {
			page = page[offset:]
		} else if offset >= len(page) {
			page = nil
		}
		if limit != nil && *limit > 0 && *limit < len(page) {
			page = page[:*limit]
		}

		// Gap-fill placeholders.
		toFill := []slot{}
		for _, sl := range page {
			if sl.info == nil {
				toFill = append(toFill, sl)
			}
		}
		if len(toFill) > 0 {
			ids := make([]string, len(toFill))
			for i, sl := range toFill {
				ids[i] = sl.sessionID
			}
			filled, err := deriveInfosViaLoad(ctx, store, projectKey, ids, directory, projectPath)
			if err != nil {
				return nil, err
			}
			bySID := map[string]*shared.SDKSessionInfo{}
			for i := range filled {
				bySID[filled[i].SessionID] = &filled[i]
			}
			for i := range page {
				if page[i].info == nil {
					page[i].info = bySID[page[i].sessionID]
				}
			}
		}

		results := make([]shared.SDKSessionInfo, 0, len(page))
		for _, sl := range page {
			if sl.info != nil {
				results = append(results, *sl.info)
			}
		}
		return results, nil
	}

	if !errors.Is(summariesErr, shared.ErrSessionStoreNotImplemented) {
		// Real error — propagate. (Not falling through to ListSessions.)
		return nil, summariesErr
	}

	if !hasListSessions {
		return nil, errors.New(
			"session_store implements neither ListSessionSummaries nor ListSessions; cannot list",
		)
	}
	listing, err := store.ListSessions(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(listing))
	for i, e := range listing {
		ids[i] = e.SessionID
	}
	results, err := deriveInfosViaLoad(ctx, store, projectKey, ids, directory, projectPath)
	if err != nil {
		return nil, err
	}
	// Stamp mtimes from listing.
	mtimeByID := map[string]int64{}
	for _, e := range listing {
		mtimeByID[e.SessionID] = e.Mtime
	}
	for i := range results {
		if m, ok := mtimeByID[results[i].SessionID]; ok {
			results[i].LastModified = m
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].LastModified > results[j].LastModified })
	if offset > 0 {
		if offset >= len(results) {
			return []shared.SDKSessionInfo{}, nil
		}
		results = results[offset:]
	}
	if limit != nil && *limit > 0 && *limit < len(results) {
		results = results[:*limit]
	}
	return results, nil
}

// GetSessionInfoFromStore reads metadata for a single session from a SessionStore.
func GetSessionInfoFromStore(
	ctx context.Context,
	store shared.SessionStore,
	sessionID, directory string,
) (*shared.SDKSessionInfo, error) {
	if !validateUUID(sessionID) {
		return nil, nil
	}
	projectKey := ProjectKeyForDirectory(directory)
	entries, err := store.Load(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	// Build a synthetic summary by folding all entries.
	summary := FoldSessionSummary(nil, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID}, entries)
	// Use last entry's timestamp as mtime fallback.
	if last, ok := entries[len(entries)-1]["timestamp"]; ok {
		if ms := isoToEpochMs(last); ms > 0 {
			summary.Mtime = ms
		}
	}
	projectPath := canonicalizePath(directory)
	if directory == "" {
		projectPath = canonicalizePath(".")
	}
	return SummaryEntryToSDKInfo(summary, projectPath), nil
}

// GetSessionMessagesFromStore reads a session's conversation messages from a SessionStore.
func GetSessionMessagesFromStore(
	ctx context.Context,
	store shared.SessionStore,
	sessionID, directory string,
	limit *int,
	offset int,
) ([]shared.SessionMessage, error) {
	if !validateUUID(sessionID) {
		return []shared.SessionMessage{}, nil
	}
	projectKey := ProjectKeyForDirectory(directory)
	entries, err := store.Load(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []shared.SessionMessage{}, nil
	}
	transcript := filterTranscriptEntries(entries)
	chain := buildConversationChain(transcript)
	visible := []transcriptEntry{}
	for _, e := range chain {
		if isVisibleMessage(e) {
			visible = append(visible, e)
		}
	}
	messages := make([]shared.SessionMessage, 0, len(visible))
	for _, e := range visible {
		// Python parity: SessionMessage.parent_tool_use_id is always None
		// for top-level conversation messages (sidechain messages are
		// filtered out by isVisibleMessage above).
		messages = append(messages, shared.SessionMessage{
			Type:            e.Type,
			UUID:            e.UUID,
			SessionID:       e.SessionID,
			Message:         e.Message,
			ParentToolUseID: nil,
		})
	}
	return paginate(messages, limit, offset), nil
}

// ListSubagents lists subagent IDs for a given session by scanning the
// subagents directory under the session's local on-disk layout.
func ListSubagents(sessionID, directory string) []string {
	if !validateUUID(sessionID) {
		return []string{}
	}
	subDir := resolveSubagentsDir(sessionID, directory)
	if subDir == "" {
		return []string{}
	}
	out := []string{}
	for _, p := range collectAgentFiles(subDir) {
		out = append(out, p.agentID)
	}
	return out
}

// GetSubagentMessages reads a subagent's conversation messages from its
// JSONL transcript file.
func GetSubagentMessages(sessionID, agentID, directory string, limit *int, offset int) []shared.SessionMessage {
	if !validateUUID(sessionID) || agentID == "" {
		return []shared.SessionMessage{}
	}
	subDir := resolveSubagentsDir(sessionID, directory)
	if subDir == "" {
		return []shared.SessionMessage{}
	}
	for _, p := range collectAgentFiles(subDir) {
		if p.agentID != agentID {
			continue
		}
		entries, err := readRawEntries(p.path)
		if err != nil {
			return []shared.SessionMessage{}
		}
		transcript := filterTranscriptEntries(entries)
		return entriesToSubagentMessages(transcript, limit, offset)
	}
	return []shared.SessionMessage{}
}

// ListSubagentsFromStore lists subagent IDs for a session from a SessionStore.
func ListSubagentsFromStore(
	ctx context.Context,
	store shared.SessionStore,
	sessionID, directory string,
) ([]string, error) {
	if !validateUUID(sessionID) {
		return []string{}, nil
	}
	projectKey := ProjectKeyForDirectory(directory)
	subkeys, err := store.ListSubkeys(ctx, shared.SessionListSubkeysKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	})
	if errors.Is(err, shared.ErrSessionStoreNotImplemented) {
		return nil, errors.New(
			"session_store does not implement ListSubkeys; cannot list subagents",
		)
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	ids := []string{}
	for _, sub := range subkeys {
		if !strings.HasPrefix(sub, "subagents/") {
			continue
		}
		idx := strings.LastIndex(sub, "/")
		last := sub[idx+1:]
		if !strings.HasPrefix(last, "agent-") {
			continue
		}
		id := strings.TrimPrefix(last, "agent-")
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetSubagentMessagesFromStore reads a subagent's conversation messages from
// a SessionStore.
func GetSubagentMessagesFromStore(
	ctx context.Context,
	store shared.SessionStore,
	sessionID, agentID, directory string,
	limit *int,
	offset int,
) ([]shared.SessionMessage, error) {
	if !validateUUID(sessionID) || agentID == "" {
		return []shared.SessionMessage{}, nil
	}
	projectKey := ProjectKeyForDirectory(directory)

	subpath := "subagents/agent-" + agentID
	if subkeys, err := store.ListSubkeys(ctx, shared.SessionListSubkeysKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	}); err == nil {
		target := "agent-" + agentID
		match := ""
		for _, sk := range subkeys {
			if !strings.HasPrefix(sk, "subagents/") {
				continue
			}
			idx := strings.LastIndex(sk, "/")
			if sk[idx+1:] == target {
				match = sk
				break
			}
		}
		if match == "" {
			return []shared.SessionMessage{}, nil
		}
		subpath = match
	} else if !errors.Is(err, shared.ErrSessionStoreNotImplemented) {
		return nil, err
	}

	entries, err := store.Load(ctx, shared.SessionKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
		Subpath:    subpath,
	})
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return []shared.SessionMessage{}, nil
	}
	// Drop synthetic agent_metadata entries.
	transcript := []shared.SessionStoreEntry{}
	for _, e := range entries {
		if e["type"] == "agent_metadata" {
			continue
		}
		transcript = append(transcript, e)
	}
	if len(transcript) == 0 {
		return []shared.SessionMessage{}, nil
	}
	filtered := filterTranscriptEntries(transcript)
	return entriesToSubagentMessages(filtered, limit, offset), nil
}

// --- mutation helpers ---

// RenameSessionViaStore appends a custom-title entry to a SessionStore.
func RenameSessionViaStore(ctx context.Context, store shared.SessionStore, sessionID, title, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	stripped := strings.TrimSpace(title)
	if stripped == "" {
		return errors.New("title must be non-empty")
	}
	projectKey := ProjectKeyForDirectory(directory)
	entry := shared.SessionStoreEntry{
		"type":        "custom-title",
		"customTitle": stripped,
		"sessionId":   sessionID,
		"uuid":        newUUID(),
		"timestamp":   isoNow(),
	}
	return store.Append(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID}, []shared.SessionStoreEntry{entry})
}

// TagSessionViaStore appends a tag entry to a SessionStore. Pass tag=nil to clear.
func TagSessionViaStore(ctx context.Context, store shared.SessionStore, sessionID string, tag *string, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	tagValue := ""
	if tag != nil {
		sanitized := strings.TrimSpace(sanitizeUnicode(*tag))
		if sanitized == "" {
			return errors.New("tag must be non-empty (use nil to clear)")
		}
		tagValue = sanitized
	}
	projectKey := ProjectKeyForDirectory(directory)
	entry := shared.SessionStoreEntry{
		"type":      "tag",
		"tag":       tagValue,
		"sessionId": sessionID,
		"uuid":      newUUID(),
		"timestamp": isoNow(),
	}
	return store.Append(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID}, []shared.SessionStoreEntry{entry})
}

// DeleteSessionViaStore deletes a session from a SessionStore. No-op when
// the adapter doesn't implement Delete (matches the Python contract).
func DeleteSessionViaStore(ctx context.Context, store shared.SessionStore, sessionID, directory string) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	projectKey := ProjectKeyForDirectory(directory)
	err := store.Delete(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID})
	if errors.Is(err, shared.ErrSessionStoreNotImplemented) {
		return nil
	}
	return err
}

// --- internal helpers ---

func filterTranscriptEntries(entries []shared.SessionStoreEntry) []transcriptEntry {
	result := make([]transcriptEntry, 0, len(entries))
	for _, e := range entries {
		t, _ := e["type"].(string)
		uuid, _ := e["uuid"].(string)
		if !transcriptTypes[t] || uuid == "" {
			continue
		}
		entry := transcriptEntry{
			Type:      t,
			UUID:      uuid,
			SessionID: stringValue(e, "sessionId"),
			Raw:       e,
		}
		if msg, ok := e["message"].(map[string]any); ok {
			entry.Message = msg
		}
		if v, ok := e["parentToolUseId"].(string); ok {
			entry.ParentToolUseID = &v
		}
		if v, ok := e["parentUuid"].(string); ok {
			entry.ParentUUID = &v
		}
		result = append(result, entry)
	}
	return result
}

func entriesToSubagentMessages(entries []transcriptEntry, limit *int, offset int) []shared.SessionMessage {
	chain := buildSubagentChain(entries)
	messages := make([]shared.SessionMessage, 0, len(chain))
	for _, e := range chain {
		if e.Type != "user" && e.Type != "assistant" {
			continue
		}
		// Python parity: SessionMessage.parent_tool_use_id is always None.
		messages = append(messages, shared.SessionMessage{
			Type:            e.Type,
			UUID:            e.UUID,
			SessionID:       e.SessionID,
			Message:         e.Message,
			ParentToolUseID: nil,
		})
	}
	return paginate(messages, limit, offset)
}

// buildSubagentChain builds the linear conversation chain for a subagent
// transcript. Subagent transcripts have no compaction or sidechains; find
// the last user/assistant entry and walk parentUuid back to the root.
func buildSubagentChain(entries []transcriptEntry) []transcriptEntry {
	if len(entries) == 0 {
		return nil
	}
	byUUID := make(map[string]transcriptEntry, len(entries))
	for _, e := range entries {
		byUUID[e.UUID] = e
	}
	var leaf *transcriptEntry
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == "user" || entries[i].Type == "assistant" {
			leaf = &entries[i]
			break
		}
	}
	if leaf == nil {
		return nil
	}
	chain := []transcriptEntry{}
	seen := map[string]struct{}{}
	current := leaf
	for current != nil {
		if _, ok := seen[current.UUID]; ok {
			break
		}
		seen[current.UUID] = struct{}{}
		chain = append(chain, *current)
		if current.ParentUUID == nil {
			break
		}
		next, ok := byUUID[*current.ParentUUID]
		if !ok {
			break
		}
		current = &next
	}
	// Reverse.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func paginate(messages []shared.SessionMessage, limit *int, offset int) []shared.SessionMessage {
	if offset > 0 {
		if offset >= len(messages) {
			return []shared.SessionMessage{}
		}
		messages = messages[offset:]
	}
	if limit != nil && *limit > 0 && *limit < len(messages) {
		messages = messages[:*limit]
	}
	return messages
}

// agentFile represents a discovered subagent transcript file.
type agentFile struct {
	agentID string
	path    string
}

// resolveSubagentsDir resolves the subagents directory for a session.
// Returns "" if not found.
func resolveSubagentsDir(sessionID, directory string) string {
	resolved, _ := findSessionFile(sessionID, directory)
	if resolved == "" {
		return ""
	}
	// Strip the .jsonl suffix to derive the session directory.
	sessionDir := strings.TrimSuffix(resolved, ".jsonl")
	return sessionDir + string(separator()) + "subagents"
}

// collectAgentFiles recursively walks a subagents directory and returns each
// discovered agent-<id>.jsonl file as agentFile.
func collectAgentFiles(baseDir string) []agentFile {
	var results []agentFile
	walkAgentDir(baseDir, &results)
	return results
}

// deriveInfosViaLoad loads each session's entries to derive a real summary
// via fold + SummaryEntryToSDKInfo. Bounded concurrency to avoid exhausting
// adapter connection pools.
func deriveInfosViaLoad(
	ctx context.Context,
	store shared.SessionStore,
	projectKey string,
	sessionIDs []string,
	directory, projectPath string,
) ([]shared.SDKSessionInfo, error) {
	type result struct {
		idx  int
		info *shared.SDKSessionInfo
	}
	sem := make(chan struct{}, storeListLoadConcurrency)
	out := make([]result, len(sessionIDs))
	errs := make([]error, len(sessionIDs))
	done := make(chan struct{})
	count := 0

	for i, sid := range sessionIDs {
		i, sid := i, sid
		go func() {
			sem <- struct{}{}
			defer func() {
				<-sem
				count++
				if count == len(sessionIDs) {
					close(done)
				}
			}()
			entries, err := store.Load(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sid})
			if err != nil {
				out[i] = result{idx: i, info: &shared.SDKSessionInfo{SessionID: sid}}
				return
			}
			if len(entries) == 0 {
				out[i] = result{idx: i, info: nil}
				return
			}
			summary := FoldSessionSummary(nil, shared.SessionKey{ProjectKey: projectKey, SessionID: sid}, entries)
			if last, ok := entries[len(entries)-1]["timestamp"]; ok {
				if ms := isoToEpochMs(last); ms > 0 {
					summary.Mtime = ms
				}
			}
			info := SummaryEntryToSDKInfo(summary, projectPath)
			out[i] = result{idx: i, info: info}
		}()
	}

	if len(sessionIDs) == 0 {
		return nil, nil
	}
	<-done

	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	results := make([]shared.SDKSessionInfo, 0, len(out))
	for _, r := range out {
		if r.info != nil {
			results = append(results, *r.info)
		}
	}
	return results, nil
}

// storeImplements probes whether a store actually implements an optional
// method by performing a cheap call and inspecting the error.
func storeImplements(_ shared.SessionStore, _ string, probe func() error) bool {
	err := probe()
	return !errors.Is(err, shared.ErrSessionStoreNotImplemented)
}

func stringValue(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func isoNow() string {
	return timeNowUTC().Format("2006-01-02T15:04:05.000Z")
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

// separator returns the platform path separator as a byte.
func separator() byte {
	return byte('/')
}

// walkAgentDir recursively walks dir and appends agentFile entries for each
// file matching agent-*.jsonl.
func walkAgentDir(dir string, out *[]agentFile) {
	dirents, err := osReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range dirents {
		name := d.Name()
		full := dir + string(separator()) + name
		if d.IsDir() {
			walkAgentDir(full, out)
			continue
		}
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		agentID := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		*out = append(*out, agentFile{agentID: agentID, path: full})
	}
}
