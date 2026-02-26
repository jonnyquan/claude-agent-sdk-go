package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestAddMCPFlagsIncludesSDKServerName(t *testing.T) {
	t.Parallel()

	options := shared.NewOptions()
	options.McpServers["sdkServer"] = &shared.McpSdkServerConfig{
		Type: shared.McpServerTypeSDK,
		Name: "calculator",
	}

	cmd := addMCPFlags([]string{"claude"}, options)
	if len(cmd) != 3 {
		t.Fatalf("expected 3 args, got %d (%v)", len(cmd), cmd)
	}
	if cmd[1] != "--mcp-config" {
		t.Fatalf("expected --mcp-config flag, got: %v", cmd)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(cmd[2]), &payload); err != nil {
		t.Fatalf("failed to parse mcp-config payload: %v", err)
	}

	servers, ok := payload["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers in payload: %#v", payload)
	}
	rawServer, ok := servers["sdkServer"].(map[string]any)
	if !ok {
		t.Fatalf("missing sdkServer payload: %#v", servers)
	}
	if got, _ := rawServer["type"].(string); got != "sdk" {
		t.Fatalf("expected sdk type, got: %#v", rawServer["type"])
	}
	if got, _ := rawServer["name"].(string); got != "calculator" {
		t.Fatalf("expected sdk server name 'calculator', got: %#v", rawServer["name"])
	}
}

func TestAddMCPFlagsOmitsEmptyStdioType(t *testing.T) {
	t.Parallel()

	options := shared.NewOptions()
	options.McpServers["legacy-stdio"] = &shared.McpStdioServerConfig{
		Command: "node",
		Args:    []string{"server.js"},
	}

	cmd := addMCPFlags([]string{"claude"}, options)
	if len(cmd) != 3 || cmd[1] != "--mcp-config" {
		t.Fatalf("expected --mcp-config flag, got: %v", cmd)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(cmd[2]), &payload); err != nil {
		t.Fatalf("failed to parse mcp-config payload: %v", err)
	}

	servers, ok := payload["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers in payload: %#v", payload)
	}
	rawServer, ok := servers["legacy-stdio"].(map[string]any)
	if !ok {
		t.Fatalf("missing legacy-stdio payload: %#v", servers)
	}
	if _, hasType := rawServer["type"]; hasType {
		t.Fatalf("expected stdio type to be omitted when empty, got: %#v", rawServer["type"])
	}
	if got, _ := rawServer["command"].(string); got != "node" {
		t.Fatalf("expected command 'node', got %#v", rawServer["command"])
	}
}

func TestBuildSettingsValueFallbacksToFileWhenJSONParseFails(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	// Use a brace-delimited path so it first goes through JSON parsing, then path fallback.
	settingsPath := "{settings}"
	filePath := filepath.Join(tempDir, settingsPath)
	if err := os.WriteFile(filePath, []byte(`{"permissions":{"allow":["Bash(ls:*)"]}}`), 0o600); err != nil {
		t.Fatalf("failed to write settings file: %v", err)
	}

	options := shared.NewOptions()
	options.Settings = &settingsPath
	options.Sandbox = &shared.SandboxSettings{Enabled: true}

	settingsValue := buildSettingsValue(options)
	if settingsValue == "" {
		t.Fatal("expected merged settings value")
	}

	var merged map[string]any
	if err := json.Unmarshal([]byte(settingsValue), &merged); err != nil {
		t.Fatalf("failed to parse merged settings: %v", err)
	}

	if _, ok := merged["permissions"].(map[string]any); !ok {
		t.Fatalf("expected original settings from file to be preserved, got %#v", merged)
	}
	sandbox, ok := merged["sandbox"].(map[string]any)
	if !ok {
		t.Fatalf("expected sandbox to be merged, got %#v", merged["sandbox"])
	}
	if enabled, _ := sandbox["enabled"].(bool); !enabled {
		t.Fatalf("expected sandbox.enabled=true, got %#v", sandbox["enabled"])
	}
}
