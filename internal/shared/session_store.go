package shared

import (
	"context"
)

// SessionStoreEntry is one JSONL transcript line as observed by a SessionStore
// adapter. Adapters should treat entries as opaque pass-through blobs;
// round-tripping JSON encode/decode is the only required invariant.
//
// The concrete shape is the CLI's on-disk transcript format (a large
// discriminated union); the SDK never hashes or byte-compares entries, so
// adapters that reorder object keys (e.g. Postgres JSONB) remain valid.
type SessionStoreEntry = map[string]any

// SessionStoreListEntry is the result of SessionStore.ListSessions.
type SessionStoreListEntry struct {
	SessionID string `json:"session_id"`
	// Mtime is the last-modified time in Unix epoch milliseconds. Adapters
	// without native modification time (e.g. Redis) must maintain their own
	// index.
	Mtime int64 `json:"mtime"`
}

// SessionSummaryEntry is an incrementally-maintained session summary.
//
// Stores obtain this from FoldSessionSummary inside SessionStore.Append and
// persist it verbatim; they return the full set from
// SessionStore.ListSessionSummaries. Data is opaque SDK-owned state — stores
// MUST NOT interpret it.
type SessionSummaryEntry struct {
	SessionID string `json:"session_id"`
	// Mtime is the storage write time of the sidecar, in Unix epoch
	// milliseconds. Must use the same clock source as the Mtime returned
	// by SessionStore.ListSessions for this session — typically file mtime,
	// S3 LastModified, Postgres updated_at, or whatever native timestamp
	// the adapter surfaces. Do NOT derive this from entry ISO timestamps.
	// FoldSessionSummary preserves whatever Mtime the caller passes in via
	// prev and does not set it itself; stamp it after persisting.
	Mtime int64 `json:"mtime"`
	// Data is opaque SDK-owned summary state. Persist verbatim; do not interpret.
	Data map[string]any `json:"data"`
}

// SessionListSubkeysKey is the key argument to SessionStore.ListSubkeys
// (no Subpath).
type SessionListSubkeysKey struct {
	ProjectKey string `json:"project_key"`
	SessionID  string `json:"session_id"`
}

// SessionStoreFlushMode controls when transcript-mirror entries are flushed
// to a SessionStore.
type SessionStoreFlushMode string

const (
	// SessionStoreFlushBatched (default) coalesces entries and flushes once
	// per turn (on the result message) or when the pending buffer exceeds
	// 500 entries / 1 MiB. Keeps adapter latency off the streaming hot path.
	SessionStoreFlushBatched SessionStoreFlushMode = "batched"
	// SessionStoreFlushEager triggers a background flush after every
	// transcript_mirror frame so SessionStore.Append() sees entries in near
	// real time. Appends are still serialized in enqueue order; a slow
	// adapter will not stall the read loop but will see frames coalesced
	// while it is busy.
	SessionStoreFlushEager SessionStoreFlushMode = "eager"
)

// SessionStore is an adapter for mirroring session transcripts to external
// storage.
//
// The subprocess still writes to local disk (set CLAUDE_CONFIG_DIR=/tmp for
// an ephemeral local copy); the adapter receives a secondary copy.
//
// The SDK never deletes from your store unless you call DeleteSessionViaStore.
// Retention is the adapter's responsibility — implement TTL, object-storage
// lifecycle policies, or scheduled cleanup according to your compliance
// requirements (e.g. ZDR/HIPAA retention windows).
//
// Only Append and Load are required (the methods always invoked). The
// remaining methods are optional: implementers may return ErrSessionStoreNotImplemented,
// and call sites probe for it before invoking. Embed UnimplementedSessionStore
// for default unimplemented behavior on the optional methods.
type SessionStore interface {
	// Append mirrors a batch of transcript entries. Called AFTER the
	// subprocess's local write succeeds — durability is already guaranteed
	// locally.
	//
	// Batches arrive at ~100ms cadence during active turns. Entries are
	// JSON-safe plain objects — one per line in the local JSONL file.
	//
	// Most entries carry a stable "uuid" that adapters should treat as an
	// idempotency key (upsert / ignore-duplicate). Entries without a uuid
	// (e.g. titles, tags, mode markers) should be appended without dedup.
	// Failed batches are retried (3 attempts total) with short backoff
	// before being dropped and surfaced as a MirrorErrorMessage.
	Append(ctx context.Context, key SessionKey, entries []SessionStoreEntry) error

	// Load loads a full session for resume. Called once, in the SDK parent,
	// before subprocess spawn. The result is materialized to a temporary
	// JSONL file; the subprocess resumes from that file.
	//
	// Return (nil, nil) for a key that was never written; adapters that
	// cannot distinguish "never written" from "emptied" (e.g. Redis LRANGE)
	// may return (nil, nil) for both.
	Load(ctx context.Context, key SessionKey) ([]SessionStoreEntry, error)

	// ListSessions lists sessions for a ProjectKey. Returns IDs + mtimes.
	// Mtime is Unix epoch milliseconds. Result order is unspecified — the
	// SDK sorts by Mtime descending. Optional — return ErrSessionStoreNotImplemented
	// when not supported.
	ListSessions(ctx context.Context, projectKey string) ([]SessionStoreListEntry, error)

	// ListSessionSummaries returns incrementally-maintained summaries for
	// all sessions in one call. Stores should maintain these via
	// FoldSessionSummary inside Append. Skip the fold for keys with a
	// Subpath — subagent transcripts must not contribute to the main
	// session's summary. Optional.
	ListSessionSummaries(ctx context.Context, projectKey string) ([]SessionSummaryEntry, error)

	// Delete deletes a session. Deleting a main-transcript key (no Subpath)
	// must cascade to all subkeys under that session so subagent transcripts
	// aren't orphaned. A targeted delete with an explicit Subpath removes
	// only that one entry. Optional — if unimplemented (returns
	// ErrSessionStoreNotImplemented), DeleteSessionViaStore is a no-op.
	Delete(ctx context.Context, key SessionKey) error

	// ListSubkeys lists all subpath keys under a session (e.g. subagent
	// transcripts). Used during resume to discover and materialize all
	// subagent data. Optional — if unimplemented, resume only materializes
	// the main transcript.
	ListSubkeys(ctx context.Context, key SessionListSubkeysKey) ([]string, error)
}

// ErrSessionStoreNotImplemented is returned by SessionStore methods that an
// adapter doesn't support. Call sites check via errors.Is and treat the
// method as "absent". Equivalent to Python SDK's NotImplementedError raised
// from the Protocol's default methods.
type sessionStoreNotImplementedErr struct{}

func (sessionStoreNotImplementedErr) Error() string {
	return "session store method not implemented"
}

// ErrSessionStoreNotImplemented is the sentinel error to return from optional
// SessionStore methods you don't implement.
var ErrSessionStoreNotImplemented error = sessionStoreNotImplementedErr{}

// MirrorBatcher is the subset of TranscriptMirrorBatcher that transports
// need to receive transcript_mirror frames. Defined here so both transport
// packages and the public claudesdk package can refer to it without an
// import cycle. *sessions.TranscriptMirrorBatcher satisfies this.
type MirrorBatcher interface {
	Enqueue(filePath string, entries []SessionStoreEntry)
	Flush(ctx context.Context)
	Close(ctx context.Context)
}

// UnimplementedSessionStore embeds into a SessionStore implementation to get
// default ErrSessionStoreNotImplemented returns from the optional methods.
//
// Required methods (Append, Load) are NOT defaulted — your implementation
// must provide them explicitly.
type UnimplementedSessionStore struct{}

// ListSessions returns ErrSessionStoreNotImplemented by default.
func (UnimplementedSessionStore) ListSessions(ctx context.Context, projectKey string) ([]SessionStoreListEntry, error) {
	return nil, ErrSessionStoreNotImplemented
}

// ListSessionSummaries returns ErrSessionStoreNotImplemented by default.
func (UnimplementedSessionStore) ListSessionSummaries(ctx context.Context, projectKey string) ([]SessionSummaryEntry, error) {
	return nil, ErrSessionStoreNotImplemented
}

// Delete returns ErrSessionStoreNotImplemented by default.
func (UnimplementedSessionStore) Delete(ctx context.Context, key SessionKey) error {
	return ErrSessionStoreNotImplemented
}

// ListSubkeys returns ErrSessionStoreNotImplemented by default.
func (UnimplementedSessionStore) ListSubkeys(ctx context.Context, key SessionListSubkeysKey) ([]string, error) {
	return nil, ErrSessionStoreNotImplemented
}
