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

// TestStderrFlushesTrailingPartialLine covers the case where the CLI writes its
// last diagnostic without a trailing newline before stalling or dying — exactly
// the output the caller needs at that moment, so it must not be swallowed.
func TestStderrFlushesTrailingPartialLine(t *testing.T) {
	var received []string
	tr := &Transport{options: &shared.Options{Stderr: func(line string) {
		received = append(received, line)
	}}}

	tr.wg.Add(1)
	tr.readStderr(io.NopCloser(strings.NewReader("complete\nno trailing newline")))

	want := []string{"complete", "no trailing newline"}
	if strings.Join(received, "|") != strings.Join(want, "|") {
		t.Fatalf("received %v, want %v", received, want)
	}
}

// TestStderrBoundsNewlineLessProducer covers a producer that never emits a
// newline: the framer must deliver the output as partial lines at the buffer
// limit rather than buffering without bound or (as bufio.Scanner would)
// discarding it all with ErrTooLong.
func TestStderrBoundsNewlineLessProducer(t *testing.T) {
	limit := 128 * 1024 // above maxBufferSize's 64 KiB floor
	var received []string
	tr := &Transport{options: &shared.Options{
		Stderr:        func(line string) { received = append(received, line) },
		MaxBufferSize: &limit,
	}}

	// Two limits' worth of newline-free output, plus a tail.
	payload := strings.Repeat("x", 2*limit+10)

	tr.wg.Add(1)
	tr.readStderr(io.NopCloser(strings.NewReader(payload)))

	if len(received) < 2 {
		t.Fatalf("expected the output to be flushed as several partial lines, got %d", len(received))
	}
	var total int
	for _, line := range received {
		total += len(line)
		if len(line) > 2*limit {
			t.Fatalf("a single emitted chunk (%d bytes) exceeded the bound", len(line))
		}
	}
	// Nothing may be dropped.
	if total != len(payload) {
		t.Fatalf("emitted %d bytes, want %d", total, len(payload))
	}
}
