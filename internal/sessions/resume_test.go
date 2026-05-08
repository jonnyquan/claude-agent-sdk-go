package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// hangingStore is a minimal SessionStore stub whose Load blocks until
// ctx.Done. Used to verify that LoadTimeoutMs caps the wait.
type hangingStore struct {
	shared.UnimplementedSessionStore
}

func (hangingStore) Append(_ context.Context, _ shared.SessionKey, _ []shared.SessionStoreEntry) error {
	return nil
}

func (hangingStore) Load(ctx context.Context, _ shared.SessionKey) ([]shared.SessionStoreEntry, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestMaterializeResumeRespectsLoadTimeoutMs verifies that the
// MaterializeResumeSession call honors Options.LoadTimeoutMs as the upper
// bound on Load() (Python parity for _with_timeout in session_resume.py).
func TestMaterializeResumeRespectsLoadTimeoutMs(t *testing.T) {
	resume := "550e8400-e29b-41d4-a716-446655440000"
	cwd := t.TempDir()
	opts := &shared.Options{
		Cwd:           &cwd,
		Resume:        &resume,
		SessionStore:  hangingStore{},
		LoadTimeoutMs: 100, // small enough to ensure the test is fast
	}
	start := time.Now()
	_, err := MaterializeResumeSession(context.Background(), opts)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected MaterializeResumeSession to fail when Load hangs past LoadTimeoutMs")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errStringContains(err, "Load") {
		t.Logf("got error: %v", err)
	}
	// Should return well before any default 60-second timeout.
	if elapsed > 5*time.Second {
		t.Fatalf("LoadTimeoutMs not enforced: returned after %s", elapsed)
	}
}

// TestMaterializeResumeDefaultsLoadTimeoutMs verifies that LoadTimeoutMs=0
// uses the 60s default rather than expiring immediately.
func TestMaterializeResumeDefaultsLoadTimeoutMs(t *testing.T) {
	resume := "550e8400-e29b-41d4-a716-446655440000"
	cwd := t.TempDir()
	opts := &shared.Options{
		Cwd:          &cwd,
		Resume:       &resume,
		SessionStore: NewInMemorySessionStore(),
		// LoadTimeoutMs intentionally zero — default should be 60s, far
		// more than the in-memory store's instant Load.
	}
	got, err := MaterializeResumeSession(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil materialized resume for empty store, got %+v", got)
	}
}

func errStringContains(err error, sub string) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), sub)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
