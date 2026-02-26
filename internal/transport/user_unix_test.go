//go:build !windows

package transport

import (
	"os/exec"
	"os/user"
	"testing"
)

func TestApplyUserOptionCurrentUser(t *testing.T) {
	t.Parallel()

	current, err := user.Current()
	if err != nil {
		t.Fatalf("failed to lookup current user: %v", err)
	}

	cmd := exec.Command("echo", "ok")
	u := current.Username
	if err := applyUserOption(cmd, &u); err != nil {
		t.Fatalf("applyUserOption failed for current user: %v", err)
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Credential == nil {
		t.Fatal("expected process credentials to be set")
	}
}

func TestApplyUserOptionInvalidUser(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("echo", "ok")
	bad := "this-user-should-not-exist-claude-sdk-go"
	if err := applyUserOption(cmd, &bad); err == nil {
		t.Fatal("expected error for invalid user")
	}
}
