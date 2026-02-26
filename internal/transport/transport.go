// Package transport provides the transport implementation for Claude Code CLI.
package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/discovery"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/parsing"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/query"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

const (
	// channelBufferSize is the buffer size for message and error channels.
	channelBufferSize = 10
	// terminationTimeoutSeconds is the timeout for graceful process termination.
	terminationTimeoutSeconds = 5
)

// Transport implements the Transport interface using subprocess communication.
type Transport struct {
	// Process management
	cmd        *exec.Cmd
	cliPath    string
	options    *shared.Options
	promptArg  *string // For one-shot queries, prompt sent via stdin after initialize
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
	parser *parsing.Parser

	// Channels for communication
	msgChan chan shared.Message
	errChan chan error

	// Hook and control protocol support (always initialized in streaming mode)
	controlProtocol *query.ControlProtocol
	hookProcessor   *query.HookProcessor

	// Server initialization info (stored from initialize response)
	serverInfo map[string]any

	// Control and cleanup
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new subprocess transport.
// Always uses streaming mode for bidirectional communication.
func New(cliPath string, options *shared.Options, entrypoint string, sdkVersion string) *Transport {
	return &Transport{
		cliPath:    cliPath,
		options:    options,
		entrypoint: entrypoint,
		sdkVersion: sdkVersion,
		parser:     parsing.New(),
	}
}

// NewWithPrompt creates a new subprocess transport for one-shot queries.
// Prompt is sent via stdin after initialize.
func NewWithPrompt(cliPath string, options *shared.Options, prompt string, sdkVersion string) *Transport {
	return &Transport{
		cliPath:    cliPath,
		options:    options,
		entrypoint: "sdk-go", // Query mode uses sdk-go
		sdkVersion: sdkVersion,
		parser:     parsing.New(),
		promptArg:  &prompt,
	}
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

	// Match Python SDK behavior: even with explicit CLI path, run a best-effort
	// version compatibility check (warn only, never fail connect).
	if t.options != nil && t.options.CLIPath != nil && *t.options.CLIPath != "" {
		if verErr := discovery.CheckCLIVersion(*t.options.CLIPath); verErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: %v\n", verErr)
		}
	}

	// Build command - always use streaming mode
	args := discovery.BuildCommand(t.cliPath, t.options)

	//nolint:gosec // G204: This is the core CLI SDK functionality - subprocess execution is required
	t.cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	// Apply requested process user if configured.
	if t.options != nil {
		if err := applyUserOption(t.cmd, t.options.User); err != nil {
			return err
		}
	}

	// Set up environment: system env → user env → SDK vars (SDK always wins)
	env := os.Environ()

	// Merge custom environment variables (user-provided, can be overridden by SDK vars)
	if t.options != nil && t.options.ExtraEnv != nil {
		for key, value := range t.options.ExtraEnv {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// SDK identifier vars come last (override all, matching Python SDK behavior)
	env = append(env, "CLAUDE_CODE_ENTRYPOINT="+t.entrypoint)
	if t.sdkVersion != "" {
		env = append(env, fmt.Sprintf("CLAUDE_AGENT_SDK_VERSION=%s", t.sdkVersion))
	}

	// Enable file checkpointing if requested
	if t.options != nil && t.options.EnableFileCheckpointing {
		env = append(env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
	}

	// Set working directory if specified
	if t.options != nil && t.options.Cwd != nil {
		if err := discovery.ValidateWorkingDirectory(*t.options.Cwd); err != nil {
			return err
		}
		t.cmd.Dir = *t.options.Cwd
		// Set PWD env var to match Python SDK behavior
		env = append(env, fmt.Sprintf("PWD=%s", *t.options.Cwd))
	}

	// Apply environment to command
	t.cmd.Env = env

	// Set up I/O pipes - always create stdin for streaming mode
	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr with pipe for error visibility
	var stderrPipe io.ReadCloser
	if shouldPipeStderr(t.options) {
		stderrPipe, err = t.cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("failed to create stderr pipe: %w", err)
		}
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

	// Start stderr reader goroutine when explicitly requested via callback/debug flag.
	if stderrPipe != nil {
		t.wg.Add(1)
		go t.readStderr(stderrPipe)
	}

	// Start I/O handling goroutines
	t.wg.Add(1)
	go t.handleStdout()

	// Always initialize control protocol for streaming mode

	// Create hook processor if hooks are configured
	if t.options != nil && len(t.options.Hooks) > 0 {
		t.hookProcessor = query.NewHookProcessor(t.ctx, t.options)
	}

	// Create write function for control protocol.
	// IMPORTANT: Use captured stdin directly to avoid re-entering t.mu while Connect
	// still holds the write lock (which can deadlock during Initialize()).
	stdin := t.stdin
	writeFn := func(data []byte) error {
		if stdin == nil {
			return fmt.Errorf("transport stdin not initialized")
		}
		_, err := stdin.Write(data)
		return err
	}

	// Build SDK MCP servers map
	var sdkMCPServers map[string]shared.McpSDKServer
	if t.options != nil && len(t.options.McpServers) > 0 {
		sdkMCPServers = make(map[string]shared.McpSDKServer)
		for name, cfg := range t.options.McpServers {
			if sdkCfg, ok := cfg.(*shared.McpSdkServerConfig); ok && sdkCfg.Instance != nil {
				sdkMCPServers[name] = sdkCfg.Instance
			}
		}
	}

	// Create control protocol handler
	t.controlProtocol = query.NewControlProtocol(t.ctx, t.hookProcessor, writeFn, sdkMCPServers)

	// Build agents dict for initialize request
	var agentsDict map[string]map[string]any
	if t.options != nil && len(t.options.Agents) > 0 {
		agentsDict = make(map[string]map[string]any, len(t.options.Agents))
		for name, def := range t.options.Agents {
			entry := map[string]any{
				"description": def.Description,
				"prompt":      def.Prompt,
			}
			if len(def.Tools) > 0 {
				entry["tools"] = def.Tools
			}
			if def.Model != nil && *def.Model != "" {
				entry["model"] = *def.Model
			}
			agentsDict[name] = entry
		}
	}

	// Send initialization request to CLI
	initResult, err := t.controlProtocol.Initialize(agentsDict)
	if err != nil {
		return fmt.Errorf("failed to initialize control protocol: %w", err)
	}
	t.serverInfo = initResult

	// For one-shot queries, send the prompt via stdin after initialize
	if t.promptArg != nil {
		streamMsg := shared.StreamMessage{
			Type: "user",
			Message: map[string]interface{}{
				"role":    "user",
				"content": *t.promptArg,
			},
			// Match Python one-shot query behavior: empty session_id by default.
			SessionID: "",
		}
		data, marshalErr := json.Marshal(streamMsg)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal prompt message: %w", marshalErr)
		}
		if _, writeErr := t.stdin.Write(append(data, '\n')); writeErr != nil {
			return fmt.Errorf("failed to send prompt: %w", writeErr)
		}
		if closeErr := t.stdin.Close(); closeErr != nil {
			return fmt.Errorf("failed to end input after prompt: %w", closeErr)
		}
		t.stdin = nil
	}

	t.connected = true
	return nil
}

// SendMessage sends a message to the CLI subprocess.
func (t *Transport) SendMessage(ctx context.Context, message shared.StreamMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

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

	return nil
}

// EndInput closes stdin to signal no more input messages will be sent.
// This mirrors Python transport.end_input() behavior used by one-shot queries.
func (t *Transport) EndInput(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stdin == nil {
		return nil
	}

	if err := t.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	t.stdin = nil
	return nil
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

// Interrupt sends an interrupt control request to the CLI subprocess.
// Uses the control protocol (matching Python SDK behavior) rather than SIGINT.
func (t *Transport) Interrupt(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return fmt.Errorf("transport not connected")
	}

	if t.controlProtocol == nil {
		return fmt.Errorf("control protocol not initialized")
	}

	return t.controlProtocol.InterruptControl()
}

// GetMCPStatus queries the MCP server status from CLI.
func (t *Transport) GetMCPStatus(_ context.Context) (map[string]any, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return nil, fmt.Errorf("transport not connected")
	}

	if t.controlProtocol == nil {
		return nil, fmt.Errorf("control protocol not initialized")
	}

	return t.controlProtocol.GetMCPStatus()
}

// SetPermissionMode changes the permission mode during conversation.
func (t *Transport) SetPermissionMode(_ context.Context, mode string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.controlProtocol == nil {
		return fmt.Errorf("transport not connected")
	}

	return t.controlProtocol.SetPermissionMode(mode)
}

// SetModel changes the AI model during conversation.
func (t *Transport) SetModel(_ context.Context, model *string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.controlProtocol == nil {
		return fmt.Errorf("transport not connected")
	}

	return t.controlProtocol.SetModel(model)
}

// GetServerInfo returns the server initialization info.
func (t *Transport) GetServerInfo() map[string]any {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.serverInfo
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
	debugToStderr := shouldDebugToStderr(t.options)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 {
			// If stderr callback is configured, call it
			if t.options != nil && t.options.Stderr != nil {
				t.options.Stderr(line)
			} else if debugToStderr {
				_, _ = fmt.Fprintln(os.Stderr, line)
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
	maxScannerBuffer := parsing.MaxBufferSize
	if t.options != nil && t.options.MaxBufferSize != nil && *t.options.MaxBufferSize > 0 {
		maxScannerBuffer = *t.options.MaxBufferSize
	}
	if maxScannerBuffer < 64*1024 {
		maxScannerBuffer = 64 * 1024
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerBuffer)

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
			var rawMsg map[string]any
			if err := json.Unmarshal([]byte(line), &rawMsg); err == nil {
				if msgType, ok := rawMsg["type"].(string); ok {
					switch msgType {
					case "control_request", "control_response", "control_cancel_request":
						if err := t.controlProtocol.HandleIncomingMessage(msgType, []byte(line)); err != nil {
							select {
							case t.errChan <- fmt.Errorf("control protocol error: %w", err):
							case <-t.ctx.Done():
								return
							}
						}
						continue
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
		if t.controlProtocol != nil {
			t.controlProtocol.FailPendingRequests(scanErr)
		}
		select {
		case t.errChan <- scanErr:
		case <-t.ctx.Done():
		}
	}
}

func shouldPipeStderr(options *shared.Options) bool {
	if options == nil {
		return false
	}
	return options.Stderr != nil || shouldDebugToStderr(options)
}

func shouldDebugToStderr(options *shared.Options) bool {
	if options == nil || options.ExtraArgs == nil {
		return false
	}
	_, ok := options.ExtraArgs["debug-to-stderr"]
	return ok
}

// isProcessAlreadyFinishedError checks if an error indicates the process has already terminated.
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
		if isProcessAlreadyFinishedError(err) {
			return nil
		}
		killErr := t.cmd.Process.Kill()
		if killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		return nil
	}

	// Wait exactly 5 seconds
	done := make(chan error, 1)
	cmd := t.cmd
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			if strings.Contains(err.Error(), "signal:") {
				return nil
			}
		}
		return err
	case <-time.After(terminationTimeoutSeconds * time.Second):
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		<-done
		return nil
	case <-t.ctx.Done():
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessAlreadyFinishedError(killErr) {
			return killErr
		}
		<-done
		return nil
	}
}

// cleanup cleans up all resources
func (t *Transport) cleanup() {
	if t.stdout != nil {
		_ = t.stdout.Close()
		t.stdout = nil
	}

	// Reset state
	t.cmd = nil
}
