package transport

import (
	"io"
	"strings"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestStderrCallbackPanicDoesNotTerminateLoop mirrors the Python SDK regression
// test for issue #929: a panic from the user's stderr callback must not kill the
// read loop. Previously a raise/panic would exit the loop and silently drop every
// subsequent stderr line for the rest of the session.
func TestStderrCallbackPanicDoesNotTerminateLoop(t *testing.T) {
	var received []string
	cb := func(line string) {
		received = append(received, line)
		if len(received) == 1 {
			panic("simulated handler failure")
		}
	}

	tr := &Transport{options: &shared.Options{Stderr: cb}}
	rc := io.NopCloser(strings.NewReader("line 1\nline 2\nline 3\n"))

	tr.wg.Add(1)
	tr.readStderr(rc)

	want := []string{"line 1", "line 2", "line 3"}
	if len(received) != len(want) {
		t.Fatalf("received %v, want %v", received, want)
	}
	for i := range want {
		if received[i] != want[i] {
			t.Fatalf("received %v, want %v", received, want)
		}
	}
}
