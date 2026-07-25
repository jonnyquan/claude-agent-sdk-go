package shared

import "testing"

func taskStarted(id, taskType string) *TaskStartedMessage {
	return &TaskStartedMessage{TaskID: id, TaskType: &taskType}
}

func TestTaskLedgerTracksDeferringTypesOnly(t *testing.T) {
	var ledger TaskLedger

	// A background shell may never reach a terminal status, so tracking it
	// would withhold the stdin close indefinitely.
	ledger.Observe(taskStarted("shell-1", "background_shell"))
	if !ledger.RunEnded() {
		t.Error("background shell must not be tracked")
	}

	// A task_started with no task_type carries nothing to classify on.
	ledger.Observe(&TaskStartedMessage{TaskID: "unknown-1"})
	if !ledger.RunEnded() {
		t.Error("untyped task must not be tracked")
	}

	ledger.Observe(taskStarted("agent-1", "local_agent"))
	ledger.Observe(taskStarted("wf-1", "local_workflow"))
	if got := ledger.InFlight(); got != 2 {
		t.Fatalf("InFlight() = %d, want 2", got)
	}
	if ledger.RunEnded() {
		t.Error("a result arriving now ends the turn, not the run")
	}
}

func TestTaskLedgerClearsOnEitherTerminalFrame(t *testing.T) {
	var ledger TaskLedger
	ledger.Observe(taskStarted("agent-1", "local_agent"))
	ledger.Observe(taskStarted("agent-2", "local_agent"))

	// task_notification is one terminal vocabulary...
	ledger.Observe(&TaskNotificationMessage{TaskID: "agent-1", Status: TaskNotificationStatusCompleted})
	if got := ledger.InFlight(); got != 1 {
		t.Fatalf("InFlight() = %d, want 1", got)
	}

	// ...and a task_updated patch with a terminal status is the other. A task
	// stopped via TaskStop reports the raw "killed" here with no accompanying
	// notification.
	ledger.Observe(&TaskUpdatedMessage{TaskID: "agent-2", Status: TaskUpdatedStatusKilled})
	if !ledger.RunEnded() {
		t.Fatalf("InFlight() = %d, want 0", ledger.InFlight())
	}
}

func TestTaskLedgerIgnoresNonTerminalUpdatesAndIsIdempotent(t *testing.T) {
	var ledger TaskLedger
	ledger.Observe(taskStarted("agent-1", "local_agent"))

	ledger.Observe(&TaskUpdatedMessage{TaskID: "agent-1", Status: TaskUpdatedStatusRunning})
	if ledger.RunEnded() {
		t.Error("a running patch must not clear the task")
	}
	// A patch carrying only end_time/result (no status) stays non-terminal.
	ledger.Observe(&TaskUpdatedMessage{TaskID: "agent-1", Patch: map[string]any{"end_time": 1}})
	if ledger.RunEnded() {
		t.Error("a statusless patch must not clear the task")
	}

	// Both terminal frames can arrive for the same task; clearing twice is a
	// no-op, and clearing an unknown id must not corrupt the ledger.
	ledger.Observe(&TaskNotificationMessage{TaskID: "agent-1", Status: TaskNotificationStatusCompleted})
	ledger.Observe(&TaskUpdatedMessage{TaskID: "agent-1", Status: TaskUpdatedStatusCompleted})
	ledger.Observe(&TaskNotificationMessage{TaskID: "never-started"})
	if got := ledger.InFlight(); got != 0 {
		t.Fatalf("InFlight() = %d, want 0", got)
	}
}
