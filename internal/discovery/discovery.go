// Package discovery provides CLI discovery and command building functionality.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// MinimumCLIVersion is the minimum required version of Claude Code CLI.
const MinimumCLIVersion = "2.0.0"

const windowsOS = "windows"

// DiscoveryPaths defines the standard search paths for Claude CLI.
var DiscoveryPaths = []string{
	// Will be populated with dynamic paths in FindCLI()
}

// FindCLI searches for the Claude CLI binary in standard locations.
func FindCLI() (string, error) {
	// 0. Check bundled CLI first
	if bundledPath := findBundledCLI(); bundledPath != "" {
		return bundledPath, nil
	}

	// 1. Check PATH first - most common case
	if path, err := exec.LookPath("claude"); err == nil {
		// Check version (warning only, don't fail)
		if verErr := CheckCLIVersion(path); verErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", verErr)
		}
		return path, nil
	}

	// 2. Check platform-specific common locations
	locations := getCommonCLILocations()

	for _, location := range locations {
		if info, err := os.Stat(location); err == nil && !info.IsDir() {
			// Verify it's executable (Unix-like systems)
			if runtime.GOOS != windowsOS {
				if info.Mode()&0o111 == 0 {
					continue // Not executable
				}
			}
			// Check version (warning only, don't fail)
			if verErr := CheckCLIVersion(location); verErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", verErr)
			}
			return location, nil
		}
	}

	// 3. Check Node.js dependency
	if _, err := exec.LookPath("node"); err != nil {
		return "", shared.NewCLINotFoundError("",
			"Claude Code requires Node.js, which is not installed.\n\n"+
				"Install Node.js from: https://nodejs.org/\n\n"+
				"After installing Node.js, install Claude Code:\n"+
				"  npm install -g @anthropic-ai/claude-code")
	}

	// 4. Provide installation guidance
	return "", shared.NewCLINotFoundError("",
		"Claude Code not found. Install with:\n"+
			"  npm install -g @anthropic-ai/claude-code\n\n"+
			"If already installed locally, try:\n"+
			`  export PATH="$HOME/node_modules/.bin:$PATH"`+"\n\n"+
			"Or specify the path when creating client")
}

// getCommonCLILocations returns platform-specific CLI search locations
func getCommonCLILocations() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		homeDir = "."
	}

	var locations []string

	switch runtime.GOOS {
	case windowsOS:
		locations = []string{
			filepath.Join(homeDir, "AppData", "Roaming", "npm", "claude.cmd"),
			filepath.Join("C:", "Program Files", "nodejs", "claude.cmd"),
			filepath.Join(homeDir, ".npm-global", "claude.cmd"),
			filepath.Join(homeDir, "node_modules", ".bin", "claude.cmd"),
		}
	default: // Unix-like systems
		locations = []string{
			filepath.Join(homeDir, ".npm-global", "bin", "claude"),
			"/usr/local/bin/claude",
			filepath.Join(homeDir, ".local", "bin", "claude"),
			filepath.Join(homeDir, "node_modules", ".bin", "claude"),
			filepath.Join(homeDir, ".yarn", "bin", "claude"),
			filepath.Join(homeDir, ".claude", "local", "claude"),
			"/opt/homebrew/bin/claude",       // macOS Homebrew ARM
			"/usr/local/homebrew/bin/claude", // macOS Homebrew Intel
		}
	}

	return locations
}

// BuildCommand constructs the CLI command with all necessary flags.
// Always uses streaming mode (--input-format stream-json) for bidirectional communication.
func BuildCommand(cliPath string, options *shared.Options) []string {
	cmd := []string{cliPath}

	// Base arguments - always use streaming mode
	cmd = append(cmd, "--output-format", "stream-json", "--verbose")
	cmd = append(cmd, "--input-format", "stream-json")

	// Add all configuration options as CLI flags
	if options != nil {
		cmd = addOptionsToCommand(cmd, options)
	}

	return cmd
}

// BuildCommandWithPrompt constructs the CLI command for one-shot queries.
// Uses streaming mode; prompt is sent via stdin after initialize.
func BuildCommandWithPrompt(cliPath string, options *shared.Options) []string {
	cmd := []string{cliPath}

	// Base arguments - always use streaming mode
	cmd = append(cmd, "--output-format", "stream-json", "--verbose")
	cmd = append(cmd, "--input-format", "stream-json")

	// Add all configuration options as CLI flags
	if options != nil {
		cmd = addOptionsToCommand(cmd, options)
	}

	return cmd
}

// addOptionsToCommand adds all Options fields as CLI flags
func addOptionsToCommand(cmd []string, options *shared.Options) []string {
	cmd = addToolControlFlags(cmd, options)
	cmd = addModelAndPromptFlags(cmd, options)
	cmd = addPermissionFlags(cmd, options)
	cmd = addSessionFlags(cmd, options)
	cmd = addFileSystemFlags(cmd, options)
	cmd = addMCPFlags(cmd, options)
	cmd = addAdvancedFlags(cmd, options)
	cmd = addExtraFlags(cmd, options)
	return cmd
}

func addToolControlFlags(cmd []string, options *shared.Options) []string {
	// Handle tools option (base set of tools)
	if options.Tools != nil {
		switch tools := options.Tools.(type) {
		case []string:
			if len(tools) == 0 {
				cmd = append(cmd, "--tools", "")
			} else {
				cmd = append(cmd, "--tools", strings.Join(tools, ","))
			}
		case shared.ToolsPreset:
			// Preset: 'claude_code' preset maps to 'default'
			cmd = append(cmd, "--tools", "default")
		case *shared.ToolsPreset:
			cmd = append(cmd, "--tools", "default")
		default:
			// Treat as preset
			cmd = append(cmd, "--tools", "default")
		}
	}

	if len(options.AllowedTools) > 0 {
		cmd = append(cmd, "--allowedTools", strings.Join(options.AllowedTools, ","))
	}
	if len(options.DisallowedTools) > 0 {
		cmd = append(cmd, "--disallowedTools", strings.Join(options.DisallowedTools, ","))
	}
	return cmd
}

func addModelAndPromptFlags(cmd []string, options *shared.Options) []string {
	switch sp := options.SystemPrompt.(type) {
	case nil:
		// When no system prompt is specified, explicitly pass empty string
		// to give users full control over agent behavior
		cmd = append(cmd, "--system-prompt", "")
	case string:
		cmd = append(cmd, "--system-prompt", sp)
	case *string:
		if sp != nil {
			cmd = append(cmd, "--system-prompt", *sp)
		} else {
			cmd = append(cmd, "--system-prompt", "")
		}
	case shared.SystemPromptPreset:
		// Preset: only pass --append-system-prompt if "append" is set
		if sp.Append != nil {
			cmd = append(cmd, "--append-system-prompt", *sp.Append)
		}
	case *shared.SystemPromptPreset:
		if sp != nil && sp.Append != nil {
			cmd = append(cmd, "--append-system-prompt", *sp.Append)
		}
	}
	if options.AppendSystemPrompt != nil {
		cmd = append(cmd, "--append-system-prompt", *options.AppendSystemPrompt)
	}
	if options.Model != nil {
		cmd = append(cmd, "--model", *options.Model)
	}
	if options.FallbackModel != nil {
		cmd = append(cmd, "--fallback-model", *options.FallbackModel)
	}
	// Resolve thinking config → --max-thinking-tokens
	// `Thinking` takes precedence over the deprecated `MaxThinkingTokens`
	var resolvedMaxThinkingTokens *int
	if options.MaxThinkingTokens != nil {
		v := *options.MaxThinkingTokens
		resolvedMaxThinkingTokens = &v
	}
	if options.Thinking != nil {
		switch options.Thinking.Type {
		case shared.ThinkingTypeAdaptive:
			if resolvedMaxThinkingTokens == nil {
				v := 32000
				resolvedMaxThinkingTokens = &v
			}
		case shared.ThinkingTypeEnabled:
			v := options.Thinking.BudgetTokens
			resolvedMaxThinkingTokens = &v
		case shared.ThinkingTypeDisabled:
			v := 0
			resolvedMaxThinkingTokens = &v
		}
	}
	if resolvedMaxThinkingTokens != nil {
		cmd = append(cmd, "--max-thinking-tokens", fmt.Sprintf("%d", *resolvedMaxThinkingTokens))
	}

	// Add effort flag
	if options.Effort != nil {
		cmd = append(cmd, "--effort", string(*options.Effort))
	}

	// Add betas flag
	if len(options.Betas) > 0 {
		betaStrs := make([]string, len(options.Betas))
		for i, b := range options.Betas {
			betaStrs[i] = string(b)
		}
		cmd = append(cmd, "--betas", strings.Join(betaStrs, ","))
	}

	// Handle OutputFormat for structured outputs
	if options.OutputFormat != nil {
		if outputType, ok := options.OutputFormat["type"].(string); ok && outputType == "json_schema" {
			if schema, ok := options.OutputFormat["schema"]; ok {
				schemaBytes, err := json.Marshal(schema)
				if err == nil {
					cmd = append(cmd, "--json-schema", string(schemaBytes))
				}
			}
		}
	}

	return cmd
}

func addPermissionFlags(cmd []string, options *shared.Options) []string {
	if options.PermissionMode != nil {
		cmd = append(cmd, "--permission-mode", string(*options.PermissionMode))
	}
	if options.PermissionPromptToolName != nil {
		cmd = append(cmd, "--permission-prompt-tool", *options.PermissionPromptToolName)
	}
	return cmd
}

func addSessionFlags(cmd []string, options *shared.Options) []string {
	if options.ContinueConversation {
		cmd = append(cmd, "--continue")
	}
	if options.Resume != nil {
		cmd = append(cmd, "--resume", *options.Resume)
	}
	if options.MaxTurns > 0 {
		cmd = append(cmd, "--max-turns", fmt.Sprintf("%d", options.MaxTurns))
	}
	if options.MaxBudgetUSD != nil {
		cmd = append(cmd, "--max-budget-usd", strconv.FormatFloat(*options.MaxBudgetUSD, 'f', -1, 64))
	}
	// Handle settings and sandbox: merge sandbox into settings if both are provided
	settingsValue := buildSettingsValue(options)
	if settingsValue != "" {
		cmd = append(cmd, "--settings", settingsValue)
	}

	return cmd
}

// buildSettingsValue builds the settings value, merging sandbox settings if provided.
func buildSettingsValue(options *shared.Options) string {
	hasSettings := options.Settings != nil
	hasSandbox := options.Sandbox != nil

	if !hasSettings && !hasSandbox {
		return ""
	}

	// If only settings path and no sandbox, pass through as-is
	if hasSettings && !hasSandbox {
		return *options.Settings
	}

	// If we have sandbox settings, we need to merge into a JSON object
	settingsObj := make(map[string]interface{})

	if hasSettings {
		settingsStr := strings.TrimSpace(*options.Settings)
		if strings.HasPrefix(settingsStr, "{") && strings.HasSuffix(settingsStr, "}") {
			if err := json.Unmarshal([]byte(settingsStr), &settingsObj); err != nil {
				// Match Python behavior: when JSON parsing fails for brace-delimited
				// input, retry as file path.
				fmt.Fprintf(
					os.Stderr,
					"Warning: Failed to parse settings as JSON, treating as file path: %s\n",
					settingsStr,
				)
				if data, readErr := os.ReadFile(settingsStr); readErr == nil {
					_ = json.Unmarshal(data, &settingsObj)
				}
			}
		} else {
			// It's a file path - read and parse
			data, err := os.ReadFile(settingsStr)
			if err == nil {
				_ = json.Unmarshal(data, &settingsObj)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Settings file not found: %s\n", settingsStr)
			}
		}
	}

	// Merge sandbox settings
	if hasSandbox {
		settingsObj["sandbox"] = options.Sandbox
	}

	data, err := json.Marshal(settingsObj)
	if err != nil {
		return ""
	}

	return string(data)
}

func addFileSystemFlags(cmd []string, options *shared.Options) []string {
	// Note: Cwd is set via cmd.Dir in transport.go, not as a CLI flag
	// The --cwd flag is not supported by Claude CLI
	// if options.Cwd != nil {
	//     cmd = append(cmd, "--cwd", *options.Cwd)
	// }
	for _, dir := range options.AddDirs {
		cmd = append(cmd, "--add-dir", dir)
	}
	return cmd
}

func addMCPFlags(cmd []string, options *shared.Options) []string {
	if options == nil {
		return cmd
	}

	// Raw MCP config takes precedence (parity with Python mcp_servers as string/path).
	if options.McpConfig != nil && *options.McpConfig != "" {
		cmd = append(cmd, "--mcp-config", *options.McpConfig)
		return cmd
	}

	if len(options.McpServers) == 0 {
		return cmd
	}

	servers := make(map[string]map[string]interface{})
	for name, cfg := range options.McpServers {
		if cfg == nil {
			continue
		}

		switch server := cfg.(type) {
		case *shared.McpStdioServerConfig:
			payload := map[string]interface{}{
				"command": server.Command,
			}
			if server.Type != "" {
				payload["type"] = string(server.Type)
			}
			if len(server.Args) > 0 {
				payload["args"] = server.Args
			}
			if len(server.Env) > 0 {
				payload["env"] = server.Env
			}
			servers[name] = payload
		case *shared.McpSSEServerConfig:
			payload := map[string]interface{}{
				"type": string(server.Type),
				"url":  server.URL,
			}
			if len(server.Headers) > 0 {
				payload["headers"] = server.Headers
			}
			servers[name] = payload
		case *shared.McpHTTPServerConfig:
			payload := map[string]interface{}{
				"type": string(server.Type),
				"url":  server.URL,
			}
			if len(server.Headers) > 0 {
				payload["headers"] = server.Headers
			}
			servers[name] = payload
		case *shared.McpSdkServerConfig:
			// For SDK servers, pass type and name (instance is not serializable).
			// Matches Python behavior, which forwards all fields except "instance".
			payload := map[string]interface{}{
				"type": string(shared.McpServerTypeSDK),
			}
			if server.Name != "" {
				payload["name"] = server.Name
			}
			servers[name] = payload
		default:
			continue
		}
	}

	if len(servers) == 0 {
		return cmd
	}

	payload := map[string]interface{}{"mcpServers": servers}
	data, err := json.Marshal(payload)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, "--mcp-config", string(data))
	return cmd
}

func addAdvancedFlags(cmd []string, options *shared.Options) []string {
	if options.IncludePartialMessages {
		cmd = append(cmd, "--include-partial-messages")
	}
	if options.ForkSession {
		cmd = append(cmd, "--fork-session")
	}
	// Always emit --setting-sources (empty string if not set, matching Python SDK)
	sourcesValue := ""
	if len(options.SettingSources) > 0 {
		sourcesValue = strings.Join(options.SettingSources, ",")
	}
	cmd = append(cmd, "--setting-sources", sourcesValue)

	// Note: Agents are now sent via initialize request, not as CLI flag

	// Add plugin directories
	if len(options.Plugins) > 0 {
		for _, plugin := range options.Plugins {
			if plugin.Type == shared.PluginTypeLocal {
				cmd = append(cmd, "--plugin-dir", plugin.Path)
			}
		}
	}

	return cmd
}

func addExtraFlags(cmd []string, options *shared.Options) []string {
	for flag, value := range options.ExtraArgs {
		if value == nil {
			// Boolean flag
			cmd = append(cmd, "--"+flag)
		} else {
			// Flag with value
			cmd = append(cmd, "--"+flag, *value)
		}
	}
	return cmd
}

// ValidateNodeJS checks if Node.js is available.
func ValidateNodeJS() error {
	if _, err := exec.LookPath("node"); err != nil {
		return shared.NewCLINotFoundError("node",
			"Node.js is required for Claude CLI but was not found.\n\n"+
				"Install Node.js from: https://nodejs.org/\n\n"+
				"After installing Node.js, install Claude Code:\n"+
				"  npm install -g @anthropic-ai/claude-code")
	}
	return nil
}

// ValidateWorkingDirectory checks if the working directory exists and is valid.
func ValidateWorkingDirectory(cwd string) error {
	if cwd == "" {
		return nil // No validation needed if no cwd specified
	}

	info, err := os.Stat(cwd)
	if os.IsNotExist(err) {
		return shared.NewConnectionError(
			fmt.Sprintf("working directory does not exist: %s", cwd),
			err,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}

	if !info.IsDir() {
		return shared.NewConnectionError(
			fmt.Sprintf("working directory path is not a directory: %s", cwd),
			nil,
		)
	}

	return nil
}

// DetectCLIVersion detects the Claude CLI version for compatibility checks.
func DetectCLIVersion(ctx context.Context, cliPath string) (string, error) {
	cmd := exec.CommandContext(ctx, cliPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get CLI version: %w", err)
	}

	version := strings.TrimSpace(string(output))

	// Basic version format validation
	if !strings.Contains(version, ".") {
		return "", fmt.Errorf("invalid version format: %s", version)
	}

	return version, nil
}

// CheckCLIVersion checks if the Claude Code CLI version meets the minimum requirement.
func CheckCLIVersion(cliPath string) error {
	// Allow skipping version check via environment variable
	if os.Getenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK") != "" {
		return nil
	}

	cmd := exec.Command(cliPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check CLI version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	// Version format: "claude-code/2.0.0" or just "2.0.0"
	version = strings.TrimPrefix(version, "claude-code/")
	version = strings.TrimPrefix(version, "v")

	if !isVersionSufficient(version, MinimumCLIVersion) {
		return fmt.Errorf(
			"Claude Code CLI version %s is below minimum required version %s. "+
				"Please update:\n  npm install -g @anthropic-ai/claude-code",
			version,
			MinimumCLIVersion,
		)
	}

	return nil
}

// isVersionSufficient checks if current version >= required version.
// Versions are expected in format "major.minor.patch".
func isVersionSufficient(current, required string) bool {
	currentParts := parseVersion(current)
	requiredParts := parseVersion(required)

	for i := 0; i < 3; i++ {
		if currentParts[i] > requiredParts[i] {
			return true
		}
		if currentParts[i] < requiredParts[i] {
			return false
		}
	}

	return true // Equal versions are sufficient
}

// parseVersion parses a semantic version string into [major, minor, patch].
func parseVersion(version string) [3]int {
	parts := strings.Split(version, ".")
	var result [3]int

	for i := 0; i < 3 && i < len(parts); i++ {
		// Extract numeric part only (handle cases like "2.0.0-beta")
		numStr := strings.Split(parts[i], "-")[0]
		if num, err := strconv.Atoi(numStr); err == nil {
			result[i] = num
		}
	}

	return result
}

// findBundledCLI searches for a bundled CLI binary in the SDK package.
func findBundledCLI() string {
	// Determine the CLI binary name based on platform
	var cliName string
	if runtime.GOOS == windowsOS {
		cliName = "claude.exe"
	} else {
		cliName = "claude"
	}

	// Get the path to the bundled CLI relative to this package
	// We need to find the path to the _bundled directory from the SDK root
	// The _bundled directory should be at the same level as this package
	// or can be determined from the module path
	bundledPaths := []string{
		// Path relative to SDK root when embedded in binary
		filepath.Join("_bundled", cliName),
		// Path relative to this file's location (for development/testing)
		filepath.Join("..", "..", "..", "_bundled", cliName),
		// Additional fallback paths
		filepath.Join("claude-agent-sdk-go", "_bundled", cliName),
	}

	for _, bundledPath := range bundledPaths {
		if info, err := os.Stat(bundledPath); err == nil && !info.IsDir() {
			// Verify it's executable (Unix-like systems)
			if runtime.GOOS != windowsOS {
				if info.Mode()&0o111 == 0 {
					continue // Not executable
				}
			}
			fmt.Fprintf(os.Stderr, "Using bundled Claude CLI: %s\n", bundledPath)
			return bundledPath
		}
	}

	return ""
}
