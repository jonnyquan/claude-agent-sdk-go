// Package sessions: batching layer between transcript_mirror stdout frames
// and a SessionStore. Mirrors Python SDK's TranscriptMirrorBatcher.
//
// The CLI subprocess emits frames of the form
// {"type":"transcript_mirror","filePath":...,"entries":[...]} interleaved
// with normal SDK messages. The receive loop peels these off and hands them
// to TranscriptMirrorBatcher.Enqueue, which accumulates them and flushes to
// SessionStore.Append either when a result message arrives (explicit flush)
// or when the pending buffer exceeds size thresholds (eager background
// flush). This keeps adapter latency off the hot path during model
// streaming.
package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// Eager-flush thresholds. Exported for tests.
const (
	MirrorMaxPendingEntries = 500
	MirrorMaxPendingBytes   = 1 << 20 // 1 MiB
	MirrorSendTimeout       = 60 * time.Second

	// Bounded retry for transient adapter failures. Backoff list length
	// must be MirrorAppendMaxAttempts - 1 (one delay between each pair).
	MirrorAppendMaxAttempts = 3
)

var mirrorAppendBackoff = []time.Duration{
	200 * time.Millisecond,
	800 * time.Millisecond,
}

// MirrorErrorCallback is invoked when a flush fails after exhausting retries.
// Mirrors Python SDK's on_error callback. Implementations should be
// non-blocking — a slow callback delays subsequent drains.
type MirrorErrorCallback func(ctx context.Context, key *shared.SessionKey, err error)

// mirrorPendingEntry buffers a single enqueued frame.
type mirrorPendingEntry struct {
	filePath string
	entries  []shared.SessionStoreEntry
	bytes    int
}

// TranscriptMirrorBatcher accumulates transcript_mirror frames and flushes
// them to a SessionStore.
//
// Enqueue is fire-and-forget; Flush is synchronous. The pending queue is
// bounded — when it exceeds MaxPendingEntries or MaxPendingBytes an eager
// flush fires in the background so memory stays flat during long turns
// where no result (and thus no explicit Flush) arrives.
//
// Adapter failures are retried (MirrorAppendMaxAttempts attempts total)
// with short backoff; timeouts are not retried since the in-flight call
// may still land. Only after the final attempt fails is the batch dropped
// and reported via OnError. Failures never abort the session — the
// local-disk transcript is already durable.
type TranscriptMirrorBatcher struct {
	store              shared.SessionStore
	projectsDir        string
	onError            MirrorErrorCallback
	sendTimeout        time.Duration
	maxPendingEntries  int
	maxPendingBytes    int

	mu             sync.Mutex
	pending        []mirrorPendingEntry
	pendingEntries int
	pendingBytes   int

	// flushLock serializes flushes so append ordering holds across
	// concurrent enqueue-triggered drains and explicit Flush calls.
	flushLock sync.Mutex

	// inFlight signals that a background drain is running. callers of Flush
	// wait on it via waitInFlight.
	inFlightWG sync.WaitGroup

	closed atomic_bool
}

// atomic_bool is a tiny, dependency-free atomic flag.
type atomic_bool struct {
	mu sync.RWMutex
	v  bool
}

func (a *atomic_bool) Set(v bool) { a.mu.Lock(); a.v = v; a.mu.Unlock() }
func (a *atomic_bool) Get() bool  { a.mu.RLock(); defer a.mu.RUnlock(); return a.v }

// MirrorBatcherConfig configures TranscriptMirrorBatcher behavior.
type MirrorBatcherConfig struct {
	Store       shared.SessionStore
	ProjectsDir string
	OnError     MirrorErrorCallback

	// SendTimeout bounds each Append() call. Zero means MirrorSendTimeout.
	SendTimeout time.Duration

	// MaxPendingEntries: 0 disables entry-count thresholding. Negative
	// values use the default. Equivalent of Python's max_pending_entries=0
	// for eager mode.
	MaxPendingEntries int

	// MaxPendingBytes: 0 disables byte thresholding. Negative values use
	// the default.
	MaxPendingBytes int
}

// NewTranscriptMirrorBatcher constructs a TranscriptMirrorBatcher. ProjectsDir
// must be the absolute path that file_path → SessionKey resolution uses
// (typically the materialized config_dir + "/projects" or
// ~/.claude/projects).
func NewTranscriptMirrorBatcher(cfg MirrorBatcherConfig) *TranscriptMirrorBatcher {
	timeout := cfg.SendTimeout
	if timeout <= 0 {
		timeout = MirrorSendTimeout
	}
	maxEntries := cfg.MaxPendingEntries
	if maxEntries < 0 {
		maxEntries = MirrorMaxPendingEntries
	}
	maxBytes := cfg.MaxPendingBytes
	if maxBytes < 0 {
		maxBytes = MirrorMaxPendingBytes
	}
	return &TranscriptMirrorBatcher{
		store:             cfg.Store,
		projectsDir:       cfg.ProjectsDir,
		onError:           cfg.OnError,
		sendTimeout:       timeout,
		maxPendingEntries: maxEntries,
		maxPendingBytes:   maxBytes,
	}
}

// Enqueue buffers a frame; schedules an eager flush if thresholds are exceeded.
//
// Fire-and-forget — never blocks the caller. ProjectsDir must already be
// configured at construction time.
func (b *TranscriptMirrorBatcher) Enqueue(filePath string, entries []shared.SessionStoreEntry) {
	if b == nil || b.closed.Get() {
		return
	}
	// Approximate wire size — one stringify per frame (not per entry) keeps
	// this cheap relative to the json.Decode the transport already did.
	encoded, err := json.Marshal(entries)
	size := 0
	if err == nil {
		size = len(encoded)
	}

	b.mu.Lock()
	b.pending = append(b.pending, mirrorPendingEntry{
		filePath: filePath,
		entries:  entries,
		bytes:    size,
	})
	b.pendingEntries += len(entries)
	b.pendingBytes += size
	overflow := (b.maxPendingEntries > 0 && b.pendingEntries > b.maxPendingEntries) ||
		(b.maxPendingBytes > 0 && b.pendingBytes > b.maxPendingBytes) ||
		// Eager mode (zero thresholds): flush after every enqueue.
		(b.maxPendingEntries == 0 && b.maxPendingBytes == 0)
	b.mu.Unlock()

	if overflow {
		b.inFlightWG.Add(1)
		go func() {
			defer b.inFlightWG.Done()
			b.drain(context.Background())
		}()
	}
}

// Flush flushes all pending entries. Awaits any in-flight eager flush first
// so callers observe a fully-flushed state on return.
func (b *TranscriptMirrorBatcher) Flush(ctx context.Context) {
	if b == nil {
		return
	}
	// Wait for any in-flight eager flushes scheduled by Enqueue.
	b.inFlightWG.Wait()
	b.drain(ctx)
}

// Close performs a final flush before teardown. Safe to call multiple times;
// subsequent Enqueue calls become no-ops.
func (b *TranscriptMirrorBatcher) Close(ctx context.Context) {
	if b == nil {
		return
	}
	b.closed.Set(true)
	b.Flush(ctx)
}

// drain detaches the pending buffer, awaits any prior flush, then sends.
// Never returns an error — adapter and OnError callback errors are caught
// and logged.
func (b *TranscriptMirrorBatcher) drain(ctx context.Context) {
	b.mu.Lock()
	items := b.pending
	b.pending = nil
	b.pendingEntries = 0
	b.pendingBytes = 0
	b.mu.Unlock()
	if len(items) == 0 {
		return
	}

	type errReport struct {
		key *shared.SessionKey
		err error
	}
	var errors []errReport

	b.flushLock.Lock()
	b.doFlush(ctx, items, func(key *shared.SessionKey, err error) {
		errors = append(errors, errReport{key: key, err: err})
	})
	b.flushLock.Unlock()

	// Report errors after releasing the lock so a slow OnError callback
	// cannot block subsequent drains (which only need the lock for
	// append-ordering).
	if b.onError != nil {
		for _, e := range errors {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[TranscriptMirrorBatcher] OnError panicked: %v", r)
					}
				}()
				b.onError(ctx, e.key, e.err)
			}()
		}
	}
}

func (b *TranscriptMirrorBatcher) doFlush(
	ctx context.Context,
	items []mirrorPendingEntry,
	report func(key *shared.SessionKey, err error),
) {
	// Coalesce by file_path so each unique file gets one Append per flush
	// instead of one per enqueued frame. Map ordering doesn't matter — within
	// a path entries keep enqueue order.
	type bucket struct {
		filePath string
		entries  []shared.SessionStoreEntry
	}
	order := []string{}
	by := map[string]*bucket{}
	for _, item := range items {
		bk, ok := by[item.filePath]
		if !ok {
			bk = &bucket{filePath: item.filePath}
			by[item.filePath] = bk
			order = append(order, item.filePath)
		}
		bk.entries = append(bk.entries, item.entries...)
	}

	for _, path := range order {
		bk := by[path]
		if len(bk.entries) == 0 {
			// Avoid creating phantom keys in adapters that touch storage on
			// Append([]) — nothing to write.
			continue
		}
		key := FilePathToSessionKey(bk.filePath, b.projectsDir)
		if key == nil {
			log.Printf(
				"[SessionStore] dropping mirror frame: filePath %s is not under %s "+
					"(subprocess CLAUDE_CONFIG_DIR likely differs from parent)",
				bk.filePath, b.projectsDir,
			)
			continue
		}
		var lastErr error
		succeeded := false
		for attempt := 0; attempt < MirrorAppendMaxAttempts; attempt++ {
			if attempt > 0 {
				select {
				case <-time.After(mirrorAppendBackoff[attempt-1]):
				case <-ctx.Done():
					lastErr = ctx.Err()
					break
				}
			}
			callCtx, cancel := context.WithTimeout(ctx, b.sendTimeout)
			err := b.store.Append(callCtx, *key, bk.entries)
			cancel()
			if err == nil {
				succeeded = true
				break
			}
			lastErr = err
			// Don't retry on context-deadline / timeout cancellation: the
			// in-flight call may still land — a retry would launch a
			// concurrent duplicate. Also keeps worst-case lock hold at
			// ~SendTimeout rather than ~3×SendTimeout + backoff.
			if errors.Is(err, context.DeadlineExceeded) {
				log.Printf(
					"[TranscriptMirrorBatcher] append timed out after %s for %s — not retrying",
					b.sendTimeout, bk.filePath,
				)
				break
			}
			log.Printf(
				"[TranscriptMirrorBatcher] append attempt %d/%d failed for %s: %v",
				attempt+1, MirrorAppendMaxAttempts, bk.filePath, err,
			)
		}
		if !succeeded {
			log.Printf(
				"[TranscriptMirrorBatcher] flush failed for %s: %v",
				bk.filePath, lastErr,
			)
			report(key, lastErr)
		}
	}
}
