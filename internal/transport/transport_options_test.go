package transport

import (
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

func TestShouldDebugToStderr(t *testing.T) {
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
			name:    "nil extra args",
			options: &shared.Options{},
			want:    false,
		},
		{
			name: "debug flag absent",
			options: &shared.Options{
				ExtraArgs: map[string]*string{"other-flag": nil},
			},
			want: false,
		},
		{
			name: "debug flag present",
			options: &shared.Options{
				ExtraArgs: map[string]*string{"debug-to-stderr": nil},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldDebugToStderr(tc.options); got != tc.want {
				t.Fatalf("shouldDebugToStderr()=%v want %v", got, tc.want)
			}
		})
	}
}

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
			name:    "no callback no debug",
			options: &shared.Options{ExtraArgs: map[string]*string{}},
			want:    false,
		},
		{
			name: "with callback",
			options: &shared.Options{
				Stderr: func(string) {},
			},
			want: true,
		},
		{
			name: "with debug flag",
			options: &shared.Options{
				ExtraArgs: map[string]*string{"debug-to-stderr": nil},
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
