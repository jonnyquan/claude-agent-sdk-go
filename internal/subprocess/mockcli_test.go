package subprocess

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// The mock CLI is a real compiled binary rather than a shell script for two
// reasons:
//
//  1. Connect() performs the control-protocol initialize handshake before it
//     reports success, so the mock has to read stdin and answer control
//     requests. Expressing that in both bash and cmd.exe batch is far more
//     fragile than writing it once in Go.
//  2. RejectWindowsBatchCLI refuses .bat/.cmd CLI paths on Windows, so a batch
//     mock could not be spawned there at all.
const mockCLISource = `package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Behavior is selected by the name the binary was installed under, so one
// source serves every variant without needing extra argv or env plumbing
// (the transport controls both).
func variant() string {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if i := strings.LastIndex(name, "-"); i >= 0 {
		return name[i+1:]
	}
	return name
}

var outMu sync.Mutex

func emit(line string) {
	outMu.Lock()
	defer outMu.Unlock()
	fmt.Fprintln(os.Stdout, line)
}

// assistantLine emits the wire shape the parser actually expects: content and
// model live under "message", not at the top level.
func assistantLine(text string) string {
	return fmt.Sprintf("{\"type\":\"assistant\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"text\",\"text\":%q}],\"model\":\"claude-3\"},\"session_id\":\"mock-session\"}", text)
}

// answerControl plays the CLI's half of the control protocol: every
// control_request gets a success control_response carrying the same
// request_id. This is what unblocks Connect's initialize handshake and
// Transport.Interrupt. The channel closes when stdin reaches EOF.
func answerControl(stdinClosed chan<- struct{}) {
	defer close(stdinClosed)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var frame struct {
			Type      string ` + "`json:\"type\"`" + `
			RequestID string ` + "`json:\"request_id\"`" + `
		}
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			continue
		}
		if frame.Type != "control_request" {
			continue
		}
		resp, err := json.Marshal(map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": frame.RequestID,
				"response":   map[string]any{},
			},
		})
		if err != nil {
			continue
		}
		emit(string(resp))
	}
}

func main() {
	v := variant()

	if v == "failure" {
		fmt.Fprintln(os.Stderr, "Mock CLI failing")
		os.Exit(1)
	}
	if v == "envcheck" && os.Getenv("CLAUDE_CODE_ENTRYPOINT") != "sdk-go" {
		fmt.Fprintln(os.Stderr, "Missing environment variable")
		os.Exit(1)
	}
	if v == "longrunning" {
		// Swallow SIGTERM so Close() has to escalate to SIGKILL — that
		// escalation is exactly what the termination tests measure.
		signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)
	}

	stdinClosed := make(chan struct{})
	go answerControl(stdinClosed)

	switch v {
	case "invalidoutput":
		emit("This is not valid JSON output")
		emit("{\"invalid\": json}")
		emit(assistantLine("Valid after invalid"))
	case "envcheck":
		emit(assistantLine("Environment OK"))
	case "longrunning":
		emit(assistantLine("Long running mock"))
		// Deliberately ignore stdin EOF: this variant must outlive the
		// transport's shutdown so the SIGTERM/SIGKILL path is exercised.
		time.Sleep(30 * time.Second)
		return
	default:
		emit(assistantLine("Mock response"))
	}

	// Exit when stdin closes, like the real CLI. The cap keeps a leaked mock
	// from outliving the test binary.
	select {
	case <-stdinClosed:
	case <-time.After(30 * time.Second):
	}
}
`

var (
	mockCLIDirOnce sync.Once
	mockCLIDir     string
	mockCLIDirErr  error

	mockCLIMu   sync.Mutex
	mockCLIPath = map[string]string{}
)

// buildMockCLI compiles (once per variant, cached for the whole test binary)
// the mock CLI and returns its path.
func buildMockCLI(t *testing.T, variant string) string {
	t.Helper()

	mockCLIDirOnce.Do(func() {
		mockCLIDir, mockCLIDirErr = os.MkdirTemp("", "mock-claude-cli")
		if mockCLIDirErr != nil {
			return
		}
		mockCLIDirErr = os.WriteFile(filepath.Join(mockCLIDir, "main.go"), []byte(mockCLISource), 0o600)
	})
	if mockCLIDirErr != nil {
		t.Fatalf("Failed to stage mock CLI source: %v", mockCLIDirErr)
	}

	mockCLIMu.Lock()
	defer mockCLIMu.Unlock()

	if path, ok := mockCLIPath[variant]; ok {
		return path
	}

	name := "mock-claude-" + variant
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(mockCLIDir, name)

	build := exec.Command("go", "build", "-o", path, filepath.Join(mockCLIDir, "main.go"))
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build mock CLI %q: %v\n%s", variant, err, out)
	}

	mockCLIPath[variant] = path
	return path
}

// removeMockCLIDir drops the shared build directory once the package's tests
// are done; the old shell-script mocks leaked a file per call into TempDir.
func removeMockCLIDir() {
	if mockCLIDir != "" {
		_ = os.RemoveAll(mockCLIDir)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	removeMockCLIDir()
	os.Exit(code)
}
