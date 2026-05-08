package sessions

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestInMemorySessionStoreConformance runs the conformance suite against the
// reference InMemorySessionStore. If this fails, the conformance harness or
// the in-memory store has a bug — both are SDK-owned, so a green run here
// also serves as a regression test for the harness itself.
func TestInMemorySessionStoreConformance(t *testing.T) {
	RunSessionStoreConformance(t, func() shared.SessionStore {
		return NewInMemorySessionStore()
	}, ConformanceOptions{})
}
