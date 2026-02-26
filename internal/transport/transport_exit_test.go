package transport

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/parsing"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

func nonZeroExitCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", "exit 7")
	}
	return exec.Command("sh", "-c", "exit 7")
}

func zeroExitCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", "exit 0")
	}
	return exec.Command("sh", "-c", "exit 0")
}

func newTransportForHandleStdoutTest(t *testing.T, cmd *exec.Cmd) (*Transport, context.CancelFunc) {
	t.Helper()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() failed: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Transport{
		cmd:     cmd,
		stdout:  stdout,
		parser:  parsing.New(),
		msgChan: make(chan shared.Message, 4),
		errChan: make(chan error, 4),
		ctx:     ctx,
	}, cancel
}

func TestHandleStdoutReportsProcessErrorOnNonZeroExit(t *testing.T) {
	transport, cancel := newTransportForHandleStdoutTest(t, nonZeroExitCommand())
	defer cancel()

	done := make(chan struct{})
	transport.wg.Add(1)
	go func() {
		transport.handleStdout()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleStdout did not finish in time")
	}

	var gotErr error
	for err := range transport.errChan {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected non-zero exit to produce an error")
	}
	if !strings.Contains(gotErr.Error(), "exit code: 7") {
		t.Fatalf("expected exit code in error, got: %v", gotErr)
	}
	if !strings.Contains(gotErr.Error(), "Check stderr output for details") {
		t.Fatalf("expected stderr guidance in error, got: %v", gotErr)
	}
}

func TestHandleStdoutDoesNotReportErrorOnZeroExit(t *testing.T) {
	transport, cancel := newTransportForHandleStdoutTest(t, zeroExitCommand())
	defer cancel()

	done := make(chan struct{})
	transport.wg.Add(1)
	go func() {
		transport.handleStdout()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleStdout did not finish in time")
	}

	for err := range transport.errChan {
		if err != nil {
			t.Fatalf("expected no error for zero exit, got %v", err)
		}
	}
}
