package subprocess

import (
	"strings"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// envForOptions returns the env slice that buildSubprocessEnv would compose.
// We exercise the function indirectly by reading the "what would Connect
// build" sequence; the test mirrors the real build_env logic by replicating
// the steps in transport.go's Connect (system env minus CLAUDECODE → default
// entrypoint → user env → SDK version).
//
// This is a unit-test on the precedence semantics, not a full Connect test.
func envForOptions(opts *shared.Options, entrypoint, sdkVersion string) map[string]string {
	t := New("/tmp/claude", opts, entrypoint, sdkVersion)
	// We can't execute the real Connect (needs a CLI), so we replicate its
	// env-building section here.
	env := []string{}
	// Skip system env in the test — only verify the SDK-controlled parts.
	env = append(env, "CLAUDE_CODE_ENTRYPOINT="+t.entrypoint)
	if t.options != nil && t.options.ExtraEnv != nil {
		for key, value := range t.options.ExtraEnv {
			env = append(env, key+"="+value)
		}
	}
	if t.sdkVersion != "" {
		env = append(env, "CLAUDE_AGENT_SDK_VERSION="+t.sdkVersion)
	}
	// Last-wins: convert to map.
	final := make(map[string]string, len(env))
	for _, kv := range env {
		idx := strings.Index(kv, "=")
		if idx > 0 {
			final[kv[:idx]] = kv[idx+1:]
		}
	}
	return final
}

func TestEnvPrecedence_UserCanOverrideEntrypoint(t *testing.T) {
	opts := &shared.Options{
		ExtraEnv: map[string]string{
			"CLAUDE_CODE_ENTRYPOINT": "custom-entrypoint",
		},
	}
	env := envForOptions(opts, "sdk-go", "0.1.77")
	if env["CLAUDE_CODE_ENTRYPOINT"] != "custom-entrypoint" {
		t.Errorf("user ExtraEnv should override CLAUDE_CODE_ENTRYPOINT default, got %q",
			env["CLAUDE_CODE_ENTRYPOINT"])
	}
	// SDK version is still SDK-controlled.
	if env["CLAUDE_AGENT_SDK_VERSION"] != "0.1.77" {
		t.Errorf("CLAUDE_AGENT_SDK_VERSION should remain SDK-controlled, got %q",
			env["CLAUDE_AGENT_SDK_VERSION"])
	}
}

func TestEnvPrecedence_SDKVersionOverridesUser(t *testing.T) {
	opts := &shared.Options{
		ExtraEnv: map[string]string{
			"CLAUDE_AGENT_SDK_VERSION": "user-cant-spoof-this",
		},
	}
	env := envForOptions(opts, "sdk-go", "0.1.77")
	if env["CLAUDE_AGENT_SDK_VERSION"] != "0.1.77" {
		t.Errorf("user CLAUDE_AGENT_SDK_VERSION should NOT override SDK version, got %q",
			env["CLAUDE_AGENT_SDK_VERSION"])
	}
}

func TestEnvPrecedence_DefaultEntrypointWhenNoOverride(t *testing.T) {
	opts := &shared.Options{}
	env := envForOptions(opts, "sdk-go", "0.1.77")
	if env["CLAUDE_CODE_ENTRYPOINT"] != "sdk-go" {
		t.Errorf("default entrypoint should be sdk-go, got %q",
			env["CLAUDE_CODE_ENTRYPOINT"])
	}
}
