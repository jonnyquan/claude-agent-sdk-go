package sessions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// recordingStore wraps InMemorySessionStore to record append calls and
// optionally inject failures for retry tests.
type recordingStore struct {
	*InMemorySessionStore
	mu        sync.Mutex
	calls     int
	failTimes int
	failErr   error
}

func (r *recordingStore) Append(ctx context.Context, key shared.SessionKey, entries []shared.SessionStoreEntry) error {
	r.mu.Lock()
	r.calls++
	shouldFail := r.failTimes > 0
	if shouldFail {
		r.failTimes--
	}
	r.mu.Unlock()
	if shouldFail {
		return r.failErr
	}
	return r.InMemorySessionStore.Append(ctx, key, entries)
}

func (r *recordingStore) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func newRecording() *recordingStore {
	return &recordingStore{InMemorySessionStore: NewInMemorySessionStore()}
}

func TestMirrorBatcher_FlushAfterEnqueue(t *testing.T) {
	store := newRecording()
	tmp := t.TempDir()
	b := NewTranscriptMirrorBatcher(MirrorBatcherConfig{
		Store:       store,
		ProjectsDir: tmp,
	})

	// Use a real-looking file path so FilePathToSessionKey resolves.
	filePath := tmp + "/proj/sess.jsonl"
	b.Enqueue(filePath, []shared.SessionStoreEntry{{"type": "user", "uuid": "a"}})
	b.Enqueue(filePath, []shared.SessionStoreEntry{{"type": "user", "uuid": "b"}})

	b.Flush(context.Background())

	if got := store.callCount(); got != 1 {
		t.Fatalf("expected 1 coalesced Append call, got %d", got)
	}
	loaded, _ := store.Load(context.Background(), shared.SessionKey{ProjectKey: "proj", SessionID: "sess"})
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries appended, got %d", len(loaded))
	}
}

func TestMirrorBatcher_RetriesAdapterFailure(t *testing.T) {
	store := newRecording()
	store.failTimes = 1
	store.failErr = errors.New("transient")
	tmp := t.TempDir()
	b := NewTranscriptMirrorBatcher(MirrorBatcherConfig{
		Store:       store,
		ProjectsDir: tmp,
	})

	filePath := tmp + "/proj/sess.jsonl"
	b.Enqueue(filePath, []shared.SessionStoreEntry{{"type": "user", "uuid": "a"}})
	b.Flush(context.Background())

	if got := store.callCount(); got < 2 {
		t.Fatalf("expected at least 2 attempts (1 fail + 1 success), got %d", got)
	}
	loaded, _ := store.Load(context.Background(), shared.SessionKey{ProjectKey: "proj", SessionID: "sess"})
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry to be appended after retry, got %d", len(loaded))
	}
}

func TestMirrorBatcher_OnErrorCalledAfterAllRetriesFail(t *testing.T) {
	store := newRecording()
	store.failTimes = MirrorAppendMaxAttempts
	store.failErr = errors.New("permanent")
	tmp := t.TempDir()

	var (
		errKey *shared.SessionKey
		errMsg error
		errMu  sync.Mutex
	)
	b := NewTranscriptMirrorBatcher(MirrorBatcherConfig{
		Store:       store,
		ProjectsDir: tmp,
		OnError: func(_ context.Context, key *shared.SessionKey, err error) {
			errMu.Lock()
			errKey = key
			errMsg = err
			errMu.Unlock()
		},
	})
	b.Enqueue(tmp+"/proj/sess.jsonl", []shared.SessionStoreEntry{{"type": "user", "uuid": "a"}})
	b.Flush(context.Background())

	errMu.Lock()
	defer errMu.Unlock()
	if errKey == nil || errKey.SessionID != "sess" {
		t.Fatalf("expected OnError with SessionID=sess, got %v", errKey)
	}
	if errMsg == nil {
		t.Fatalf("expected OnError to receive a non-nil error")
	}
}

func TestMirrorBatcher_DropsFramesNotUnderProjectsDir(t *testing.T) {
	store := newRecording()
	b := NewTranscriptMirrorBatcher(MirrorBatcherConfig{
		Store:       store,
		ProjectsDir: t.TempDir(),
	})
	// Path well outside projectsDir; FilePathToSessionKey returns nil so
	// the frame should be dropped without raising.
	b.Enqueue("/totally/unrelated/path.jsonl", []shared.SessionStoreEntry{{"type": "user", "uuid": "a"}})
	b.Flush(context.Background())
	if got := store.callCount(); got != 0 {
		t.Fatalf("expected no Append for unrelated filePath, got %d", got)
	}
}

func TestMirrorBatcher_EagerFlushDeliversWithoutExplicitFlush(t *testing.T) {
	// Eager mode: each Enqueue schedules a background drain. Subsequent
	// Enqueues that arrive while a prior drain holds flushLock are
	// coalesced (matches the Python contract: "a slow adapter will not
	// stall the read loop but will see frames coalesced while it is busy").
	// We assert that the entries land in the store WITHOUT calling
	// Flush() — that's the eager-mode value proposition.
	store := newRecording()
	tmp := t.TempDir()
	b := NewTranscriptMirrorBatcher(MirrorBatcherConfig{
		Store:             store,
		ProjectsDir:       tmp,
		MaxPendingEntries: 0,
		MaxPendingBytes:   0,
	})
	filePath := tmp + "/proj/sess.jsonl"
	b.Enqueue(filePath, []shared.SessionStoreEntry{{"type": "user", "uuid": "a"}})
	b.Enqueue(filePath, []shared.SessionStoreEntry{{"type": "user", "uuid": "b"}})

	// Wait for the eager flush to deliver both entries.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		loaded, _ := store.Load(context.Background(), shared.SessionKey{ProjectKey: "proj", SessionID: "sess"})
		if len(loaded) >= 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	loaded, _ := store.Load(context.Background(), shared.SessionKey{ProjectKey: "proj", SessionID: "sess"})
	t.Fatalf("eager flush did not deliver both entries within deadline; got %d", len(loaded))
}
