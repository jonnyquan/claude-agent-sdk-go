package discovery

import (
	"context"
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

func TestAddAdvancedFlagsOmitsEmptySettingSources(t *testing.T) {
	t.Parallel()

	cmd := addAdvancedFlags([]string{"claude"}, shared.NewOptions())
	for _, arg := range cmd {
		if arg == "--setting-sources" {
			t.Fatalf("expected empty setting sources to be omitted, got %v", cmd)
		}
	}
}

func TestSessionFlagsBindDashLeadingValues(t *testing.T) {
	t.Parallel()

	// A dash-leading value must never be able to detach from its flag and be
	// parsed as an independent CLI flag: the CLI declares --resume with an
	// optional value, so only the =-joined form binds reliably.
	resume := "--dangerously-skip-permissions"
	sessionID := "-x"
	options := shared.NewOptions()
	options.Resume = &resume
	options.SessionID = &sessionID

	cmd := addSessionFlags([]string{"claude"}, options)

	if !containsArg(cmd, "--resume="+resume) {
		t.Errorf("expected --resume=%s as a single token, got %v", resume, cmd)
	}
	if !containsArg(cmd, "--session-id="+sessionID) {
		t.Errorf("expected --session-id=%s as a single token, got %v", sessionID, cmd)
	}
	if containsArg(cmd, resume) {
		t.Errorf("dash-leading value leaked as its own argv token: %v", cmd)
	}
}

func TestExtraFlagsBindDashLeadingValues(t *testing.T) {
	t.Parallel()

	dashValue := "-oops"
	plainValue := "plain"
	options := shared.NewOptions()
	options.ExtraArgs = map[string]*string{
		"dash":  &dashValue,
		"plain": &plainValue,
		"bare":  nil,
	}

	cmd := addExtraFlags(nil, options)

	if !containsArg(cmd, "--dash="+dashValue) {
		t.Errorf("expected --dash=%s as a single token, got %v", dashValue, cmd)
	}
	// A value that cannot be mistaken for a flag keeps the two-token form.
	if !containsArgPair(cmd, "--plain", plainValue) {
		t.Errorf("expected --plain %s as two tokens, got %v", plainValue, cmd)
	}
	if !containsArg(cmd, "--bare") {
		t.Errorf("expected boolean flag --bare, got %v", cmd)
	}
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func containsArgPair(args []string, flag, value string) bool {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestAdvancedFlagsForwardPreviouslyDroppedOptions(t *testing.T) {
	t.Parallel()

	options := shared.NewOptions()
	options.IncludeHookEvents = true
	options.StrictMcpConfig = true
	options.SessionStore = stubSessionStore{}

	cmd := addAdvancedFlags(nil, options)

	for _, flag := range []string{"--include-hook-events", "--strict-mcp-config", "--session-mirror"} {
		if !containsArg(cmd, flag) {
			t.Errorf("expected %s, got %v", flag, cmd)
		}
	}
}

func TestSettingSourcesUsesEqualsFormAndPreservesEmpty(t *testing.T) {
	t.Parallel()

	// An explicitly empty list must still reach the CLI: it means "disable
	// every setting source", which the two-token form cannot express.
	options := shared.NewOptions()
	options.SettingSources = []string{}
	if cmd := addAdvancedFlags(nil, options); !containsArg(cmd, "--setting-sources=") {
		t.Errorf("expected --setting-sources= for an empty list, got %v", cmd)
	}

	options = shared.NewOptions()
	options.SettingSources = []string{"user", "project"}
	if cmd := addAdvancedFlags(nil, options); !containsArg(cmd, "--setting-sources=user,project") {
		t.Errorf("expected --setting-sources=user,project, got %v", cmd)
	}

	// Unset and no Skills: the flag is not passed at all.
	options = shared.NewOptions()
	for _, arg := range addAdvancedFlags(nil, options) {
		if strings.HasPrefix(arg, "--setting-sources") {
			t.Errorf("expected no --setting-sources when unset, got %v", arg)
		}
	}
}

func TestSkillsInjectAllowedToolsAndSettingSourceDefaults(t *testing.T) {
	t.Parallel()

	// skills=all injects the bare Skill tool.
	options := shared.NewOptions()
	options.Skills = shared.SkillsAll()
	cmd := addToolControlFlags(nil, options)
	if !containsArgPair(cmd, "--allowedTools", "Skill") {
		t.Errorf("expected Skill in --allowedTools, got %v", cmd)
	}
	// ...and defaults setting sources so the CLI can discover installed skills.
	if !containsArg(addAdvancedFlags(nil, options), "--setting-sources=user,project") {
		t.Error("skills should default setting sources to user,project")
	}

	// An explicit list injects Skill(name) specifiers alongside existing tools.
	options = shared.NewOptions()
	options.AllowedTools = []string{"Read"}
	options.Skills = shared.SkillsList("deploy", "review")
	cmd = addToolControlFlags(nil, options)
	if !containsArgPair(cmd, "--allowedTools", "Read,Skill(deploy),Skill(review)") {
		t.Errorf("unexpected --allowedTools: %v", cmd)
	}

	// The caller's own SettingSources wins over the skills default.
	options.SettingSources = []string{"project"}
	if !containsArg(addAdvancedFlags(nil, options), "--setting-sources=project") {
		t.Error("explicit SettingSources should win over the skills default")
	}

	// No Skills configured: AllowedTools passes through untouched.
	options = shared.NewOptions()
	options.AllowedTools = []string{"Read"}
	if !containsArgPair(addToolControlFlags(nil, options), "--allowedTools", "Read") {
		t.Error("AllowedTools should pass through unchanged without Skills")
	}
}

func TestThinkingDisplayIsForwarded(t *testing.T) {
	t.Parallel()

	options := shared.NewOptions()
	options.Thinking = &shared.ThinkingConfig{
		Type:    shared.ThinkingTypeAdaptive,
		Display: shared.ThinkingDisplaySummarized,
	}
	if !containsArgPair(addModelAndPromptFlags(nil, options), "--thinking-display", string(shared.ThinkingDisplaySummarized)) {
		t.Error("expected --thinking-display to be forwarded")
	}

	// Not meaningful when thinking is disabled — there is nothing to display.
	options.Thinking = &shared.ThinkingConfig{
		Type:    shared.ThinkingTypeDisabled,
		Display: shared.ThinkingDisplaySummarized,
	}
	for _, arg := range addModelAndPromptFlags(nil, options) {
		if arg == "--thinking-display" {
			t.Error("--thinking-display should be omitted when thinking is disabled")
		}
	}
}

type stubSessionStore struct {
	shared.UnimplementedSessionStore
}

func (stubSessionStore) Append(context.Context, shared.SessionKey, []shared.SessionStoreEntry) error {
	return nil
}

func (stubSessionStore) Load(context.Context, shared.SessionKey) ([]shared.SessionStoreEntry, error) {
	return nil, nil
}
