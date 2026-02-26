package claudesdk

import "testing"

func TestWithMcpConfig(t *testing.T) {
	t.Parallel()

	options := NewOptions(WithMcpConfig("/tmp/mcp.json"))
	if options.McpConfig == nil {
		t.Fatal("expected McpConfig to be set")
	}
	if got := *options.McpConfig; got != "/tmp/mcp.json" {
		t.Fatalf("unexpected McpConfig: %q", got)
	}
}

func TestWithMcpConfigEmptyClearsValue(t *testing.T) {
	t.Parallel()

	options := NewOptions(
		WithMcpConfig("/tmp/mcp.json"),
		WithMcpConfig(""),
	)
	if options.McpConfig != nil {
		t.Fatalf("expected McpConfig to be nil, got %q", *options.McpConfig)
	}
}

func TestWithFallbackModel(t *testing.T) {
	t.Parallel()

	options := NewOptions(WithFallbackModel("claude-sonnet-4-5"))
	if options.FallbackModel == nil {
		t.Fatal("expected FallbackModel to be set")
	}
	if got := *options.FallbackModel; got != "claude-sonnet-4-5" {
		t.Fatalf("unexpected FallbackModel: %q", got)
	}
}
