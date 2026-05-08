// Package subprocess: parent-exit cleanup for live CLI subprocesses.
//
// Mirrors Python SDK's atexit handler — installs an os.Signal listener that
// terminates any registered Cmd when the parent process receives SIGINT or
// SIGTERM, preventing orphaned `claude` processes from leaking when callers
// crash or exit before awaiting Close().
package subprocess

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

var (
	activeMu       sync.Mutex
	activeChildren = make(map[*exec.Cmd]struct{})
	signalsOnce    sync.Once
)

// registerActiveChild adds cmd to the live-child set. The set is consulted
// when the parent receives SIGINT/SIGTERM and on a best-effort cleanup pass
// the SDK fires when the program exits normally.
func registerActiveChild(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	activeMu.Lock()
	activeChildren[cmd] = struct{}{}
	activeMu.Unlock()
	signalsOnce.Do(installSignalHandler)
}

// unregisterActiveChild removes cmd from the live-child set. Idempotent —
// safe to call from Close() regardless of whether the child has already
// exited.
func unregisterActiveChild(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	activeMu.Lock()
	delete(activeChildren, cmd)
	activeMu.Unlock()
}

// killActiveChildren sends SIGTERM to every registered cmd and clears the
// set. Best-effort — failures are swallowed so a single misbehaving child
// can't block other cleanup. Mirrors Python's _kill_active_children.
func killActiveChildren() {
	activeMu.Lock()
	cmds := make([]*exec.Cmd, 0, len(activeChildren))
	for c := range activeChildren {
		cmds = append(cmds, c)
	}
	activeChildren = make(map[*exec.Cmd]struct{})
	activeMu.Unlock()
	for _, c := range cmds {
		if c.Process != nil {
			_ = c.Process.Signal(syscall.SIGTERM)
		}
	}
}

func installSignalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		killActiveChildren()
		// Reset to default handlers so a second signal terminates as usual
		// (Python's atexit only fires once for clean exit; signal-driven
		// termination here parallels that).
		signal.Reset(os.Interrupt, syscall.SIGTERM)
	}()
}
