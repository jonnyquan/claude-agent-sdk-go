package discovery

import (
	"strings"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

func TestAddMCPFlagsWithRawConfig(t *testing.T) {
	t.Parallel()

	raw := "/tmp/mcp-config.json"
	options := shared.NewOptions()
	options.McpConfig = &raw

	cmd := addMCPFlags([]string{"claude"}, options)
	if len(cmd) != 3 {
		t.Fatalf("expected 3 args, got %d (%v)", len(cmd), cmd)
	}
	if cmd[1] != "--mcp-config" || cmd[2] != raw {
		t.Fatalf("unexpected mcp args: %v", cmd)
	}
}

func TestAddMCPFlagsRawConfigTakesPrecedence(t *testing.T) {
	t.Parallel()

	raw := `{"mcpServers":{"fromRaw":{"type":"stdio","command":"x"}}}`
	options := shared.NewOptions()
	options.McpConfig = &raw
	options.McpServers["fromMap"] = &shared.McpStdioServerConfig{
		Type:    shared.McpServerTypeStdio,
		Command: "node",
		Args:    []string{"server.js"},
	}

	cmd := addMCPFlags([]string{"claude"}, options)
	if len(cmd) != 3 {
		t.Fatalf("expected 3 args when raw config is set, got %d (%v)", len(cmd), cmd)
	}
	if cmd[2] != raw {
		t.Fatalf("expected raw config to be passed through, got: %s", cmd[2])
	}
	if strings.Contains(cmd[2], "fromMap") {
		t.Fatalf("expected map-based config to be ignored when raw config is set, got: %s", cmd[2])
	}
}
