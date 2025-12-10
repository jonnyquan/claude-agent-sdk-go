// Package subprocess provides the subprocess transport implementation for Claude Code CLI.
package subprocess

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/cli"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/parser"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/query"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

const (
	// channelBufferSize is the buffer size for message and error channels.
	channelBufferSize = 10
	// terminationTimeoutSeconds is the timeout for graceful process termination.
	terminationTimeoutSeconds = 5
	// windowsOS is the GOOS value for Windows platform.
	windowsOS = "windows"
	// cmdLengthLimitWindows is the command line length limit for Windows (cmd.exe has 8191 char limit)
	cmdLengthLimitWindows = 8000
	// cmdLengthLimitOther is the command line length limit for other platforms
	cmdLengthLimitOther = 100000
)

// Transport implements the Transport interface using subprocess communication.
type Transport struct {
	// Process management
	cmd        *exec.Cmd
	cliPath    string
	options    *shared.Options
	closeStdin bool
	promptArg  *string // For one-shot queries, prompt passed as CLI argument
	entrypoint string  // CLAUDE_CODE_ENTRYPOINT value (sdk-go or sdk-go-client)
	sdkVersion string  // CLAUDE_AGENT_SDK_VERSION value

	// Connection state
	connected bool
	mu        sync.RWMutex

	// I/O streams
	stdin  io.WriteCloser
	stdout io.ReadCloser
	// Note: stderr is now handled via pipe in readStderr goroutine

	// Message parsing
	parser *parser.Parser

	// Channels for communication
	msgChan chan shared.Message
	errChan chan error

	// Hook and control protocol support
	controlProtocol *query.ControlProtocol
	hookProcessor   *query.HookProcessor
	isStreamingMode bool

	// Control and cleanup
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	tempFiles []string // Track temporary files for cleanup

	// Track first result for proper stream closure with SDK MCP servers
	firstResultReceived chan struct{}
	firstResultOnce     sync.Once
	streamCloseTimeout  time.Duration
}

// New creates a new subprocess transport.
func New(cliPath string, options *shared.Options, closeStdin bool, entrypoint string, sdkVersion string) *Transport {
	return &Transport{
		cliPath:             cliPath,
		options:             options,
		closeStdin:          closeStdin,
		entrypoint:          entrypoint,
		sdkVersion:          sdkVersion,
		parser:              parser.New(),
		firstResultReceived: make(chan struct{}),
		streamCloseTimeout:  getStreamCloseTimeout(),
	}
}

// NewWithPrompt creates a new subprocess transport for one-shot queries with prompt as CLI argument.
func NewWithPrompt(cliPath string, options *shared.Options, prompt string, sdkVersion string) *Transport {
	return &Transport{
		cliPath:             cliPath,
		options:             options,
		closeStdin:          true,
		entrypoint:          "sdk-go", // Query mode uses sdk-go
		sdkVersion:          sdkVersion,
		parser:              parser.New(),
		promptArg:           &prompt,
		firstResultReceived: make(chan struct{}),
		streamCloseTimeout:  getStreamCloseTimeout(),
	}
}

// getStreamCloseTimeout returns the stream close timeout from environment variable.
// Default is 60 seconds. Environment variable is in milliseconds.
func getStreamCloseTimeout() time.Duration {
	timeoutMs := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT")
	if timeoutMs == "" {
		return 60 * time.Second // Default 60 seconds
	}
	ms, err := strconv.ParseInt(timeoutMs, 10, 64)
	if err != nil {
		return 60 * time.Second
	}
	return time.Duration(ms) * time.Millisecond
}

// IsConnected returns whether the transport is currently connected.
func (t *Transport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected && t.cmd != nil && t.cmd.Process != nil
}

// Connect starts the Claude CLI subprocess.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("transport already connected")
	}

	// Build command with all options
	var args []string
	if t.promptArg != nil {
		// One-shot query with prompt as CLI argument
		args = cli.BuildCommandWithPrompt(t.cliPath, t.options, *t.promptArg)
	} else {
		// Streaming mode or regular one-shot
		args = cli.BuildCommand(t.cliPath, t.options, t.closeStdin)
	}

	// Handle command line length limits (Windows compatibility)
	if err := t.handleCommandLineLength(&args); err != nil {
		return fmt.Errorf("failed to handle command line length: %w", err)
	}

	//nolint:gosec // G204: This is the core CLI SDK functionality - subprocess execution is required
	t.cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	// Set up environment - idiomatic Go: start with system env
	env := os.Environ()

	// Add SDK identifier (required)
	env = append(env, "CLAUDE_CODE_ENTRYPOINT="+t.entrypoint)
	if t.sdkVersion != "" {
		env = append(env, fmt.Sprintf("CLAUDE_AGENT_SDK_VERSION=%s", t.sdkVersion))
	}

	// Enable file checkpointing if requested
	if t.options != nil && t.options.EnableFileCheckpointing {
		env = append(env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
	}

	// Merge custom environment variables
	if t.options != nil && t.options.ExtraEnv != nil {
		for key, value := range t.options.ExtraEnv {
			// Use fmt.Sprintf for clarity and consistency
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Apply environment to command
	t.cmd.Env = env

	// Set working directory if specified
	if t.options != nil && t.options.Cwd != nil {
		if err := cli.ValidateWorkingDirectory(*t.options.Cwd); err != nil {
			return err
		}
		t.cmd.Dir = *t.options.Cwd
	}

	// Set up I/O pipes
	var err error
	if t.promptArg == nil {
		// Only create stdin pipe if we need to send messages via stdin
		t.stdin, err = t.cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdin pipe: %w", err)
		}
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr with pipe for error visibility
	// Using pipe + goroutine to prevent deadlocks while maintaining error diagnostics
	stderrPipe, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := t.cmd.Start(); err != nil {
		t.cleanup()
		return shared.NewConnectionError(
			fmt.Sprintf("failed to start Claude CLI: %v", err),
			err,
		)
	}

	// Set up context for goroutine management
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Initialize channels
	t.msgChan = make(chan shared.Message, channelBufferSize)
	t.errChan = make(chan error, channelBufferSize)

	// Start stderr reader goroutine
	t.wg.Add(1)
	go t.readStderr(stderrPipe)

	// Start I/O handling goroutines
	t.wg.Add(1)
	go t.handleStdout()

	// Note: Do NOT close stdin here for one-shot mode
	// The CLI still needs stdin to receive the message, even with --print flag
	// stdin will be closed after sending the message in SendMessage()

	// Initialize hook support for streaming mode
	t.isStreamingMode = !t.closeStdin && t.promptArg == nil
	if t.isStreamingMode && t.options != nil && len(t.options.Hooks) > 0 {
		// Create hook processor
		t.hookProcessor = query.NewHookProcessor(t.ctx, t.options)
		
		// Create write function for control protocol
		writeFn := func(data []byte) error {
			t.mu.RLock()
			defer t.mu.RUnlock()
			
			if !t.connected || t.stdin == nil {
				return fmt.Errorf("transport not connected")
			}
			
			_, err := t.stdin.Write(data)
			return err
		}
		
		// Create control protocol handler
		t.controlProtocol = query.NewControlProtocol(t.ctx, t.hookProcessor, writeFn)
		
		// Send initialization request to CLI
		if _, err := t.controlProtocol.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize control protocol: %w", err)
		}
	}

	t.connected = true
	return nil
}

// SendMessage sends a message to the CLI subprocess.
func (t *Transport) SendMessage(ctx context.Context, message shared.StreamMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// For one-shot queries with promptArg, the prompt is already passed as CLI argument
	// so we don't need to send any messages via stdin
	if t.promptArg != nil {
		return nil // No-op for one-shot queries
	}

	if !t.connected || t.stdin == nil {
		return fmt.Errorf("transport not connected or stdin closed")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Serialize message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send with newline
	_, err = t.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	// For one-shot mode, close stdin after sending the message
	// If we have SDK MCP servers or hooks that need bidirectional communication,
	// wait for first result before closing stdin
	if t.closeStdin {
		hasHooks := t.options != nil && len(t.options.Hooks) > 0
		hasSdkMcpServers := t.hasSdkMcpServers()

		if hasHooks || hasSdkMcpServers {
			// Wait for first result before closing stdin
			select {
			case <-t.firstResultReceived:
				// Received first result, safe to close
			case <-time.After(t.streamCloseTimeout):
				// Timeout, close anyway
			case <-ctx.Done():
				// Context canceled
			}
		}

		_ = t.stdin.Close()
		t.stdin = nil
	}

	return nil
}

// hasSdkMcpServers checks if there are any SDK MCP servers configured.
func (t *Transport) hasSdkMcpServers() bool {
	if t.options == nil || len(t.options.McpServers) == 0 {
		return false
	}
	for _, server := range t.options.McpServers {
		if server != nil && server.GetType() == shared.McpServerTypeSDK {
			return true
		}
	}
	return false
}

// ReceiveMessages returns channels for receiving messages and errors.
func (t *Transport) ReceiveMessages(_ context.Context) (<-chan shared.Message, <-chan error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		// Return closed channels if not connected
		msgChan := make(chan shared.Message)
		errChan := make(chan error)
		close(msgChan)
		close(errChan)
		return msgChan, errChan
	}

	return t.msgChan, t.errChan
}

// Interrupt sends an interrupt signal to the subprocess.
func (t *Transport) Interrupt(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.cmd == nil || t.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}

	// Windows doesn't support os.Interrupt signal
	if runtime.GOOS == windowsOS {
		return fmt.Errorf("interrupt not supported by windows")
	}

	// Send interrupt signal (Unix/Linux/macOS)
	return t.cmd.Process.Signal(os.Interrupt)
}

// RewindFiles rewinds tracked files to their state at a specific user message.
// Requires file checkpointing to be enabled via the EnableFileCheckpointing option.
func (t *Transport) RewindFiles(_ context.Context, userMessageID string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	if t.controlProtocol == nil {
		return fmt.Errorf("control protocol not initialized - streaming mode required")
	}

	return t.controlProtocol.RewindFiles(userMessageID)
}

// Close terminates the subprocess connection.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		// Clean up temp files even if already closed
		for _, tempFile := range t.tempFiles {
			_ = os.Remove(tempFile)
		}
		t.tempFiles = nil
		return nil // Already closed
	}

	t.connected = false

	// Clean up control protocol
	if t.controlProtocol != nil {
		_ = t.controlProtocol.Close()
		t.controlProtocol = nil
		t.hookProcessor = nil
	}

	// Cancel context to stop goroutines
	if t.cancel != nil {
		t.cancel()
	}

	// Close stdin if open
	if t.stdin != nil {
		_ = t.stdin.Close()
		t.stdin = nil
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Goroutines finished gracefully
	case <-time.After(terminationTimeoutSeconds * time.Second):
		// Timeout: proceed with cleanup anyway
		// Goroutines should terminate when process is killed
	}

	// Terminate process with 5-second timeout
	var err error
	if t.cmd != nil && t.cmd.Process != nil {
		err = t.terminateProcess()
	}

	// Cleanup resources
	t.cleanup()

	return err
}

// readStderr reads and logs stderr output from CLI for error diagnostics
func (t *Transport) readStderr(stderr io.ReadCloser) {
	defer t.wg.Done()
	defer stderr.Close()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		// Log stderr for debugging - users can see CLI errors
		// In production, you might want to use a proper logger
		if len(line) > 0 {
			// Send to error channel for visibility
			select {
			case t.errChan <- fmt.Errorf("CLI stderr: %s", line):
			default:
				// Channel full, skip (prevent blocking)
			}
		}
	}
}

// handleStdout processes stdout in a separate goroutine
func (t *Transport) handleStdout() {
	defer t.wg.Done()
	defer close(t.msgChan)
	defer close(t.errChan)

	scanner := bufio.NewScanner(t.stdout)

	for scanner.Scan() {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Check if this is a control message and route accordingly
		if t.controlProtocol != nil {
			// Try to parse as JSON to check message type
			var rawMsg map[string]any
			if err := json.Unmarshal([]byte(line), &rawMsg); err == nil {
				if msgType, ok := rawMsg["type"].(string); ok {
					// Route control messages to control protocol
					switch msgType {
					case "control_request", "control_response", "control_cancel_request":
						if err := t.controlProtocol.HandleIncomingMessage(msgType, []byte(line)); err != nil {
							// Log error but don't stop processing
							select {
							case t.errChan <- fmt.Errorf("control protocol error: %w", err):
							case <-t.ctx.Done():
								return
							}
						}
						continue // Don't send control messages to regular message channel
					}
				}
			}
		}

		// Parse line with the parser (normal messages)
		messages, err := t.parser.ProcessLine(line)
		if err != nil {
			select {
			case t.errChan <- err:
			case <-t.ctx.Done():
				return
			}
			continue
		}

		// Send parsed messages
		for _, msg := range messages {
			if msg != nil {
				// Track results for proper stream closure
				if _, isResult := msg.(*shared.ResultMessage); isResult {
					t.firstResultOnce.Do(func() {
						close(t.firstResultReceived)
					})
				}

				select {
				case t.msgChan <- msg:
				case <-t.ctx.Done():
					return
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		scanErr := fmt.Errorf("stdout scanner error: %w", err)
		// Signal all pending control requests so they fail fast instead of timing out
		if t.controlProtocol != nil {
			t.controlProtocol.FailPendingRequests(scanErr)
		}
		select {
		case t.errChan <- scanErr:
		case <-t.ctx.Done():
		}
	}
}

// isProcessAlreadyFinishedError checks if an error indicates the process has already terminated.
// This follows the Python SDK pattern of suppressing "process not found" type errors.
func isProcessAlreadyFinishedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "process already finished") ||
		strings.Contains(errStr, "process already released") ||
		strings.Contains(errStr, "no child processes") ||
		strings.Contains(errStr, "signal: killed")
}

// terminateProcess implements the 5-second SIGTERM → SIGKILL sequence
func (t *Transport) terminateProcess() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM
	if err := t.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// If process is already finished, that's success
		if isProcessAlreadyFinishedError(err) {
			return nil
		}
		// If SIGTERM fails for other reasons, try SIGKILL immediately
		killErr := t.cmd.Process.Kill()
		if killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		return nil // Don't return error for expected termination
	}

	// Wait exactly 5 seconds
	done := make(chan error, 1)
	// Capture cmd while we know it's valid to avoid data race
	cmd := t.cmd
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Normal termination or expected signals are not errors
		if err != nil {
			// Check if it's an expected exit signal
			if strings.Contains(err.Error(), "signal:") {
				return nil // Expected signal termination
			}
		}
		return err
	case <-time.After(terminationTimeoutSeconds * time.Second):
		// Force kill after 5 seconds
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		// Wait for process to exit after kill
		<-done
		return nil
	case <-t.ctx.Done():
		// Context canceled - force kill immediately
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		// Wait for process to exit after kill, but don't return context error
		// since this is normal cleanup behavior
		<-done
		return nil
	}
}

// handleCommandLineLength checks command line length and uses temp files if needed (Windows compatibility)
func (t *Transport) handleCommandLineLength(args *[]string) error {
	// Calculate command line length
	cmdStr := strings.Join(*args, " ")
	cmdLength := len(cmdStr)

	// Determine limit based on platform
	limit := cmdLengthLimitOther
	if runtime.GOOS == windowsOS {
		limit = cmdLengthLimitWindows
	}

	// Check if command exceeds limit
	if cmdLength <= limit {
		return nil // No action needed
	}

	// Command is too long - find --agents argument and move to temp file
	// This only applies if we have agents configured
	if t.options == nil || len(t.options.Agents) == 0 {
		return nil // No agents to optimize
	}

	// Find --agents flag index
	agentsIdx := -1
	for i, arg := range *args {
		if arg == "--agents" && i+1 < len(*args) {
			agentsIdx = i
			break
		}
	}

	if agentsIdx == -1 {
		// No --agents flag found, can't optimize
		return fmt.Errorf("command line length (%d) exceeds limit (%d) but no --agents flag found to optimize", cmdLength, limit)
	}

	// Get agents JSON value
	agentsJSON := (*args)[agentsIdx+1]

	// Create temporary file
	tempFile, err := os.CreateTemp("", "claude-agents-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Write agents JSON to temp file
	if _, err := tempFile.WriteString(agentsJSON); err != nil {
		_ = os.Remove(tempFile.Name())
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(tempFile.Name())
	if err != nil {
		_ = os.Remove(tempFile.Name())
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Track temp file for cleanup
	t.tempFiles = append(t.tempFiles, absPath)

	// Replace agents JSON with @filepath reference
	(*args)[agentsIdx+1] = "@" + absPath

	// Log the optimization
	fmt.Fprintf(os.Stderr, "Command line length (%d) exceeds limit (%d). Using temp file for --agents: %s\n",
		cmdLength, limit, absPath)

	return nil
}

// cleanup cleans up all resources
func (t *Transport) cleanup() {
	// Clean up temporary files first
	for _, tempFile := range t.tempFiles {
		_ = os.Remove(tempFile)
	}
	t.tempFiles = nil

	if t.stdout != nil {
		_ = t.stdout.Close()
		t.stdout = nil
	}

	// Note: stderr is now handled via pipe, no cleanup needed

	// Reset state
	t.cmd = nil
}
