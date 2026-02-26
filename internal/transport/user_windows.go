//go:build windows

package transport

import (
	"fmt"
	"os/exec"
)

func applyUserOption(_ *exec.Cmd, userOpt *string) error {
	if userOpt == nil || *userOpt == "" {
		return nil
	}
	return fmt.Errorf("WithUser is not supported on Windows")
}
