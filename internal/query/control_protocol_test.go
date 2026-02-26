package query

import (
	"testing"
	"time"
)

func TestInitializeTimeoutDefaultsToSixtySeconds(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "")
	if got := initializeTimeout(); got != 60*time.Second {
		t.Fatalf("expected 60s default timeout, got %v", got)
	}
}

func TestInitializeTimeoutHonorsLargerEnvValue(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "120000")
	if got := initializeTimeout(); got != 120*time.Second {
		t.Fatalf("expected 120s timeout, got %v", got)
	}
}

func TestInitializeTimeoutClampsToMinimum(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT", "1000")
	if got := initializeTimeout(); got != 60*time.Second {
		t.Fatalf("expected clamped 60s timeout, got %v", got)
	}
}
