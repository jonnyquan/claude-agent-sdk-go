// Package sessions: materialize a SessionStore-backed resume into a temp
// CLAUDE_CONFIG_DIR.
//
// When Options.Resume (or Options.ContinueConversation) is paired with
// Options.SessionStore, the session JSONL almost certainly does not exist
// on local disk — it lives in the external store. The CLI subprocess only
// knows how to resume from a local file. This module bridges the gap: it
// loads the session from the store, writes it to a temporary directory laid
// out exactly like ~/.claude/, and returns the path so the caller can point
// the subprocess at it via CLAUDE_CONFIG_DIR.
//
// Mirrors the behavior of the Python and TypeScript SDKs.
package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// MaterializedResume is the result of MaterializeResumeSession.
//
// ConfigDir is the temporary directory laid out like ~/.claude/ — point the
// subprocess at it via CLAUDE_CONFIG_DIR.
//
// ResumeSessionID is the session ID to pass as --resume. When the input was
// ContinueConversation, this is the most-recent session resolved via
// SessionStore.ListSessions.
//
// Cleanup removes ConfigDir (best-effort). Call it after the subprocess exits.
type MaterializedResume struct {
	ConfigDir       string
	ResumeSessionID string
	Cleanup         func() error
}

// MaterializeResumeSession loads a session from store and writes it to a temp
// dir.
//
// Returns (nil, nil) when no materialization is needed (no store, no
// resume/continue, store has no entries, or the resolved session ID is not
// a valid UUID) — caller falls through to the normal (no-store) resume/spawn
// path. For ContinueConversation this means a fresh session; for an explicit
// Resume value the CLI receives it unchanged.
//
// Returns an error if a store call fails or times out.
func MaterializeResumeSession(ctx context.Context, options *shared.Options) (*MaterializedResume, error) {
	store := options.SessionStore
	if store == nil {
		return nil, nil
	}
	if options.Resume == nil && !options.ContinueConversation {
		return nil, nil
	}

	cwd := ""
	if options.Cwd != nil {
		cwd = *options.Cwd
	}
	projectKey := ProjectKeyForDirectory(cwd)

	// Enforce Options.LoadTimeoutMs on every Load/ListSessions call,
	// matching Python's _with_timeout. Defaults to 60_000 ms when zero.
	timeoutMs := options.LoadTimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 60_000
	}
	loadCtx, cancelLoad := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancelLoad()

	var (
		sessionID string
		entries   []shared.SessionStoreEntry
		err       error
	)

	if options.Resume != nil {
		// session_id is used as a path component below; reject anything that
		// isn't a UUID to prevent traversal and match every other resume path.
		if !validateUUID(*options.Resume) {
			return nil, nil
		}
		sessionID = *options.Resume
		entries, err = loadCandidate(loadCtx, store, projectKey, sessionID)
		if err != nil {
			return nil, fmt.Errorf("SessionStore.Load() for session %s during resume materialization: %w",
				sessionID, err)
		}
	} else {
		sessionID, entries, err = resolveContinueCandidate(loadCtx, store, projectKey)
		if err != nil {
			return nil, fmt.Errorf("SessionStore.ListSessions() during resume materialization: %w", err)
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}

	tmpBase, err := os.MkdirTemp("", "claude-resume-")
	if err != nil {
		return nil, fmt.Errorf("create temp resume dir: %w", err)
	}
	cleanup := func() error {
		return os.RemoveAll(tmpBase)
	}

	projectDir := filepath.Join(tmpBase, "projects", projectKey)
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		_ = cleanup()
		return nil, fmt.Errorf("create project dir: %w", err)
	}
	if err := writeJSONL(filepath.Join(projectDir, sessionID+".jsonl"), entries); err != nil {
		_ = cleanup()
		return nil, err
	}

	// Copy auth config from the caller's effective config locations so the
	// subprocess can authenticate. Missing files are fine (API-key auth, etc.).
	copyAuthFiles(tmpBase, options.ExtraEnv)

	// Materialize subagent transcripts if the store can enumerate them.
	// Reuse the timeout-bounded loadCtx for ListSubkeys/Load calls.
	if err := materializeSubkeys(loadCtx, store, projectDir, projectKey, sessionID); err != nil {
		// Subagent materialization failures are non-fatal — log and continue.
		// (The session itself is materialized; only the subagent .jsonl
		// sidekicks are missing, and the CLI handles that gracefully.)
	}

	return &MaterializedResume{
		ConfigDir:       tmpBase,
		ResumeSessionID: sessionID,
		Cleanup:         cleanup,
	}, nil
}

// ApplyMaterializedOptions returns a copy of options repointed at a
// materialized temp config dir.
//
// Sets CLAUDE_CONFIG_DIR in ExtraEnv, Resume to the materialized session id,
// and clears ContinueConversation (already resolved to a concrete session id
// during materialization).
func ApplyMaterializedOptions(options *shared.Options, m *MaterializedResume) *shared.Options {
	if options == nil || m == nil {
		return options
	}
	out := *options
	if out.ExtraEnv == nil {
		out.ExtraEnv = map[string]string{}
	} else {
		// Defensive copy so we don't mutate the caller's map.
		copyEnv := make(map[string]string, len(out.ExtraEnv)+1)
		for k, v := range out.ExtraEnv {
			copyEnv[k] = v
		}
		out.ExtraEnv = copyEnv
	}
	out.ExtraEnv["CLAUDE_CONFIG_DIR"] = m.ConfigDir
	resumeID := m.ResumeSessionID
	out.Resume = &resumeID
	out.ContinueConversation = false
	return &out
}

// ValidateSessionStoreOptions raises an error for invalid SessionStore option
// combinations. Called before subprocess spawn so misconfiguration fails fast.
func ValidateSessionStoreOptions(ctx context.Context, options *shared.Options) error {
	store := options.SessionStore
	if store == nil {
		return nil
	}

	if options.ContinueConversation && options.Resume == nil {
		// When Resume is explicitly set, ListSessions is provably never
		// called (Resume wins over Continue), so a minimal store is fine.
		// Probe ListSessions implementation by calling with empty key.
		_, err := store.ListSessions(ctx, ProjectKeyForDirectory(""))
		if errors.Is(err, shared.ErrSessionStoreNotImplemented) {
			return errors.New(
				"continue_conversation with session_store requires the store to implement ListSessions()",
			)
		}
	}

	if options.EnableFileCheckpointing {
		return errors.New(
			"session_store cannot be combined with enable_file_checkpointing " +
				"(checkpoints are local-disk only and would diverge from the mirrored transcript)",
		)
	}
	return nil
}

// --- Helpers ---

func loadCandidate(ctx context.Context, store shared.SessionStore, projectKey, sessionID string) ([]shared.SessionStoreEntry, error) {
	entries, err := store.Load(ctx, shared.SessionKey{ProjectKey: projectKey, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func resolveContinueCandidate(ctx context.Context, store shared.SessionStore, projectKey string) (string, []shared.SessionStoreEntry, error) {
	listed, err := store.ListSessions(ctx, projectKey)
	if err != nil {
		return "", nil, err
	}
	if len(listed) == 0 {
		return "", nil, nil
	}
	// Newest → oldest, skip sidechains and invalid IDs.
	sort.SliceStable(listed, func(i, j int) bool { return listed[i].Mtime > listed[j].Mtime })
	for _, cand := range listed {
		if !validateUUID(cand.SessionID) {
			continue
		}
		entries, err := loadCandidate(ctx, store, projectKey, cand.SessionID)
		if err != nil {
			return "", nil, err
		}
		if len(entries) == 0 {
			continue
		}
		first := entries[0]
		if first["isSidechain"] == true {
			continue
		}
		return cand.SessionID, entries, nil
	}
	return "", nil, nil
}

func writeJSONL(path string, entries []shared.SessionStoreEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, e := range entries {
		// Encode with stable shape; we don't byte-compare entries elsewhere.
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// copyAuthFiles copies .credentials.json (with refreshToken redacted) and
// .claude.json from the caller's config dir to the materialized tmp tree so
// the subprocess can authenticate.
//
// macOS default OAuth setup keeps tokens in the Keychain, not a file.
// Redirecting CLAUDE_CONFIG_DIR changes the Keychain service-name suffix,
// so the resumed subprocess's Keychain lookup misses and falls back to
// plainTextStorage at ${tmpBase}/.credentials.json. We populate that file
// from the parent's Keychain so the resumed subprocess can authenticate.
// Skipped when env-based auth or a custom CLAUDE_CONFIG_DIR is in play.
// Mirrors Python SDK's _copy_auth_files Keychain fallback.
func copyAuthFiles(tmpBase string, optEnv map[string]string) {
	caller := optEnv["CLAUDE_CONFIG_DIR"]
	if caller == "" {
		caller = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	var sourceConfigDir string
	if caller != "" {
		sourceConfigDir = caller
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			sourceConfigDir = filepath.Join(home, ".claude")
		}
	}

	// .credentials.json — copy with refresh token redacted.
	var credsJSON []byte
	if sourceConfigDir != "" {
		credsPath := filepath.Join(sourceConfigDir, ".credentials.json")
		if data, err := os.ReadFile(credsPath); err == nil {
			credsJSON = data
		}
	}

	// macOS Keychain fallback when no .credentials.json on disk and no
	// caller-supplied CLAUDE_CONFIG_DIR or env-based auth in play.
	if caller == "" && envAuthAbsent(optEnv) {
		if keychainCreds := readKeychainCredentials(); keychainCreds != nil {
			credsJSON = keychainCreds
		}
	}

	if credsJSON != nil {
		redacted := redactRefreshToken(credsJSON)
		dst := filepath.Join(tmpBase, ".credentials.json")
		_ = os.WriteFile(dst, redacted, 0o600)
	}

	// .claude.json lives at $CLAUDE_CONFIG_DIR/.claude.json when set, else
	// ~/.claude.json (NOT ~/.claude/.claude.json).
	var claudeJSONSrc string
	if caller != "" {
		claudeJSONSrc = filepath.Join(caller, ".claude.json")
	} else if home, err := os.UserHomeDir(); err == nil {
		claudeJSONSrc = filepath.Join(home, ".claude.json")
	}
	if claudeJSONSrc != "" {
		copyIfPresent(claudeJSONSrc, filepath.Join(tmpBase, ".claude.json"))
	}
}

// envAuthAbsent returns true when neither ANTHROPIC_API_KEY nor
// CLAUDE_CODE_OAUTH_TOKEN is set in optEnv or the process env. When auth
// already comes from env, the Keychain fallback would be a wasted spawn.
func envAuthAbsent(optEnv map[string]string) bool {
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if v, ok := optEnv[key]; ok && v != "" {
			return false
		}
		if os.Getenv(key) != "" {
			return false
		}
	}
	return true
}

// redactRefreshToken removes claudeAiOauth.refreshToken from a credentials
// file payload. Single-use refresh token would be consumed server-side and
// new tokens written to a location the parent never reads back, leaving the
// parent's stored creds revoked.
func redactRefreshToken(creds []byte) []byte {
	var data map[string]any
	if err := json.Unmarshal(creds, &data); err != nil {
		return creds
	}
	oauth, ok := data["claudeAiOauth"].(map[string]any)
	if !ok {
		return creds
	}
	if _, has := oauth["refreshToken"]; !has {
		return creds
	}
	delete(oauth, "refreshToken")
	out, err := json.Marshal(data)
	if err != nil {
		return creds
	}
	return out
}

func copyIfPresent(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}
	defer out.Close()
	_, _ = io.Copy(out, in)
}

func materializeSubkeys(
	ctx context.Context,
	store shared.SessionStore,
	projectDir, projectKey, sessionID string,
) error {
	subkeys, err := store.ListSubkeys(ctx, shared.SessionListSubkeysKey{
		ProjectKey: projectKey,
		SessionID:  sessionID,
	})
	if errors.Is(err, shared.ErrSessionStoreNotImplemented) {
		return nil
	}
	if err != nil {
		return err
	}
	sessionDir := filepath.Join(projectDir, sessionID)
	for _, subpath := range subkeys {
		// Subpaths come from an external store and are used as filesystem
		// path components below. Reject anything that would escape the
		// session directory.
		if !isSafeSubpath(subpath, sessionDir) {
			continue
		}
		subEntries, err := store.Load(ctx, shared.SessionKey{
			ProjectKey: projectKey,
			SessionID:  sessionID,
			Subpath:    subpath,
		})
		if err != nil {
			continue
		}
		if len(subEntries) == 0 {
			continue
		}
		// Partition: agent_metadata entries describe the .meta.json sidecar;
		// everything else is a transcript line.
		var metadata []shared.SessionStoreEntry
		var transcript []shared.SessionStoreEntry
		for _, e := range subEntries {
			if e["type"] == "agent_metadata" {
				metadata = append(metadata, e)
			} else {
				transcript = append(transcript, e)
			}
		}

		subFile := filepath.Join(sessionDir, subpath) + ".jsonl"
		if len(transcript) > 0 {
			if err := writeJSONL(subFile, transcript); err != nil {
				continue
			}
		}
		if len(metadata) > 0 {
			// Last metadata entry wins; strip the synthetic type field.
			last := metadata[len(metadata)-1]
			meta := make(map[string]any, len(last))
			for k, v := range last {
				if k == "type" {
					continue
				}
				meta[k] = v
			}
			metaFile := strings.TrimSuffix(subFile, ".jsonl") + ".meta.json"
			if data, err := json.Marshal(meta); err == nil {
				_ = os.MkdirAll(filepath.Dir(metaFile), 0o700)
				_ = os.WriteFile(metaFile, data, 0o600)
			}
		}
	}
	return nil
}

func isSafeSubpath(subpath, sessionDir string) bool {
	if subpath == "" {
		return false
	}
	if filepath.IsAbs(subpath) || strings.HasPrefix(subpath, "/") || strings.HasPrefix(subpath, `\`) {
		return false
	}
	for _, p := range strings.FieldsFunc(subpath, func(r rune) bool { return r == '/' || r == '\\' }) {
		if p == "." || p == ".." {
			return false
		}
	}
	if strings.ContainsRune(subpath, 0) {
		return false
	}
	target, err := filepath.Abs(filepath.Join(sessionDir, subpath))
	if err != nil {
		return false
	}
	base, err := filepath.Abs(sessionDir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(target, base+string(filepath.Separator)) || target == base
}
