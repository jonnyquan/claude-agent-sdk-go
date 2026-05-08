//go:build !windows

package subprocess

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// applyUserOption sets the subprocess to run as the named user (uid/gid).
// Mirrors Python SDK's `user=options.user` argument to `anyio.open_process`.
func applyUserOption(cmd *exec.Cmd, userOpt *string) error {
	if userOpt == nil || *userOpt == "" {
		return nil
	}

	targetUser, err := user.Lookup(*userOpt)
	if err != nil {
		return fmt.Errorf("failed to lookup user %q: %w", *userOpt, err)
	}

	uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid uid %q for user %q: %w", targetUser.Uid, *userOpt, err)
	}
	gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid gid %q for user %q: %w", targetUser.Gid, *userOpt, err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}

	return nil
}
