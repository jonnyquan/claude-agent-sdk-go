//go:build darwin

package sessions

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
)

// keychainServiceName matches the CLI's default macOS Keychain service name
// for OAuth credentials when CLAUDE_CONFIG_DIR is unset (production
// OAUTH_FILE_SUFFIX is empty). Mirrors Python SDK's _KEYCHAIN_SERVICE_NAME.
const keychainServiceName = "Claude Code-credentials"

// readKeychainCredentials reads OAuth credentials JSON from the macOS
// Keychain (default service name).
//
// Best-effort — returns nil on any error. Used by copyAuthFiles to populate
// the resumed subprocess's .credentials.json file when:
//   - the parent runs the default OAuth flow (no CLAUDE_CONFIG_DIR, no env
//     ANTHROPIC_API_KEY / CLAUDE_CODE_OAUTH_TOKEN)
//   - OAuth tokens live in the Keychain (not on disk)
//   - the resumed subprocess runs with a redirected CLAUDE_CONFIG_DIR so its
//     own Keychain lookup misses (different service-name suffix).
//
// Without this, Mac users on the default OAuth flow couldn't resume from a
// SessionStore-backed temp config dir.
func readKeychainCredentials() []byte {
	usr := os.Getenv("USER")
	if usr == "" {
		if u, err := user.Current(); err == nil {
			usr = u.Username
		}
	}
	if usr == "" {
		usr = "claude-code-user"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"security",
		"find-generic-password",
		"-a", usr,
		"-w",
		"-s", keychainServiceName,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return []byte(trimmed)
}
