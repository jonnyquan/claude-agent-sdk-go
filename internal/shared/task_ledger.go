package shared

import "sync"

// DeferringTaskTypes lists the task types whose completion runs a follow-up
// turn, and which therefore may still need the control channel after the turn's
// result frame.
//
// This mirrors the set the CLI itself holds a result back for, which is narrower
// than its notion of "delegated agent work". The types left out are left out on
// purpose, and none of them is merely an oversight:
//   - background shells and monitors run indefinitely by design, so deferring
//     the close on one withholds it forever rather than briefly;
//   - teammates are long-lived too — their status stays running for their whole
//     lifetime, so they never settle the ledger;
//   - remote agents can be long-running monitors the CLI likewise refuses to
//     wait on.
//
// Anything added here must be a type that reliably reaches a terminal status, or
// it will stall the stdin close until the stream-close timeout expires.
var DeferringTaskTypes = map[string]struct{}{
	"local_agent":    {},
	"local_workflow": {},
}

// TaskLedger tracks started-but-not-finished delegated tasks so a result frame
// can be told apart from the end of the run.
//
// A result frame ends one turn, not the run: background tasks keep running past
// it and still need stdin for hook / SDK-MCP control responses. Closing stdin
// then silently disables hooks and fails SDK-MCP calls with "Stream closed".
// Each task completion wakes the parent for a follow-up turn, so a later result
// frame arrives with no tasks in flight and closes stdin then.
//
// This is a mitigation, not a complete answer. An empty ledger means "nothing we
// know of is running", which is not the same as "the run is over": a task that
// settles *before* the turn's result frame leaves the ledger empty at that
// result, so stdin closes even though the completion may still wake the parent
// for a continuation turn. No ledger can close that gap, because it cannot
// distinguish a settled task whose continuation is pending from no work at all —
// that needs a run-boundary signal from the CLI. What this does fix is the
// common ordering, where the task outlives the turn that spawned it.
//
// The background_tasks_changed frame is deliberately not consumed, in either
// direction. Its payload is the live *background* set, while a subagent is
// registered in the foreground and only flips to backgrounded later, without a
// second task_started. So the snapshot omits tracked work that is still running:
// narrowing against it would drop an agent that goes on to outlive its turn,
// which is the very close-too-early bug this exists to prevent. Widening from it
// is no better — the snapshot spans every background task type and carries
// nothing marking an observer agent, whose start and terminal frames are both
// suppressed, so it could admit an id no later frame ever clears.
//
// The zero value is ready to use and is safe for concurrent use.
type TaskLedger struct {
	mu       sync.Mutex
	inFlight map[string]struct{}
}

// Observe updates the ledger from a parsed message. Messages that are not task
// lifecycle frames are ignored, so callers can feed it the whole stream.
//
// task_started marks a task in flight; task_notification or a task_updated patch
// with a terminal status clears it. Terminal completion can arrive as either
// frame (not every terminal task emits a notification), so both are handled, and
// clearing an unknown id is a no-op so the pair stays idempotent.
func (l *TaskLedger) Observe(msg Message) {
	switch m := msg.(type) {
	case *TaskStartedMessage:
		if m.TaskID == "" || m.TaskType == nil {
			return
		}
		if _, deferring := DeferringTaskTypes[*m.TaskType]; !deferring {
			return
		}
		l.mu.Lock()
		if l.inFlight == nil {
			l.inFlight = make(map[string]struct{})
		}
		l.inFlight[m.TaskID] = struct{}{}
		l.mu.Unlock()
	case *TaskNotificationMessage:
		l.clear(m.TaskID)
	case *TaskUpdatedMessage:
		if IsTerminalTaskStatus(string(m.Status)) {
			l.clear(m.TaskID)
		}
	}
}

func (l *TaskLedger) clear(taskID string) {
	if taskID == "" {
		return
	}
	l.mu.Lock()
	delete(l.inFlight, taskID)
	l.mu.Unlock()
}

// InFlight returns the number of tracked tasks that have not reached a terminal
// status.
func (l *TaskLedger) InFlight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.inFlight)
}

// RunEnded reports whether a result frame observed right now ends the run rather
// than just the current turn.
func (l *TaskLedger) RunEnded() bool {
	return l.InFlight() == 0
}
