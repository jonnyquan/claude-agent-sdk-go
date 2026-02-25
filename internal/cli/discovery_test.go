package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// TestCLIDiscovery tests CLI binary discovery functionality
func TestCLIDiscovery(t *testing.T) {
	tests := []struct {
		name          string
		setupEnv      func(t *testing.T) (cleanup func())
		expectError   bool
		errorContains string
	}{
		{
			name:          "cli_not_found_error",
			setupEnv:      setupIsolatedEnvironment,
			expectError:   true,
			errorContains: "install",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleanup := test.setupEnv(t)
			defer cleanup()

			_, err := FindCLI()
			assertCLIDiscoveryError(t, err, test.expectError, test.errorContains)
		})
	}
}

// TestCommandBuilding tests CLI command construction with various options
func TestCommandBuilding(t *testing.T) {
	tests := []struct {
		name     string
		cliPath  string
		options  *shared.Options
		validate func(*testing.T, []string)
	}{
		{
			name:     "basic_streaming_command",
			cliPath:  "/usr/local/bin/claude",
			options:  &shared.Options{},
			validate: validateStreamingCommand,
		},
		{
			name:     "all_options_command",
			cliPath:  "/usr/local/bin/claude",
			options:  createFullOptionsSet(),
			validate: validateFullOptionsCommand,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := BuildCommand(test.cliPath, test.options)
			test.validate(t, cmd)
		})
	}
}

// TestCLIDiscoveryLocations tests CLI discovery path generation
func TestCLIDiscoveryLocations(t *testing.T) {
	locations := getCommonCLILocations()

	assertDiscoveryLocations(t, locations)
	assertPlatformSpecificPaths(t, locations)
}

// TestNodeJSDependencyValidation tests Node.js validation
func TestNodeJSDependencyValidation(t *testing.T) {
	err := ValidateNodeJS()
	assertNodeJSValidation(t, err)
}

// TestExtraArgsSupport tests arbitrary CLI flag support
func TestExtraArgsSupport(t *testing.T) {
	tests := []struct {
		name      string
		extraArgs map[string]*string
		validate  func(*testing.T, []string)
	}{
		{
			name:      "boolean_flags",
			extraArgs: map[string]*string{"debug": nil, "trace": nil},
			validate:  validateBooleanExtraArgs,
		},
		{
			name:      "value_flags",
			extraArgs: map[string]*string{"log-level": &[]string{"info"}[0]},
			validate:  validateValueExtraArgs,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := &shared.Options{ExtraArgs: test.extraArgs}
			cmd := BuildCommand("/usr/local/bin/claude", options)
			test.validate(t, cmd)
		})
	}
}

func TestBuildCommandAdvancedFlags(t *testing.T) {
	opts := &shared.Options{
		IncludePartialMessages: true,
		ForkSession:            true,
		SettingSources:         []string{"user", "project"},
	}

	cmd := BuildCommand("/usr/local/bin/claude", opts)

	assertContainsArg(t, cmd, "--include-partial-messages")
	assertContainsArg(t, cmd, "--fork-session")
	assertContainsArgs(t, cmd, "--setting-sources", "user,project")

	// Agents are no longer passed via CLI flag; they are sent via initialize request
	assertNotContainsArg(t, cmd, "--agents")
}

func TestBuildCommandWithMcpServers(t *testing.T) {
	opts := &shared.Options{
		McpServers: map[string]shared.McpServerConfig{
			"file": &shared.McpStdioServerConfig{
				Type:    shared.McpServerTypeStdio,
				Command: "python",
				Args:    []string{"server.py"},
			},
		},
	}

	cmd := BuildCommand("/usr/local/bin/claude", opts)

	mcpJSON, ok := getFlagValue(cmd, "--mcp-config")
	if !ok {
		t.Fatal("Expected --mcp-config flag in command")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(mcpJSON), &payload); err != nil {
		t.Fatalf("Failed to unmarshal MCP payload: %v", err)
	}

	servers, ok := payload["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected mcpServers object, got: %v", payload)
	}

	server, ok := servers["file"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected file server configuration, got: %v", servers)
	}

	if server["type"] != string(shared.McpServerTypeStdio) {
		t.Errorf("Unexpected MCP server type: %v", server["type"])
	}
	if server["command"] != "python" {
		t.Errorf("Unexpected MCP command: %v", server["command"])
	}
}

// TestBuildCommandWithPrompt tests CLI command construction for one-shot queries
// In always-streaming mode, BuildCommandWithPrompt produces the same streaming args
func TestBuildCommandWithPrompt(t *testing.T) {
	tests := []struct {
		name     string
		options  *shared.Options
		validate func(*testing.T, []string)
	}{
		{"basic_prompt", &shared.Options{}, validateStreamingCommand},
		{"nil_options", nil, validateStreamingCommand},
		{"with_model", &shared.Options{Model: stringPtr("claude-3-sonnet")}, func(t *testing.T, cmd []string) {
			t.Helper()
			validateStreamingCommand(t, cmd)
			assertContainsArgs(t, cmd, "--model", "claude-3-sonnet")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := BuildCommandWithPrompt("/usr/local/bin/claude", test.options)
			test.validate(t, cmd)
		})
	}
}

// TestWorkingDirectoryValidation tests working directory validation
func TestWorkingDirectoryValidation(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) string
		expectError   bool
		errorContains string
	}{
		{
			name:        "existing_directory",
			setup:       func(t *testing.T) string { return t.TempDir() },
			expectError: false,
		},
		{
			name:        "empty_path",
			setup:       func(_ *testing.T) string { return "" },
			expectError: false,
		},
		{
			name: "nonexistent_directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			expectError: true,
		},
		{
			name: "file_not_directory",
			setup: func(t *testing.T) string {
				tempFile := filepath.Join(t.TempDir(), "testfile")
				if err := os.WriteFile(tempFile, []byte("test"), 0o600); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
				return tempFile
			},
			expectError:   true,
			errorContains: "not a directory",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.setup(t)
			err := ValidateWorkingDirectory(path)
			assertValidationError(t, err, test.expectError, test.errorContains)
		})
	}
}

// TestCLIVersionDetection tests CLI version detection
func TestCLIVersionDetection(t *testing.T) {
	nonExistentPath := "/this/path/does/not/exist/claude"
	ctx := context.Background()
	_, err := DetectCLIVersion(ctx, nonExistentPath)
	assertVersionDetectionError(t, err)
}

// Helper Functions

func setupIsolatedEnvironment(t *testing.T) func() {
	t.Helper()
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	originalPath := os.Getenv("PATH")

	if runtime.GOOS == windowsOS {
		originalHome = os.Getenv("USERPROFILE")
		_ = os.Setenv("USERPROFILE", tempHome)
	} else {
		_ = os.Setenv("HOME", tempHome)
	}
	_ = os.Setenv("PATH", "/nonexistent/path")

	return func() {
		if runtime.GOOS == windowsOS {
			_ = os.Setenv("USERPROFILE", originalHome)
		} else {
			_ = os.Setenv("HOME", originalHome)
		}
		_ = os.Setenv("PATH", originalPath)
	}
}

func createFullOptionsSet() *shared.Options {
	systemPrompt := "You are a helpful assistant"
	appendPrompt := "Additional context"
	model := "claude-3-sonnet"
	permissionMode := shared.PermissionModeAcceptEdits
	resume := "session123"
	settings := "/path/to/settings.json"
	cwd := "/workspace"
	testValue := "test"

	return &shared.Options{
		AllowedTools:         []string{"Read", "Write"},
		DisallowedTools:      []string{"Bash", "Delete"},
		SystemPrompt:         &systemPrompt,
		AppendSystemPrompt:   &appendPrompt,
		Model:                &model,
		MaxThinkingTokens:    intPtr(10000),
		PermissionMode:       &permissionMode,
		ContinueConversation: true,
		Resume:               &resume,
		MaxTurns:             25,
		Settings:             &settings,
		Cwd:                  &cwd,
		AddDirs:              []string{"/extra/dir1", "/extra/dir2"},
		McpServers:           make(map[string]shared.McpServerConfig),
		ExtraArgs:            map[string]*string{"custom-flag": nil, "with-value": &testValue},
	}
}

// Assertion helpers

func assertCLIDiscoveryError(t *testing.T, err error, expectError bool, errorContains string) {
	t.Helper()
	if (err != nil) != expectError {
		t.Errorf("error = %v, expectError %v", err, expectError)
		return
	}
	if expectError && errorContains != "" && !strings.Contains(err.Error(), errorContains) {
		t.Errorf("error = %v, expected message to contain %q", err, errorContains)
	}
}

func assertDiscoveryLocations(t *testing.T, locations []string) {
	t.Helper()
	if len(locations) == 0 {
		t.Fatal("Expected at least one CLI location, got none")
	}
}

func assertPlatformSpecificPaths(t *testing.T, locations []string) {
	t.Helper()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	expectedNpmGlobal := filepath.Join(homeDir, ".npm-global", "bin", "claude")
	if runtime.GOOS == windowsOS {
		expectedNpmGlobal = filepath.Join(homeDir, ".npm-global", "claude.cmd")
	}

	found := false
	for _, location := range locations {
		if location == expectedNpmGlobal {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected npm-global location %s in discovery paths", expectedNpmGlobal)
	}
}

func assertNodeJSValidation(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Node.js") {
			t.Error("Error message should mention Node.js")
		}
		if !strings.Contains(errMsg, "https://nodejs.org") {
			t.Error("Error message should include Node.js download URL")
		}
	}
}

func assertValidationError(t *testing.T, err error, expectError bool, errorContains string) {
	t.Helper()
	if (err != nil) != expectError {
		t.Errorf("error = %v, expectError %v", err, expectError)
		return
	}
	if expectError && errorContains != "" && !strings.Contains(err.Error(), errorContains) {
		t.Errorf("error = %v, expected message to contain %q", err, errorContains)
	}
}

func assertVersionDetectionError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("Expected error when CLI path does not exist")
		return
	}
	if !strings.Contains(err.Error(), "version") {
		t.Error("Error message should mention version detection failure")
	}
}

// Command validation helpers

func validateStreamingCommand(t *testing.T, cmd []string) {
	t.Helper()
	assertContainsArgs(t, cmd, "--output-format", "stream-json")
	assertContainsArg(t, cmd, "--verbose")
	assertContainsArgs(t, cmd, "--input-format", "stream-json")
	assertNotContainsArg(t, cmd, "--print")
}

func validateFullOptionsCommand(t *testing.T, cmd []string) {
	t.Helper()
	assertContainsArgs(t, cmd, "--allowedTools", "Read,Write")
	assertContainsArgs(t, cmd, "--disallowedTools", "Bash,Delete")
	assertContainsArgs(t, cmd, "--system-prompt", "You are a helpful assistant")
	assertContainsArgs(t, cmd, "--model", "claude-3-sonnet")
	assertContainsArg(t, cmd, "--continue")
	assertContainsArgs(t, cmd, "--resume", "session123")
	assertContainsArg(t, cmd, "--custom-flag")
	assertContainsArgs(t, cmd, "--with-value", "test")
}

func validateBooleanExtraArgs(t *testing.T, cmd []string) {
	t.Helper()
	assertContainsArg(t, cmd, "--debug")
	assertContainsArg(t, cmd, "--trace")
}

func validateValueExtraArgs(t *testing.T, cmd []string) {
	t.Helper()
	assertContainsArgs(t, cmd, "--log-level", "info")
}

// Low-level assertion helpers

func assertContainsArg(t *testing.T, args []string, target string) {
	t.Helper()
	for _, arg := range args {
		if arg == target {
			return
		}
	}
	t.Errorf("Expected command to contain %s, got %v", target, args)
}

func assertNotContainsArg(t *testing.T, args []string, target string) {
	t.Helper()
	for _, arg := range args {
		if arg == target {
			t.Errorf("Expected command to not contain %s, got %v", target, args)
			return
		}
	}
}

func assertContainsArgs(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("Expected command to contain %s %s, got %v", flag, value, args)
}

func getFlagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
	}
	return "", false
}

func assertNotContainsArgs(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			t.Errorf("Expected command to not contain %s %s, got %v", flag, value, args)
			return
		}
	}
}

// Helper function for string pointers
// TestFindCLISuccess tests successful CLI discovery paths
func TestFindCLISuccess(t *testing.T) {
	// Test when CLI is found in PATH
	t.Run("cli_found_in_path", func(t *testing.T) {
		// Create a temporary executable file
		tempDir := t.TempDir()
		cliPath := filepath.Join(tempDir, "claude")
		if runtime.GOOS == windowsOS {
			cliPath += ".exe"
		}

		// Create and make executable
		//nolint:gosec // G306: Test file needs execute permission for mock CLI binary
		err := os.WriteFile(cliPath, []byte("#!/bin/bash\necho test"), 0o700)
		if err != nil {
			t.Fatalf("Failed to create test CLI: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		newPath := tempDir + string(os.PathListSeparator) + originalPath
		if err := os.Setenv("PATH", newPath); err != nil {
			t.Fatalf("Failed to set PATH: %v", err)
		}
		defer func() {
			if err := os.Setenv("PATH", originalPath); err != nil {
				t.Logf("Failed to restore PATH: %v", err)
			}
		}()

		found, err := FindCLI()
		if err != nil {
			t.Errorf("Expected CLI to be found, got error: %v", err)
		}
		if !strings.Contains(found, "claude") {
			t.Errorf("Expected found path to contain 'claude', got: %s", found)
		}
	})

	// Test executable validation on Unix
	if runtime.GOOS != windowsOS {
		t.Run("non_executable_file_skipped", func(t *testing.T) {
			// Create a non-executable file in a location that would be found
			tempDir := t.TempDir()
			cliPath := filepath.Join(tempDir, ".npm-global", "bin", "claude")
			if err := os.MkdirAll(filepath.Dir(cliPath), 0o750); err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}
			if err := os.WriteFile(cliPath, []byte("not executable"), 0o600); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			// Mock home directory
			originalHome := os.Getenv("HOME")
			if err := os.Setenv("HOME", tempDir); err != nil {
				t.Fatalf("Failed to set HOME: %v", err)
			}
			defer func() {
				if err := os.Setenv("HOME", originalHome); err != nil {
					t.Logf("Failed to restore HOME: %v", err)
				}
			}()

			// Isolate PATH to force common location search
			originalPath := os.Getenv("PATH")
			if err := os.Setenv("PATH", "/nonexistent"); err != nil {
				t.Fatalf("Failed to set PATH: %v", err)
			}
			defer func() {
				if err := os.Setenv("PATH", originalPath); err != nil {
					t.Logf("Failed to restore PATH: %v", err)
				}
			}()

			_, err := FindCLI()
			// Should fail because file is not executable
			if err == nil {
				t.Error("Expected error for non-executable file")
			}
		})
	}
}

// TestFindCLINodeJSValidation tests Node.js dependency checks
func TestFindCLINodeJSValidation(t *testing.T) {
	// Test when Node.js is not available
	t.Run("nodejs_not_found", func(t *testing.T) {
		// Isolate environment
		originalPath := os.Getenv("PATH")
		if err := os.Setenv("PATH", "/nonexistent/path"); err != nil {
			t.Fatalf("Failed to set PATH: %v", err)
		}
		defer func() {
			if err := os.Setenv("PATH", originalPath); err != nil {
				t.Logf("Failed to restore PATH: %v", err)
			}
		}()

		_, err := FindCLI()
		if err == nil {
			t.Error("Expected error when Node.js not found")
			return
		}

		errMsg := err.Error()
		if !strings.Contains(errMsg, "Node.js") {
			t.Error("Error should mention Node.js requirement")
		}
		if !strings.Contains(errMsg, "nodejs.org") {
			t.Error("Error should include Node.js installation URL")
		}
	})
}

// TestGetCommonCLILocationsPlatforms tests platform-specific path generation
func TestGetCommonCLILocationsPlatforms(t *testing.T) {
	// Test Windows paths
	if runtime.GOOS == windowsOS {
		t.Run("windows_paths", func(t *testing.T) {
			locations := getCommonCLILocations()

			// Check for Windows-specific patterns
			foundAppData := false
			foundProgramFiles := false

			for _, location := range locations {
				if strings.Contains(location, "AppData") && strings.HasSuffix(location, ".cmd") {
					foundAppData = true
				}
				if strings.Contains(location, "Program Files") && strings.HasSuffix(location, ".cmd") {
					foundProgramFiles = true
				}
			}

			if !foundAppData {
				t.Error("Expected Windows AppData path with .cmd extension")
			}
			if !foundProgramFiles {
				t.Error("Expected Program Files path with .cmd extension")
			}
		})
	}

	// Test home directory fallback
	t.Run("home_directory_fallback", func(t *testing.T) {
		// Temporarily unset home directory env vars
		var originalHome string
		var envVar string

		if runtime.GOOS == windowsOS {
			envVar = "USERPROFILE"
		} else {
			envVar = "HOME"
		}

		originalHome = os.Getenv(envVar)
		if err := os.Unsetenv(envVar); err != nil {
			t.Fatalf("Failed to unset %s: %v", envVar, err)
		}
		defer func() {
			if err := os.Setenv(envVar, originalHome); err != nil {
				t.Logf("Failed to restore %s: %v", envVar, err)
			}
		}()

		locations := getCommonCLILocations()
		// Should still return paths, using current directory as fallback
		if len(locations) == 0 {
			t.Error("Expected fallback paths when home directory unavailable")
		}
	})
}

// TestValidateNodeJSSuccess tests successful Node.js validation
func TestValidateNodeJSSuccess(t *testing.T) {
	// This test assumes Node.js is available in the test environment
	// If Node.js is not available, we'll create a mock
	err := ValidateNodeJS()
	if err != nil {
		// Node.js not found - test the error path
		assertNodeJSValidation(t, err)
	} else {
		// Node.js found - validation should succeed
		t.Log("Node.js validation succeeded")
	}
}

// TestDetectCLIVersionSuccess tests successful version detection
func TestDetectCLIVersionSuccess(t *testing.T) {
	ctx := context.Background()

	// Create a mock CLI that outputs a version
	tempDir := t.TempDir()
	mockCLI := filepath.Join(tempDir, "mock-claude")
	if runtime.GOOS == windowsOS {
		mockCLI += ".bat"
	}

	var script string
	if runtime.GOOS == windowsOS {
		script = "@echo off\necho 1.2.3"
	} else {
		script = "#!/bin/bash\necho '1.2.3'"
	}

	//nolint:gosec // G306: Test file needs execute permission for mock CLI binary
	err := os.WriteFile(mockCLI, []byte(script), 0o700)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	version, err := DetectCLIVersion(ctx, mockCLI)
	if err != nil {
		t.Errorf("Expected successful version detection, got error: %v", err)
		return
	}

	if version != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got '%s'", version)
	}
}

// TestDetectCLIVersionInvalidFormat tests version format validation
func TestDetectCLIVersionInvalidFormat(t *testing.T) {
	ctx := context.Background()

	// Create a mock CLI that outputs invalid version format
	tempDir := t.TempDir()
	mockCLI := filepath.Join(tempDir, "mock-claude-invalid")
	if runtime.GOOS == windowsOS {
		mockCLI += ".bat"
	}

	var script string
	if runtime.GOOS == windowsOS {
		script = "@echo off\necho invalid-version-format"
	} else {
		script = "#!/bin/bash\necho 'invalid-version-format'"
	}

	//nolint:gosec // G306: Test file needs execute permission for mock CLI binary
	err := os.WriteFile(mockCLI, []byte(script), 0o700)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	_, err = DetectCLIVersion(ctx, mockCLI)
	if err == nil {
		t.Error("Expected error for invalid version format")
		return
	}

	if !strings.Contains(err.Error(), "invalid version format") {
		t.Errorf("Expected 'invalid version format' error, got: %v", err)
	}
}

// TestAddPermissionFlagsComplete tests all permission flag combinations
func TestAddPermissionFlagsComplete(t *testing.T) {
	tests := []struct {
		name    string
		options *shared.Options
		expect  map[string]string // flag -> value pairs
	}{
		{
			name: "permission_mode_only",
			options: &shared.Options{
				PermissionMode: func() *shared.PermissionMode {
					mode := shared.PermissionModeAcceptEdits
					return &mode
				}(),
			},
			expect: map[string]string{
				"--permission-mode": "acceptEdits",
			},
		},
		{
			name: "permission_prompt_tool_only",
			options: &shared.Options{
				PermissionPromptToolName: stringPtr("custom-tool"),
			},
			expect: map[string]string{
				"--permission-prompt-tool": "custom-tool",
			},
		},
		{
			name: "both_permission_flags",
			options: &shared.Options{
				PermissionMode: func() *shared.PermissionMode {
					mode := shared.PermissionModeBypassPermissions
					return &mode
				}(),
				PermissionPromptToolName: stringPtr("security-tool"),
			},
			expect: map[string]string{
				"--permission-mode":        "bypassPermissions",
				"--permission-prompt-tool": "security-tool",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := BuildCommand("/usr/local/bin/claude", test.options)

			for flag, expectedValue := range test.expect {
				assertContainsArgs(t, cmd, flag, expectedValue)
			}
		})
	}
}

// TestWorkingDirectoryValidationStatError tests stat error handling
func TestWorkingDirectoryValidationStatError(t *testing.T) {
	// Test with a path that will cause os.Stat to return a non-IsNotExist error
	// This is platform-dependent and hard to trigger reliably, so we test what we can

	// Test permission denied scenario (where possible)
	if runtime.GOOS != windowsOS {
		t.Run("permission_denied_directory", func(t *testing.T) {
			// Create a directory and remove permissions
			tempDir := t.TempDir()
			restrictedDir := filepath.Join(tempDir, "restricted")
			if err := os.Mkdir(restrictedDir, 0o000); err != nil {
				t.Fatalf("Failed to create restricted directory: %v", err)
			}
			defer func() {
				if err := os.Chmod(restrictedDir, 0o600); err != nil {
					t.Logf("Failed to restore directory permissions: %v", err)
				}
			}()

			// Try to validate a subdirectory of the restricted directory
			testPath := filepath.Join(restrictedDir, "subdir")
			err := ValidateWorkingDirectory(testPath)

			// Should return an error (either not exist or permission denied)
			if err == nil {
				t.Error("Expected error for inaccessible directory")
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// TestFallbackModelSupport tests fallback_model option
func TestFallbackModelSupport(t *testing.T) {
	tests := []struct {
		name           string
		options        *shared.Options
		expectContains map[string]string // flag -> value pairs
	}{
		{
			name: "model_and_fallback_model_both_set",
			options: &shared.Options{
				Model:         stringPtr("opus"),
				FallbackModel: stringPtr("sonnet"),
			},
			expectContains: map[string]string{
				"--model":          "opus",
				"--fallback-model": "sonnet",
			},
		},
		{
			name: "only_fallback_model_set",
			options: &shared.Options{
				FallbackModel: stringPtr("sonnet"),
			},
			expectContains: map[string]string{
				"--fallback-model": "sonnet",
			},
		},
		{
			name: "only_model_set",
			options: &shared.Options{
				Model: stringPtr("opus"),
			},
			expectContains: map[string]string{
				"--model": "opus",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildCommand("/usr/local/bin/claude", tt.options)

			for flag, expectedValue := range tt.expectContains {
				assertContainsArgs(t, cmd, flag, expectedValue)
			}
		})
	}
}

// TestSystemPromptDefaultBehavior tests that empty system prompt is passed when SystemPrompt is nil
func TestSystemPromptDefaultBehavior(t *testing.T) {
	tests := []struct {
		name           string
		options        *shared.Options
		expectContains []string // Expected arguments to be present
	}{
		{
			name:    "nil_system_prompt_passes_empty_string",
			options: &shared.Options{SystemPrompt: nil},
			expectContains: []string{
				"--system-prompt",
				"", // Empty string should be the next argument
			},
		},
		{
			name: "explicit_system_prompt_passes_value",
			options: &shared.Options{
				SystemPrompt: stringPtr("You are a helpful assistant"),
			},
			expectContains: []string{
				"--system-prompt",
				"You are a helpful assistant",
			},
		},
		{
			name: "empty_string_system_prompt_passes_empty",
			options: &shared.Options{
				SystemPrompt: stringPtr(""),
			},
			expectContains: []string{
				"--system-prompt",
				"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildCommand("/usr/local/bin/claude", tt.options)

			// Find --system-prompt flag
			found := false
			for i := 0; i < len(cmd)-1; i++ {
				if cmd[i] == "--system-prompt" {
					found = true
					// Verify the value after the flag
					if cmd[i+1] != tt.expectContains[1] {
						t.Errorf("Expected system prompt value %q, got %q", tt.expectContains[1], cmd[i+1])
					}
					break
				}
			}

			if !found {
				t.Error("Expected --system-prompt flag to be present")
			}
		})
	}
}

// TestBuildSettingsValue tests the sandbox settings merging logic
func TestBuildSettingsValue(t *testing.T) {
	t.Run("sandbox_only", func(t *testing.T) {
		options := &shared.Options{
			Sandbox: &shared.SandboxSettings{
				Enabled:                  true,
				AutoAllowBashIfSandboxed: true,
				Network: &shared.SandboxNetworkConfig{
					AllowLocalBinding: true,
					AllowUnixSockets:  []string{"/var/run/docker.sock"},
				},
			},
		}

		result := buildSettingsValue(options)
		if result == "" {
			t.Fatal("Expected non-empty settings value")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse result as JSON: %v", err)
		}

		sandbox, ok := parsed["sandbox"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected sandbox key in parsed result")
		}

		if sandbox["enabled"] != true {
			t.Error("Expected enabled to be true")
		}
		if sandbox["autoAllowBashIfSandboxed"] != true {
			t.Error("Expected autoAllowBashIfSandboxed to be true")
		}

		network, ok := sandbox["network"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected network key in sandbox")
		}
		if network["allowLocalBinding"] != true {
			t.Error("Expected allowLocalBinding to be true")
		}
	})

	t.Run("sandbox_and_settings_json", func(t *testing.T) {
		existingSettings := `{"permissions": {"allow": ["Bash(ls:*)"]}, "verbose": true}`
		options := &shared.Options{
			Settings: &existingSettings,
			Sandbox: &shared.SandboxSettings{
				Enabled:          true,
				ExcludedCommands: []string{"git", "docker"},
			},
		}

		result := buildSettingsValue(options)
		if result == "" {
			t.Fatal("Expected non-empty settings value")
		}

		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse result as JSON: %v", err)
		}

		// Original settings should be preserved
		permissions, ok := parsed["permissions"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected permissions key to be preserved")
		}
		if parsed["verbose"] != true {
			t.Error("Expected verbose to be preserved")
		}
		_ = permissions // Used above

		// Sandbox should be merged in
		sandbox, ok := parsed["sandbox"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected sandbox key in parsed result")
		}
		if sandbox["enabled"] != true {
			t.Error("Expected enabled to be true")
		}
	})

	t.Run("settings_file_path_no_sandbox", func(t *testing.T) {
		settingsPath := "/path/to/settings.json"
		options := &shared.Options{
			Settings: &settingsPath,
		}

		result := buildSettingsValue(options)
		if result != settingsPath {
			t.Errorf("Expected path to be passed through, got %s", result)
		}
	})

	t.Run("sandbox_minimal", func(t *testing.T) {
		options := &shared.Options{
			Sandbox: &shared.SandboxSettings{
				Enabled: true,
			},
		}

		result := buildSettingsValue(options)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse result as JSON: %v", err)
		}

		sandbox, ok := parsed["sandbox"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected sandbox key")
		}
		if sandbox["enabled"] != true {
			t.Error("Expected enabled to be true")
		}
	})

	t.Run("sandbox_network_config", func(t *testing.T) {
		options := &shared.Options{
			Sandbox: &shared.SandboxSettings{
				Enabled: true,
				Network: &shared.SandboxNetworkConfig{
					AllowUnixSockets:    []string{"/tmp/ssh-agent.sock"},
					AllowAllUnixSockets: false,
					AllowLocalBinding:   true,
					HTTPProxyPort:       8080,
					SOCKSProxyPort:      8081,
				},
			},
		}

		result := buildSettingsValue(options)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("Failed to parse result as JSON: %v", err)
		}

		sandbox := parsed["sandbox"].(map[string]interface{})
		network := sandbox["network"].(map[string]interface{})

		sockets, ok := network["allowUnixSockets"].([]interface{})
		if !ok || len(sockets) != 1 || sockets[0] != "/tmp/ssh-agent.sock" {
			t.Error("Expected allowUnixSockets to contain /tmp/ssh-agent.sock")
		}
		// allowAllUnixSockets=false is omitted due to omitempty
		if val, exists := network["allowAllUnixSockets"]; exists && val != false {
			t.Error("Expected allowAllUnixSockets to be false or omitted")
		}
		if network["allowLocalBinding"] != true {
			t.Error("Expected allowLocalBinding to be true")
		}
		if network["httpProxyPort"] != float64(8080) {
			t.Errorf("Expected httpProxyPort to be 8080, got %v", network["httpProxyPort"])
		}
		if network["socksProxyPort"] != float64(8081) {
			t.Errorf("Expected socksProxyPort to be 8081, got %v", network["socksProxyPort"])
		}
	})

	t.Run("no_settings_no_sandbox", func(t *testing.T) {
		options := &shared.Options{}

		result := buildSettingsValue(options)
		if result != "" {
			t.Errorf("Expected empty string, got %s", result)
		}
	})
}
