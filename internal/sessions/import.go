// Package sessions: replay a local on-disk session transcript into a
// SessionStore.
//
// This is the inverse of MaterializeResumeSession — where MaterializeResumeSession
// reads a store and writes a temp ~/.claude tree, ImportSessionToStore reads
// the local ~/.claude/projects/<dir>/<sessionID>.jsonl (plus subagent
// transcripts) and replays each line into store.Append().
//
// Mirrors Python SDK's import_session_to_store.
package sessions

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// ImportSessionOptions configures ImportSessionToStore behavior.
type ImportSessionOptions struct {
	// Directory selects the project directory the session lives in. When
	// empty, all project directories are searched for the session file.
	Directory string

	// IncludeSubagents controls whether to also import subagent transcripts
	// under <sessionID>/subagents/** and their .meta.json sidecars.
	// Defaults to true when zero-valued — set to false explicitly to skip.
	IncludeSubagents *bool

	// BatchSize is the maximum entries per Append() call. Default
	// MirrorMaxPendingEntries (500). Negative or zero values use the default.
	BatchSize int
}

// ImportSessionToStore replays a local session transcript into a SessionStore.
//
// Streams the on-disk JSONL line-by-line and calls store.Append(key, batch)
// every BatchSize entries (or 1 MiB of line bytes, whichever comes first).
// Useful for migrating existing local sessions to a remote store, or for
// catching a store up after a MirrorErrorMessage indicated a live-mirror gap.
// Adapters should treat entry["uuid"] as an idempotency key so re-import is
// duplicate-safe.
//
// The destination ProjectKey is the name of the on-disk project directory
// the session file was found in — matching what FilePathToSessionKey produces
// for the same file — so an imported session is indistinguishable from a
// live-mirrored one and resumable via Options{SessionStore, Resume:sessionID}
// from the original cwd.
func ImportSessionToStore(
	ctx context.Context,
	sessionID string,
	store shared.SessionStore,
	opts *ImportSessionOptions,
) error {
	if !validateUUID(sessionID) {
		return fmt.Errorf("invalid session_id: %s", sessionID)
	}
	if opts == nil {
		opts = &ImportSessionOptions{}
	}
	includeSubagents := true
	if opts.IncludeSubagents != nil {
		includeSubagents = *opts.IncludeSubagents
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = MirrorMaxPendingEntries
	}

	resolvedPath, _ := findSessionFile(sessionID, opts.Directory)
	if resolvedPath == "" {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Key under the on-disk project directory name — matches
	// FilePathToSessionKey / TranscriptMirrorBatcher even when the resolver's
	// search (Directory empty) found the file somewhere other than
	// `Directory`.
	projectKey := filepath.Base(filepath.Dir(resolvedPath))

	mainKey := shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID}
	if err := appendJSONLFileInBatches(ctx, resolvedPath, mainKey, store, batchSize); err != nil {
		return err
	}

	if !includeSubagents {
		return nil
	}

	// Subagent transcripts live at <projectDir>/<sessionID>/subagents/**.
	sessionDir := strings.TrimSuffix(resolvedPath, ".jsonl")
	subagentsDir := filepath.Join(sessionDir, "subagents")
	for _, filePath := range collectJSONLFiles(subagentsDir) {
		// subpath is the path relative to sessionDir, /-joined, sans .jsonl —
		// e.g. subagents/agent-abc or subagents/workflows/run-1/agent-def.
		// Matches FilePathToSessionKey so ListSubkeys and
		// GetSubagentMessagesFromStore round-trip.
		rel, err := filepath.Rel(sessionDir, filePath)
		if err != nil {
			continue
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 {
			continue
		}
		parts[len(parts)-1] = strings.TrimSuffix(parts[len(parts)-1], ".jsonl")
		subKey := shared.SessionKey{
			ProjectKey: projectKey,
			SessionID:  sessionID,
			Subpath:    strings.Join(parts, "/"),
		}
		if err := appendJSONLFileInBatches(ctx, filePath, subKey, store, batchSize); err != nil {
			return err
		}

		// The on-disk .jsonl does NOT contain agent_metadata entries — those
		// are only sent to live mirrors and persisted in the .meta.json
		// sidecar. Import the sidecar so MaterializeResumeSession can recreate
		// it and resumed subagents keep their agentType/worktreePath.
		metaPath := strings.TrimSuffix(filePath, ".jsonl") + ".meta.json"
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		// Inject the synthetic type field so adapters can identify it.
		meta["type"] = "agent_metadata"
		if err := store.Append(ctx, subKey, []shared.SessionStoreEntry{meta}); err != nil {
			return err
		}
	}
	return nil
}

// appendJSONLFileInBatches stream-reads a JSONL file line-by-line, parsing
// each line, and flushes to store.Append in batches of batchSize entries
// (or MirrorMaxPendingBytes of line text, whichever comes first). Skips
// blank lines.
func appendJSONLFileInBatches(
	ctx context.Context,
	filePath string,
	key shared.SessionKey,
	store shared.SessionStore,
	batchSize int,
) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 64*1024)
	batch := make([]shared.SessionStoreEntry, 0, batchSize)
	nbytes := 0
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			trimmed := strings.TrimRight(line, "\n")
			if trimmed != "" {
				var entry shared.SessionStoreEntry
				if jerr := json.Unmarshal([]byte(trimmed), &entry); jerr != nil {
					// Skip malformed lines rather than aborting the whole
					// import — matches Python's tolerance for trailing/blank
					// content.
				} else {
					batch = append(batch, entry)
					nbytes += len(trimmed)
				}
			}
		}
		if len(batch) >= batchSize || nbytes >= MirrorMaxPendingBytes {
			if err := store.Append(ctx, key, batch); err != nil {
				return err
			}
			batch = batch[:0]
			nbytes = 0
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if len(batch) > 0 {
		if err := store.Append(ctx, key, batch); err != nil {
			return err
		}
	}
	return nil
}

// collectJSONLFiles recursively collects all *.jsonl files under baseDir.
// Returns nothing if baseDir does not exist. Sorted per directory so import
// order is deterministic across platforms.
func collectJSONLFiles(baseDir string) []string {
	var results []string
	walkJSONL(baseDir, &results)
	return results
}

func walkJSONL(dir string, out *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// Sort for deterministic order.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			walkJSONL(full, out)
			continue
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			*out = append(*out, full)
		}
	}
}
