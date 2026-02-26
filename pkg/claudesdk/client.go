package claudesdk

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/discovery"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/transport"
)

const defaultSessionID = "default"

// Client provides bidirectional streaming communication with Claude Code CLI.
type Client interface {
	Connect(ctx context.Context, prompt ...StreamMessage) error
	Disconnect() error
	Query(ctx context.Context, prompt string) error
	QueryWithSession(ctx context.Context, prompt string, sessionID string) error
	QueryStream(ctx context.Context, messages <-chan StreamMessage) error
	ReceiveMessages(ctx context.Context) <-chan Message
	ReceiveResponse(ctx context.Context) MessageIterator
	Interrupt(ctx context.Context) error
	RewindFiles(ctx context.Context, userMessageID string) error
	GetMCPStatus(ctx context.Context) (map[string]any, error)
	SetPermissionMode(ctx context.Context, mode string) error
	SetModel(ctx context.Context, model *string) error
	GetServerInfo() map[string]any
}

// ClientImpl implements the Client interface.
type ClientImpl struct {
	mu              sync.RWMutex
	transport       Transport
	customTransport Transport // For testing with WithTransport
	options         *Options
	connected       bool
	msgChan         <-chan Message
	errChan         <-chan error
}

// NewClient creates a new Client with the given options.
func NewClient(opts ...Option) Client {
	options := NewOptions(opts...)
	client := &ClientImpl{
		options: options,
	}
	return client
}

// NewClientWithTransport creates a new Client with a custom transport (for testing).
func NewClientWithTransport(transport Transport, opts ...Option) Client {
	options := NewOptions(opts...)
	return &ClientImpl{
		customTransport: transport,
		options:         options,
	}
}

// WithClient provides Go-idiomatic resource management equivalent to Python SDK's async context manager.
// It automatically connects to Claude Code CLI, executes the provided function, and ensures proper cleanup.
// This eliminates the need for manual Connect/Disconnect calls and prevents resource leaks.
//
// The function follows Go's established resource management patterns using defer for guaranteed cleanup,
// similar to how database connections, files, and other resources are typically managed in Go.
//
// Example - Basic usage:
//
//	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
//	    return client.Query(ctx, "What is 2+2?")
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Example - With configuration options:
//
//	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
//	    if err := client.Query(ctx, "Calculate the area of a circle with radius 5"); err != nil {
//	        return err
//	    }
//
//	    // Process responses
//	    for msg := range client.ReceiveMessages(ctx) {
//	        if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
//	            fmt.Println("Claude:", assistantMsg.Content[0].(*claudecode.TextBlock).Text)
//	        }
//	    }
//	    return nil
//	}, claudecode.WithSystemPrompt("You are a helpful math tutor"),
//	   claudecode.WithAllowedTools("Read", "Write"))
//
// The client will be automatically connected before fn is called and disconnected after fn returns,
// even if fn returns an error or panics. This provides 100% functional parity with Python SDK's
// 'async with ClaudeSDKClient()' pattern while using idiomatic Go resource management.
//
// Parameters:
//   - ctx: Context for connection management and cancellation
//   - fn: Function to execute with the connected client
//   - opts: Optional client configuration options
//
// Returns an error if connection fails or if fn returns an error.
// Disconnect errors are handled gracefully without overriding the original error from fn.
func WithClient(ctx context.Context, fn func(Client) error, opts ...Option) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	client := NewClient(opts...)

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect client: %w", err)
	}

	defer func() {
		// Following Go idiom: cleanup errors don't override the original error
		// This matches patterns in database/sql, os.File, and other stdlib packages
		if disconnectErr := client.Disconnect(); disconnectErr != nil {
			// Log cleanup errors but don't return them to preserve the original error
			// This follows the standard Go pattern for resource cleanup
			_ = disconnectErr // Explicitly acknowledge we're ignoring this error
		}
	}()

	return fn(client)
}

// WithClientTransport provides Go-idiomatic resource management with a custom transport for testing.
// This is the testing-friendly version of WithClient that accepts an explicit transport parameter.
//
// Usage in tests:
//
//	transport := newClientMockTransport()
//	err := WithClientTransport(ctx, transport, func(client claudecode.Client) error {
//	    return client.Query(ctx, "What is 2+2?")
//	}, opts...)
//
// Parameters:
//   - ctx: Context for connection management and cancellation
//   - transport: Custom transport to use (typically a mock for testing)
//   - fn: Function to execute with the connected client
//   - opts: Optional client configuration options
//
// Returns an error if connection fails or if fn returns an error.
// Disconnect errors are handled gracefully without overriding the original error from fn.
func WithClientTransport(ctx context.Context, transport Transport, fn func(Client) error, opts ...Option) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	client := NewClientWithTransport(transport, opts...)

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect client: %w", err)
	}

	defer func() {
		// Following Go idiom: cleanup errors don't override the original error
		if disconnectErr := client.Disconnect(); disconnectErr != nil {
			// Log cleanup errors but don't return them to preserve the original error
			_ = disconnectErr // Explicitly acknowledge we're ignoring this error
		}
	}()

	return fn(client)
}

// validateOptions validates the client configuration options
func (c *ClientImpl) validateOptions() error {
	if c.options == nil {
		return nil // Nil options are acceptable (use defaults)
	}

	if err := c.options.Validate(); err != nil {
		return err
	}

	// Validate working directory
	if c.options.Cwd != nil {
		if _, err := os.Stat(*c.options.Cwd); os.IsNotExist(err) {
			return fmt.Errorf("working directory does not exist: %s", *c.options.Cwd)
		}
	}

	// Validate permission mode
	if c.options.PermissionMode != nil {
		validModes := map[PermissionMode]bool{
			PermissionModeDefault:           true,
			PermissionModeAcceptEdits:       true,
			PermissionModePlan:              true,
			PermissionModeBypassPermissions: true,
		}
		if !validModes[*c.options.PermissionMode] {
			return fmt.Errorf("invalid permission mode: %s", string(*c.options.PermissionMode))
		}
	}

	// Match Python SDK behavior: these two are mutually exclusive.
	if c.options.CanUseTool != nil && c.options.PermissionPromptToolName != nil {
		return fmt.Errorf(
			"can_use_tool callback cannot be used with permission_prompt_tool_name. Please use one or the other",
		)
	}

	return nil
}

// transportOptions returns options prepared for transport startup.
// Match Python SDK behavior: when can_use_tool is set, automatically use stdio prompt tool.
func (c *ClientImpl) transportOptions() *Options {
	if c.options == nil {
		return nil
	}
	if c.options.CanUseTool == nil {
		return c.options
	}

	cloned := *c.options
	stdio := "stdio"
	cloned.PermissionPromptToolName = &stdio
	return &cloned
}

// Connect establishes a connection to the Claude Code CLI.
func (c *ClientImpl) Connect(ctx context.Context, prompts ...StreamMessage) error {
	// Check context before acquiring lock
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check context again after acquiring lock
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Validate configuration before connecting
	if err := c.validateOptions(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	transportOptions := c.transportOptions()

	// Match Python client behavior: allow repeated connect() calls by
	// tearing down any existing connection first.
	if c.connected && c.transport != nil {
		if err := c.transport.Close(); err != nil {
			return fmt.Errorf("failed to close existing transport before reconnect: %w", err)
		}
		c.connected = false
		c.transport = nil
		c.msgChan = nil
		c.errChan = nil
	}

	// Use custom transport if provided, otherwise create default
	if c.customTransport != nil {
		c.transport = c.customTransport
	} else {
		var cliPath string
		if c.options != nil && c.options.CLIPath != nil && *c.options.CLIPath != "" {
			cliPath = *c.options.CLIPath
		} else {
			var err error
			cliPath, err = discovery.FindCLI()
			if err != nil {
				return fmt.Errorf("claude CLI not found: %w", err)
			}
		}

		// Create subprocess transport (always uses streaming mode)
		c.transport = transport.New(cliPath, transportOptions, "sdk-go-client", Version)
	}

	// Connect the transport
	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Get message channels
	c.msgChan, c.errChan = c.transport.ReceiveMessages(ctx)

	c.connected = true

	// Optional initial prompts, sent after connection is ready.
	for _, msg := range prompts {
		if err := c.transport.SendMessage(ctx, msg); err != nil {
			_ = c.transport.Close()
			c.connected = false
			c.transport = nil
			c.msgChan = nil
			c.errChan = nil
			return fmt.Errorf("failed to send initial prompt: %w", err)
		}
	}

	return nil
}

// Disconnect closes the connection to the Claude Code CLI.
func (c *ClientImpl) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transport != nil && c.connected {
		if err := c.transport.Close(); err != nil {
			return fmt.Errorf("failed to close transport: %w", err)
		}
	}
	c.connected = false
	c.transport = nil
	c.msgChan = nil
	c.errChan = nil
	return nil
}

// Query sends a simple text query using the default session.
// This is equivalent to QueryWithSession(ctx, prompt, "default").
//
// Example:
//
//	client.Query(ctx, "What is Go?")
func (c *ClientImpl) Query(ctx context.Context, prompt string) error {
	return c.queryWithSession(ctx, prompt, defaultSessionID)
}

// QueryWithSession sends a simple text query using the specified session ID.
// Each session maintains its own conversation context, allowing for isolated
// conversations within the same client connection.
//
// If sessionID is empty, it defaults to "default".
//
// Example:
//
//	client.QueryWithSession(ctx, "Remember this", "my-session")
//	client.QueryWithSession(ctx, "What did I just say?", "my-session") // Remembers context
//	client.Query(ctx, "What did I just say?")                          // Won't remember, different session
func (c *ClientImpl) QueryWithSession(ctx context.Context, prompt string, sessionID string) error {
	// Use default session if empty session ID provided
	if sessionID == "" {
		sessionID = defaultSessionID
	}
	return c.queryWithSession(ctx, prompt, sessionID)
}

// queryWithSession is the internal implementation for sending queries with session management.
func (c *ClientImpl) queryWithSession(ctx context.Context, prompt string, sessionID string) error {
	// Check context before proceeding
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	// Check context again after acquiring connection info
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Create user message in Python SDK compatible format
	streamMsg := StreamMessage{
		Type: "user",
		Message: map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
		ParentToolUseID: nil,
		SessionID:       sessionID,
	}

	// Send message via transport (without holding mutex to avoid blocking other operations)
	return transport.SendMessage(ctx, streamMsg)
}

// QueryStream sends a stream of messages.
func (c *ClientImpl) QueryStream(ctx context.Context, messages <-chan StreamMessage) error {
	if messages == nil {
		return fmt.Errorf("messages stream is required")
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			if msg.SessionID == "" {
				msg.SessionID = defaultSessionID
			}
			if err := transport.SendMessage(ctx, msg); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ReceiveMessages returns a channel of incoming messages.
func (c *ClientImpl) ReceiveMessages(_ context.Context) <-chan Message {
	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	msgChan := c.msgChan
	errChan := c.errChan
	c.mu.RUnlock()

	if !connected || msgChan == nil {
		// Surface connection misuse explicitly instead of silently returning no messages.
		errorChan := make(chan Message, 1)
		errorChan <- &SystemMessage{
			Subtype: "error",
			Data: map[string]any{
				"type":    MessageTypeSystem,
				"subtype": "error",
				"error":   "not connected. call connect() first",
			},
		}
		close(errorChan)
		return errorChan
	}

	// Forward messages while surfacing transport errors as system messages.
	out := make(chan Message)
	go func() {
		defer close(out)
		localMsgChan := msgChan
		localErrChan := errChan
		for {
			select {
			case msg, ok := <-localMsgChan:
				if !ok {
					return
				}
				out <- msg
			case err, ok := <-localErrChan:
				if !ok {
					localErrChan = nil
					continue
				}
				if err != nil {
					out <- &SystemMessage{
						Subtype: "error",
						Data: map[string]any{
							"type":    MessageTypeSystem,
							"subtype": "error",
							"error":   err.Error(),
						},
					}
				}
				return
			}
		}
	}()
	return out
}

// ReceiveResponse returns an iterator for the response messages.
func (c *ClientImpl) ReceiveResponse(_ context.Context) MessageIterator {
	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	msgChan := c.msgChan
	errChan := c.errChan
	c.mu.RUnlock()

	if !connected || msgChan == nil {
		return &clientErrorIterator{
			err: fmt.Errorf("not connected. call connect() first"),
		}
	}

	// Create a simple iterator over the message channel
	return &clientIterator{
		msgChan:      msgChan,
		errChan:      errChan,
		stopOnResult: true,
	}
}

type clientErrorIterator struct {
	err      error
	consumed bool
}

func (it *clientErrorIterator) Next(context.Context) (Message, error) {
	if it.consumed {
		return nil, ErrNoMoreMessages
	}
	it.consumed = true
	if it.err != nil {
		return nil, it.err
	}
	return nil, ErrNoMoreMessages
}

func (it *clientErrorIterator) Close() error {
	it.consumed = true
	return nil
}

// Interrupt sends an interrupt signal to stop the current operation.
func (c *ClientImpl) Interrupt(ctx context.Context) error {
	// Check context before proceeding
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	return transport.Interrupt(ctx)
}

// RewindFiles rewinds tracked files to their state at a specific user message.
//
// Requires file checkpointing to be enabled via the WithEnableFileCheckpointing(true) option
// when creating the Client.
//
// Parameters:
//   - userMessageID: UUID of the user message to rewind to. This should be
//     the UUID field from a UserMessage received during the conversation.
//
// Example:
//
//	client := claudesdk.NewClient(claudesdk.WithEnableFileCheckpointing(true))
//	// ... after making changes ...
//	err := client.RewindFiles(ctx, checkpointID)
func (c *ClientImpl) RewindFiles(ctx context.Context, userMessageID string) error {
	// Check context before proceeding
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check connection status with read lock
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	return transport.RewindFiles(ctx, userMessageID)
}

// GetMCPStatus queries the MCP server status from CLI.
func (c *ClientImpl) GetMCPStatus(ctx context.Context) (map[string]any, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return nil, fmt.Errorf("client not connected")
	}

	return transport.GetMCPStatus(ctx)
}

// SetPermissionMode changes the permission mode during conversation.
func (c *ClientImpl) SetPermissionMode(ctx context.Context, mode string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	return transport.SetPermissionMode(ctx, mode)
}

// SetModel changes the AI model during conversation.
func (c *ClientImpl) SetModel(ctx context.Context, model *string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return fmt.Errorf("client not connected")
	}

	return transport.SetModel(ctx, model)
}

// GetServerInfo returns the server initialization info.
func (c *ClientImpl) GetServerInfo() map[string]any {
	c.mu.RLock()
	connected := c.connected
	transport := c.transport
	c.mu.RUnlock()

	if !connected || transport == nil {
		return nil
	}

	return transport.GetServerInfo()
}

// clientIterator implements MessageIterator for client message reception
type clientIterator struct {
	msgChan      <-chan Message
	errChan      <-chan error
	closed       bool
	stopOnResult bool
}

func (ci *clientIterator) Next(ctx context.Context) (Message, error) {
	if ci.closed {
		return nil, ErrNoMoreMessages
	}

	for {
		select {
		case msg, ok := <-ci.msgChan:
			if !ok {
				ci.closed = true
				return nil, ErrNoMoreMessages
			}
			if ci.stopOnResult {
				if _, ok := msg.(*ResultMessage); ok {
					ci.closed = true
				}
			}
			return msg, nil
		case err, ok := <-ci.errChan:
			if !ok {
				ci.errChan = nil
				continue
			}
			ci.closed = true
			return nil, err
		case <-ctx.Done():
			ci.closed = true
			return nil, ctx.Err()
		}
	}
}

func (ci *clientIterator) Close() error {
	ci.closed = true
	return nil
}
