//go:build windows

package subprocess

import (
	"fmt"
	"os/exec"
)

// applyUserOption is a no-op on Windows because Go's os/exec doesn't expose
// a portable way to spawn a child as another user the way POSIX does.
// Returns an error if the caller actually requested a user — that's a
// configuration mistake worth surfacing rather than silently ignoring.
func applyUserOption(_ *exec.Cmd, userOpt *string) error {
	if userOpt == nil || *userOpt == "" {
		return nil
	}
	return fmt.Errorf("WithUser is not supported on Windows")
}
