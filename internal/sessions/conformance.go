// Package sessions: shared conformance test suite for SessionStore adapters.
//
// Call RunSessionStoreConformance from a Go test to assert the 14 behavioral
// contracts every adapter must satisfy. Tests for optional methods
// (ListSessions, ListSessionSummaries, Delete, ListSubkeys) are skipped when
// the adapter returns ErrSessionStoreNotImplemented from the corresponding
// method or when the test author opts out via SkipOptional.
//
// This mirrors Python SDK's claude_agent_sdk.testing.run_session_store_conformance.
//
// Example:
//
//	import (
//	    "testing"
//	    "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
//	)
//
//	func TestMyStoreConformance(t *testing.T) {
//	    claudesdk.RunSessionStoreConformance(t, func() claudesdk.SessionStore {
//	        return NewMyRedisStore()
//	    }, claudesdk.ConformanceOptions{})
//	}
package sessions

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// ConformanceOptions configures RunSessionStoreConformance.
type ConformanceOptions struct {
	// SkipOptional names optional methods to skip even if the adapter
	// implements them. Valid values: "ListSessions",
	// "ListSessionSummaries", "Delete", "ListSubkeys".
	SkipOptional []string
}

// RunSessionStoreConformance asserts the 14 SessionStore behavioral
// contracts. makeStore is invoked once per contract to provide isolation.
func RunSessionStoreConformance(
	t *testing.T,
	makeStore func() shared.SessionStore,
	opts ConformanceOptions,
) {
	t.Helper()
	ctx := context.Background()
	skipSet := map[string]struct{}{}
	for _, name := range opts.SkipOptional {
		skipSet[name] = struct{}{}
	}

	probe := makeStore()
	hasListSessions := implementsOptional(ctx, probe, "ListSessions", skipSet)
	hasListSummaries := implementsOptional(ctx, probe, "ListSessionSummaries", skipSet)
	hasDelete := implementsOptional(ctx, probe, "Delete", skipSet)
	hasListSubkeys := implementsOptional(ctx, probe, "ListSubkeys", skipSet)

	key := shared.SessionKey{ProjectKey: "proj", SessionID: "sess"}

	// 1. Append then Load returns same entries in same order.
	t.Run("AppendLoadRoundtrip", func(t *testing.T) {
		s := makeStore()
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{
			testEntry("uuid", "b", "n", 1),
			testEntry("uuid", "a", "n", 2),
		}))
		loaded, err := s.Load(ctx, key)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(loaded) != 2 || loaded[0]["uuid"] != "b" || loaded[1]["uuid"] != "a" {
			t.Fatalf("expected [{b}, {a}], got %v", loaded)
		}
	})

	// 2. Load unknown key returns nil.
	t.Run("LoadUnknownKeyNil", func(t *testing.T) {
		s := makeStore()
		got, err := s.Load(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "nope"})
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "x")}))
		subKey := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "nope"}
		got, _ = s.Load(ctx, subKey)
		if got != nil {
			t.Fatalf("expected nil for unknown subpath, got %v", got)
		}
	})

	// 3. Multiple Append calls preserve call order.
	t.Run("AppendOrderPreserved", func(t *testing.T) {
		s := makeStore()
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "z")}))
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "a"), testEntry("uuid", "m")}))
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "b")}))
		loaded, _ := s.Load(ctx, key)
		ids := []string{}
		for _, e := range loaded {
			ids = append(ids, e["uuid"].(string))
		}
		if !reflect.DeepEqual(ids, []string{"z", "a", "m", "b"}) {
			t.Fatalf("expected [z a m b], got %v", ids)
		}
	})

	// 4. Append([]) is a no-op.
	t.Run("EmptyAppendNoOp", func(t *testing.T) {
		s := makeStore()
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "a")}))
		mustAppend(t, s.Append(ctx, key, nil))
		loaded, _ := s.Load(ctx, key)
		if len(loaded) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(loaded))
		}
	})

	// 5. Subpath keys are stored independently of main.
	t.Run("SubpathIsolation", func(t *testing.T) {
		s := makeStore()
		sub := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-1"}
		mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("uuid", "m")}))
		mustAppend(t, s.Append(ctx, sub, []shared.SessionStoreEntry{testEntry("uuid", "s")}))
		main, _ := s.Load(ctx, key)
		subData, _ := s.Load(ctx, sub)
		if len(main) != 1 || main[0]["uuid"] != "m" {
			t.Fatalf("main load mismatch: %v", main)
		}
		if len(subData) != 1 || subData[0]["uuid"] != "s" {
			t.Fatalf("sub load mismatch: %v", subData)
		}
	})

	// 6. project_key isolation.
	t.Run("ProjectKeyIsolation", func(t *testing.T) {
		s := makeStore()
		ka := shared.SessionKey{ProjectKey: "A", SessionID: "s1"}
		kb := shared.SessionKey{ProjectKey: "B", SessionID: "s1"}
		mustAppend(t, s.Append(ctx, ka, []shared.SessionStoreEntry{testEntry("from", "A")}))
		mustAppend(t, s.Append(ctx, kb, []shared.SessionStoreEntry{testEntry("from", "B")}))
		la, _ := s.Load(ctx, ka)
		lb, _ := s.Load(ctx, kb)
		if la[0]["from"] != "A" || lb[0]["from"] != "B" {
			t.Fatalf("project_key isolation broken: A=%v B=%v", la, lb)
		}
		if hasListSessions {
			lsA, _ := s.ListSessions(ctx, "A")
			lsB, _ := s.ListSessions(ctx, "B")
			if len(lsA) != 1 || len(lsB) != 1 {
				t.Fatalf("ListSessions per-project mismatch")
			}
		}
	})

	if hasListSessions {
		// 7. ListSessions returns session_ids for project; mtime is epoch-ms.
		t.Run("ListSessions", func(t *testing.T) {
			s := makeStore()
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "a"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "b"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "other", SessionID: "c"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			sessions, err := s.ListSessions(ctx, "proj")
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			ids := []string{}
			for _, e := range sessions {
				ids = append(ids, e.SessionID)
				// epoch-ms; >1e12 rules out epoch-seconds (≈2001 in ms).
				if e.Mtime <= 1e12 {
					t.Fatalf("mtime not epoch-ms: %d", e.Mtime)
				}
			}
			sort.Strings(ids)
			if !reflect.DeepEqual(ids, []string{"a", "b"}) {
				t.Fatalf("expected [a b], got %v", ids)
			}
			empty, _ := s.ListSessions(ctx, "never-appended-project")
			if len(empty) != 0 {
				t.Fatalf("expected empty list for unknown project, got %d", len(empty))
			}
		})

		// 8. ListSessions excludes subagent subpaths.
		t.Run("ListSessionsExcludesSubpaths", func(t *testing.T) {
			s := makeStore()
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "main"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "main", Subpath: "subagents/agent-1"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			sessions, _ := s.ListSessions(ctx, "proj")
			if len(sessions) != 1 || sessions[0].SessionID != "main" {
				t.Fatalf("ListSessions should only include main: %v", sessions)
			}
		})
	}

	if hasListSummaries {
		// 14. ListSessionSummaries returns persisted fold output that
		// round-trips through FoldSessionSummary again.
		t.Run("ListSessionSummariesRoundtrip", func(t *testing.T) {
			s := makeStore()
			summKey := shared.SessionKey{ProjectKey: "proj", SessionID: "summ-sess"}
			mustAppend(t, s.Append(ctx, summKey, []shared.SessionStoreEntry{
				testEntry("timestamp", "2024-01-01T00:00:00.000Z", "customTitle", "first"),
				testEntry("timestamp", "2024-01-01T00:00:01.000Z"),
			}))
			mustAppend(t, s.Append(ctx, summKey, []shared.SessionStoreEntry{
				testEntry("timestamp", "2024-01-01T00:00:02.000Z", "customTitle", "second"),
			}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: "other", SessionID: "elsewhere"}, []shared.SessionStoreEntry{
				testEntry("timestamp", "2024-01-01T00:00:00.000Z"),
			}))
			summaries, _ := s.ListSessionSummaries(ctx, "proj")
			byID := map[string]shared.SessionSummaryEntry{}
			for _, sm := range summaries {
				byID[sm.SessionID] = sm
			}
			if len(byID) != 1 || byID["summ-sess"].SessionID == "" {
				t.Fatalf("expected one summary for proj, got %d", len(byID))
			}
			summ := byID["summ-sess"]
			if summ.Mtime <= 1e12 {
				t.Fatalf("summary mtime not epoch-ms: %d", summ.Mtime)
			}
			if hasListSessions {
				ls, _ := s.ListSessions(ctx, "proj")
				lsByID := map[string]int64{}
				for _, e := range ls {
					lsByID[e.SessionID] = e.Mtime
				}
				if summ.Mtime < lsByID["summ-sess"] {
					t.Fatalf("summary mtime older than list_sessions mtime: defeats fast-path freshness check")
				}
			}
			refolded := FoldSessionSummary(&summ, summKey, []shared.SessionStoreEntry{testEntry("timestamp", "2024-01-01T00:00:03.000Z")})
			if refolded.SessionID != "summ-sess" {
				t.Fatalf("re-fold returned wrong session_id: %s", refolded.SessionID)
			}
			if refolded.Mtime != summ.Mtime {
				t.Fatalf("FoldSessionSummary should preserve prev.Mtime verbatim, got %d (was %d)", refolded.Mtime, summ.Mtime)
			}
			// Subagent appends must NOT affect the main session's summary.
			subKey := shared.SessionKey{ProjectKey: summKey.ProjectKey, SessionID: summKey.SessionID, Subpath: "subagents/agent-1"}
			mustAppend(t, s.Append(ctx, subKey, []shared.SessionStoreEntry{
				testEntry("timestamp", "2024-01-01T00:00:09.000Z", "customTitle", "subagent"),
			}))
			afterSub, _ := s.ListSessionSummaries(ctx, "proj")
			afterByID := map[string]shared.SessionSummaryEntry{}
			for _, sm := range afterSub {
				afterByID[sm.SessionID] = sm
			}
			if !reflect.DeepEqual(afterByID["summ-sess"].Data, summ.Data) {
				t.Fatalf("subagent append changed main summary data")
			}
			empty, _ := s.ListSessionSummaries(ctx, "never-appended-project")
			if len(empty) != 0 {
				t.Fatalf("expected empty summaries for unknown project, got %d", len(empty))
			}
			if hasDelete {
				_ = s.Delete(ctx, summKey)
				remaining, _ := s.ListSessionSummaries(ctx, "proj")
				if len(remaining) != 0 {
					t.Fatalf("Delete should remove summary; remaining=%d", len(remaining))
				}
			}
		})
	}

	if hasDelete {
		// 9. Delete main then Load returns nil.
		t.Run("DeleteMainNilLoad", func(t *testing.T) {
			s := makeStore()
			_ = s.Delete(ctx, shared.SessionKey{ProjectKey: "proj", SessionID: "never-written"})
			mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("n", 1)}))
			if err := s.Delete(ctx, key); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			loaded, _ := s.Load(ctx, key)
			if loaded != nil {
				t.Fatalf("expected nil after Delete, got %v", loaded)
			}
		})

		// 10. Delete main cascades to subkeys.
		t.Run("DeleteCascades", func(t *testing.T) {
			s := makeStore()
			sub1 := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-1"}
			sub2 := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-2"}
			other := shared.SessionKey{ProjectKey: "proj", SessionID: "sess2"}
			otherProj := shared.SessionKey{ProjectKey: "other-proj", SessionID: key.SessionID}
			mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, sub1, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, sub2, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, other, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, otherProj, []shared.SessionStoreEntry{testEntry("n", 1)}))

			_ = s.Delete(ctx, key)
			if v, _ := s.Load(ctx, key); v != nil {
				t.Fatalf("Delete should nil out main")
			}
			if v, _ := s.Load(ctx, sub1); v != nil {
				t.Fatalf("Delete should cascade to sub1")
			}
			if v, _ := s.Load(ctx, sub2); v != nil {
				t.Fatalf("Delete should cascade to sub2")
			}
			if v, _ := s.Load(ctx, other); v == nil {
				t.Fatalf("Delete should not affect sibling sess2")
			}
			if v, _ := s.Load(ctx, otherProj); v == nil {
				t.Fatalf("Delete should not affect other project")
			}
		})

		// 11. Delete with subpath removes only that subkey.
		t.Run("DeleteSubpathOnly", func(t *testing.T) {
			s := makeStore()
			sub1 := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-1"}
			sub2 := shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-2"}
			mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, sub1, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, sub2, []shared.SessionStoreEntry{testEntry("n", 1)}))

			_ = s.Delete(ctx, sub1)
			if v, _ := s.Load(ctx, sub1); v != nil {
				t.Fatalf("Delete sub1 failed")
			}
			if v, _ := s.Load(ctx, sub2); v == nil {
				t.Fatalf("Delete sub1 should not affect sub2")
			}
			if v, _ := s.Load(ctx, key); v == nil {
				t.Fatalf("Delete sub1 should not affect main")
			}
		})
	}

	if hasListSubkeys {
		// 12. ListSubkeys returns subpaths.
		t.Run("ListSubkeys", func(t *testing.T) {
			s := makeStore()
			mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-1"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID, Subpath: "subagents/agent-2"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			mustAppend(t, s.Append(ctx, shared.SessionKey{ProjectKey: key.ProjectKey, SessionID: "other-sess", Subpath: "subagents/agent-x"}, []shared.SessionStoreEntry{testEntry("n", 1)}))
			subkeys, _ := s.ListSubkeys(ctx, shared.SessionListSubkeysKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID})
			sort.Strings(subkeys)
			if !reflect.DeepEqual(subkeys, []string{"subagents/agent-1", "subagents/agent-2"}) {
				t.Fatalf("expected [agent-1 agent-2], got %v", subkeys)
			}
		})

		// 13. ListSubkeys excludes main transcript and returns empty for unknown.
		t.Run("ListSubkeysExcludesMain", func(t *testing.T) {
			s := makeStore()
			mustAppend(t, s.Append(ctx, key, []shared.SessionStoreEntry{testEntry("n", 1)}))
			subkeys, _ := s.ListSubkeys(ctx, shared.SessionListSubkeysKey{ProjectKey: key.ProjectKey, SessionID: key.SessionID})
			if len(subkeys) != 0 {
				t.Fatalf("expected empty subkeys for session with no subagents, got %v", subkeys)
			}
			empty, _ := s.ListSubkeys(ctx, shared.SessionListSubkeysKey{ProjectKey: "proj", SessionID: "never-appended"})
			if len(empty) != 0 {
				t.Fatalf("expected empty subkeys for unknown session, got %v", empty)
			}
		})
	}
}

// implementsOptional probes whether store actually implements an optional
// method by performing a cheap call and inspecting the error.
func implementsOptional(ctx context.Context, store shared.SessionStore, method string, skip map[string]struct{}) bool {
	if _, ok := skip[method]; ok {
		return false
	}
	probeKey := shared.SessionKey{ProjectKey: "__conformance_probe__", SessionID: "probe"}
	var err error
	switch method {
	case "ListSessions":
		_, err = store.ListSessions(ctx, "__conformance_probe__")
	case "ListSessionSummaries":
		_, err = store.ListSessionSummaries(ctx, "__conformance_probe__")
	case "Delete":
		err = store.Delete(ctx, probeKey)
	case "ListSubkeys":
		_, err = store.ListSubkeys(ctx, shared.SessionListSubkeysKey{ProjectKey: "__conformance_probe__", SessionID: "probe"})
	default:
		return false
	}
	return !errors.Is(err, shared.ErrSessionStoreNotImplemented)
}

// testEntry builds a SessionStoreEntry from alternating key/value pairs. The
// "type" field is auto-injected so adapters that key on it never see a
// missing one.
func testEntry(kv ...any) shared.SessionStoreEntry {
	entry := shared.SessionStoreEntry{"type": "x"}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		entry[key] = kv[i+1]
	}
	return entry
}

func mustAppend(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
}
