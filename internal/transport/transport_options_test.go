package transport

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestShouldPipeStderr verifies stderr piping is gated solely on whether
// a stderr callback is registered (Python SDK 0.1.65+ parity, fix #860 —
// the prior `--debug-to-stderr` extra-arg fallback was removed).
func TestShouldPipeStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options *shared.Options
		want    bool
	}{
		{
			name:    "nil options",
			options: nil,
			want:    false,
		},
		{
			name:    "no callback",
			options: &shared.Options{},
			want:    false,
		},
		{
			name:    "no callback even with debug-to-stderr extra arg",
			options: &shared.Options{ExtraArgs: map[string]*string{"debug-to-stderr": nil}},
			want:    false,
		},
		{
			name: "with callback",
			options: &shared.Options{
				Stderr: func(string) {},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldPipeStderr(tc.options); got != tc.want {
				t.Fatalf("shouldPipeStderr()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestShouldCreateHookProcessor(t *testing.T) {
	t.Parallel()

	allow := func(string, map[string]any, shared.ToolPermissionContext) (shared.PermissionResult, error) {
		return shared.NewPermissionAllow(nil, nil), nil
	}

	tests := []struct {
		name    string
		options *shared.Options
		want    bool
	}{
		{
			name:    "nil options",
			options: nil,
			want:    false,
		},
		{
			name:    "no hooks no can_use_tool",
			options: &shared.Options{},
			want:    false,
		},
		{
			name: "hooks configured",
			options: &shared.Options{
				Hooks: map[string][]any{
					"PreToolUse": {},
				},
			},
			want: true,
		},
		{
			name: "can_use_tool configured",
			options: &shared.Options{
				CanUseTool: allow,
			},
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldCreateHookProcessor(tc.options); got != tc.want {
				t.Fatalf("shouldCreateHookProcessor()=%v want %v", got, tc.want)
			}
		})
	}
}
