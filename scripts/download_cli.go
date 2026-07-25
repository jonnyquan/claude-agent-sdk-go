// CLI download utility for bundling Claude Code CLI with the Go SDK
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const (
	defaultCLIVersion = "2.1.220"
	baseDownloadURL   = "https://github.com/anthropics/claude-code/releases/download/v"
)

// versionPattern admits a deliberate subset of what the installer accepts. The
// installer's suffix rule (`-[^\s]+`) would allow quotes, slashes and
// semicolons; narrowing it to the characters real versions use keeps a value
// taken from the environment from steering the download URL somewhere else.
//
// Matched with a full-string anchor so a trailing newline or an obvious prefix
// like "1.0.0; id" is rejected rather than silently accepted.
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.+-]+)?$`)

// validateVersion returns the usable form of version, or an error naming why it
// is unusable. Surrounding whitespace is stripped first — a trailing "\n" from a
// file read or a "\r" from a CRLF checkout is unambiguous in intent — and the
// stripped value is what the caller must use downstream.
func validateVersion(version, source string) (string, error) {
	candidate := strings.TrimSpace(version)
	if versionPattern.MatchString(candidate) {
		return candidate, nil
	}
	// Never normalized away: the caller asked for something unsupported, and
	// silently downloading a different string is worse.
	if trimmed := strings.TrimPrefix(strings.TrimPrefix(candidate, "v"), "V"); trimmed != candidate &&
		versionPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid %s: %q. Did you mean %q? (no leading 'v')", source, candidate, trimmed)
	}
	return "", fmt.Errorf(
		"invalid %s: %q. Expected a concrete version matching %s",
		source, version, versionPattern.String(),
	)
}

func main() {
	version := os.Getenv("CLAUDE_CLI_VERSION")
	if version == "" {
		version = defaultCLIVersion
	}
	version, err := validateVersion(version, "CLAUDE_CLI_VERSION")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloading Claude CLI version %s for %s/%s\n", version, runtime.GOOS, runtime.GOARCH)

	// Determine the download URL and binary name based on platform
	var downloadURL, binaryName string
	switch runtime.GOOS {
	case "windows":
		downloadURL = fmt.Sprintf("%s%s/claude-code-win-x64.exe", baseDownloadURL, version)
		binaryName = "claude.exe"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			downloadURL = fmt.Sprintf("%s%s/claude-code-macos-arm64", baseDownloadURL, version)
		} else {
			downloadURL = fmt.Sprintf("%s%s/claude-code-macos-x64", baseDownloadURL, version)
		}
		binaryName = "claude"
	case "linux":
		if runtime.GOARCH == "arm64" {
			downloadURL = fmt.Sprintf("%s%s/claude-code-linux-arm64", baseDownloadURL, version)
		} else {
			downloadURL = fmt.Sprintf("%s%s/claude-code-linux-x64", baseDownloadURL, version)
		}
		binaryName = "claude"
	default:
		fmt.Fprintf(os.Stderr, "Unsupported platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(1)
	}

	// Create _bundled directory if it doesn't exist
	bundledDir := "_bundled"
	if err := os.MkdirAll(bundledDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create _bundled directory: %v\n", err)
		os.Exit(1)
	}

	// Download the CLI binary
	outputPath := filepath.Join(bundledDir, binaryName)
	if err := downloadFile(downloadURL, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download CLI: %v\n", err)
		os.Exit(1)
	}

	// Make binary executable on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(outputPath, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to make binary executable: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Downloaded Claude CLI to %s\n", outputPath)
}

func downloadFile(url, outputPath string) error {
	fmt.Printf("Downloading from: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
