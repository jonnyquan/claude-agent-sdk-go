package claudesdk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/discovery"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
	"github.com/jonnyquan/claude-agent-sdk-go/internal/transport"
)

// ErrNoMoreMessages indicates the message iterator has no more messages.
var ErrNoMoreMessages = errors.New("no more messages")

type inputEnder interface {
	EndInput(ctx context.Context) error
}

// Query executes a one-shot query with automatic cleanup.
// This follows the Python SDK pattern but uses dependency injection for transport.
func Query(ctx context.Context, prompt string, opts ...Option) (MessageIterator, error) {
	options := NewOptions(opts...)
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	if options.CanUseTool != nil {
		return nil, fmt.Errorf(
			"can_use_tool callback requires streaming mode. use Client with Query/QueryStream instead of Query()",
		)
	}

	// For one-shot queries, create a transport that passes prompt as CLI argument
	// This matches the Python SDK behavior where prompt is passed via --print flag
	transport, err := createQueryTransport(prompt, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create query transport: %w", err)
	}

	return queryWithTransportAndOptions(ctx, prompt, transport, options)
}

// QueryStream executes a unidirectional streaming query.
//
// This mirrors Python SDK's query(prompt=AsyncIterable[...]) behavior:
// send a finite stream of messages and receive all resulting output.
func QueryStream(
	ctx context.Context,
	messages <-chan StreamMessage,
	opts ...Option,
) (MessageIterator, error) {
	if messages == nil {
		return nil, fmt.Errorf("messages stream is required")
	}

	options := NewOptions(opts...)
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	configuredOptions, err := configureStreamingQueryOptions(options)
	if err != nil {
		return nil, err
	}

	transport, err := createQueryTransport("", configuredOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create query transport: %w", err)
	}

	return queryStreamWithTransportAndOptions(ctx, messages, transport, configuredOptions)
}

// QueryWithTransport executes a query with a custom transport.
// The transport parameter is required and must not be nil.
func QueryWithTransport(
	ctx context.Context,
	prompt string,
	transport Transport,
	opts ...Option,
) (MessageIterator, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}

	options := NewOptions(opts...)
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	if options.CanUseTool != nil {
		return nil, fmt.Errorf(
			"can_use_tool callback requires streaming mode. use Client with Query/QueryStream instead of Query()",
		)
	}
	return queryWithTransportAndOptions(ctx, prompt, transport, options)
}

// QueryStreamWithTransport executes a streaming query with a custom transport.
func QueryStreamWithTransport(
	ctx context.Context,
	messages <-chan StreamMessage,
	transport Transport,
	opts ...Option,
) (MessageIterator, error) {
	if messages == nil {
		return nil, fmt.Errorf("messages stream is required")
	}
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}

	options := NewOptions(opts...)
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	configuredOptions, err := configureStreamingQueryOptions(options)
	if err != nil {
		return nil, err
	}

	return queryStreamWithTransportAndOptions(ctx, messages, transport, configuredOptions)
}

// Internal helper functions
func queryWithTransportAndOptions(
	ctx context.Context,
	prompt string,
	transport Transport,
	options *Options,
) (MessageIterator, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}

	// Create iterator that manages the transport lifecycle
	return &queryIterator{
		transport: transport,
		prompt:    prompt,
		ctx:       ctx,
		options:   options,
	}, nil
}

func queryStreamWithTransportAndOptions(
	ctx context.Context,
	messages <-chan StreamMessage,
	transport Transport,
	options *Options,
) (MessageIterator, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}

	return &queryStreamIterator{
		transport: transport,
		messages:  messages,
		ctx:       ctx,
		options:   options,
		sendErr:   make(chan error, 1),
	}, nil
}

func configureStreamingQueryOptions(options *Options) (*Options, error) {
	if options == nil {
		return nil, nil
	}
	if options.CanUseTool == nil {
		return options, nil
	}
	if options.PermissionPromptToolName != nil {
		return nil, fmt.Errorf(
			"can_use_tool callback cannot be used with permission_prompt_tool_name. Please use one or the other",
		)
	}

	cloned := *options
	stdio := "stdio"
	cloned.PermissionPromptToolName = &stdio
	return &cloned, nil
}

// queryIterator implements MessageIterator for simple queries
type queryIterator struct {
	transport Transport
	prompt    string
	ctx       context.Context
	options   *Options
	started   bool
	msgChan   <-chan Message
	errChan   <-chan error
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
}

func (qi *queryIterator) Next(_ context.Context) (Message, error) {
	qi.mu.Lock()
	if qi.closed {
		qi.mu.Unlock()
		return nil, ErrNoMoreMessages
	}

	// Initialize on first call
	if !qi.started {
		if err := qi.start(); err != nil {
			qi.mu.Unlock()
			return nil, err
		}
		qi.started = true
	}
	qi.mu.Unlock()

	// Read from message channels.
	// Closed errChan must be ignored; otherwise a closed channel can yield a
	// spurious nil error and starve msgChan.
	for {
		select {
		case msg, ok := <-qi.msgChan:
			if !ok {
				_ = qi.Close()
				return nil, ErrNoMoreMessages
			}
			return msg, nil
		case err, ok := <-qi.errChan:
			if !ok {
				qi.errChan = nil
				continue
			}
			_ = qi.Close()
			return nil, err
		case <-qi.ctx.Done():
			_ = qi.Close()
			return nil, qi.ctx.Err()
		}
	}
}

func (qi *queryIterator) Close() error {
	var err error
	qi.closeOnce.Do(func() {
		qi.mu.Lock()
		qi.closed = true
		qi.mu.Unlock()
		if qi.transport != nil {
			err = qi.transport.Close()
		}
	})
	return err
}

func (qi *queryIterator) start() error {
	// Connect to transport
	if err := qi.transport.Connect(qi.ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Get message channels
	msgChan, errChan := qi.transport.ReceiveMessages(qi.ctx)
	qi.msgChan = msgChan
	qi.errChan = errChan

	// Send the prompt
	streamMsg := StreamMessage{
		Type: "user",
		Message: map[string]interface{}{
			"role":    "user",
			"content": qi.prompt,
		},
		// Match Python one-shot query behavior: empty session_id by default.
		SessionID: "",
	}

	if err := qi.transport.SendMessage(qi.ctx, streamMsg); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Match Python one-shot query behavior: close stdin after sending prompt.
	if ender, ok := qi.transport.(inputEnder); ok {
		if err := ender.EndInput(qi.ctx); err != nil {
			return fmt.Errorf("failed to end input: %w", err)
		}
	}

	return nil
}

// queryStreamIterator implements MessageIterator for streaming query input.
type queryStreamIterator struct {
	transport Transport
	messages  <-chan StreamMessage
	ctx       context.Context
	options   *Options
	started   bool
	msgChan   <-chan Message
	errChan   <-chan error
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
	sendErr   chan error

	sendCompleted bool
	expectedTurns int
	seenResults   int

	hasInputEnder                 bool
	waitForFirstResultBeforeClose bool
	firstResultCh                 chan struct{}
	firstResultOnce               sync.Once
}

func (qi *queryStreamIterator) Next(_ context.Context) (Message, error) {
	qi.mu.Lock()
	if qi.closed {
		qi.mu.Unlock()
		return nil, ErrNoMoreMessages
	}

	if !qi.started {
		if err := qi.start(); err != nil {
			qi.mu.Unlock()
			return nil, err
		}
		qi.started = true
	}
	qi.mu.Unlock()

	for {
		select {
		case msg, ok := <-qi.msgChan:
			if !ok {
				_ = qi.Close()
				return nil, ErrNoMoreMessages
			}

			shouldClose := false
			qi.mu.Lock()
			if _, ok := msg.(*ResultMessage); ok {
				qi.seenResults++
				qi.firstResultOnce.Do(func() {
					close(qi.firstResultCh)
				})
			}
			if !qi.hasInputEnder {
				if qi.sendCompleted && qi.expectedTurns == 0 {
					shouldClose = true
				} else if qi.sendCompleted && qi.seenResults >= qi.expectedTurns {
					shouldClose = true
				}
			}
			qi.mu.Unlock()

			if shouldClose {
				_ = qi.Close()
			}
			return msg, nil

		case err, ok := <-qi.errChan:
			if !ok {
				qi.errChan = nil
				continue
			}
			_ = qi.Close()
			return nil, err

		case err := <-qi.sendErr:
			_ = qi.Close()
			return nil, err

		case <-qi.ctx.Done():
			_ = qi.Close()
			return nil, qi.ctx.Err()
		}
	}
}

func (qi *queryStreamIterator) Close() error {
	var err error
	qi.closeOnce.Do(func() {
		qi.mu.Lock()
		qi.closed = true
		qi.mu.Unlock()
		if qi.transport != nil {
			err = qi.transport.Close()
		}
	})
	return err
}

func (qi *queryStreamIterator) start() error {
	if err := qi.transport.Connect(qi.ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	msgChan, errChan := qi.transport.ReceiveMessages(qi.ctx)
	qi.msgChan = msgChan
	qi.errChan = errChan
	qi.firstResultCh = make(chan struct{})
	_, qi.hasInputEnder = qi.transport.(inputEnder)
	qi.waitForFirstResultBeforeClose = shouldWaitForFirstResultBeforeEndInput(qi.options)

	go qi.sendInput()

	return nil
}

func (qi *queryStreamIterator) sendInput() {
	for msg := range qi.messages {
		if msg.Type == "user" {
			qi.mu.Lock()
			qi.expectedTurns++
			qi.mu.Unlock()
		}
		if err := qi.transport.SendMessage(qi.ctx, msg); err != nil {
			select {
			case qi.sendErr <- fmt.Errorf("failed to send stream message: %w", err):
			default:
			}
			return
		}
	}

	if qi.hasInputEnder {
		if qi.waitForFirstResultBeforeClose {
			timeout := streamCloseTimeout()
			if timeout > 0 {
				select {
				case <-qi.firstResultCh:
				case <-time.After(timeout):
				case <-qi.ctx.Done():
				}
			}
		}
		if ender, ok := qi.transport.(inputEnder); ok {
			if err := ender.EndInput(qi.ctx); err != nil {
				select {
				case qi.sendErr <- fmt.Errorf("failed to end input: %w", err):
				default:
				}
			}
		}
	}

	qi.mu.Lock()
	qi.sendCompleted = true
	shouldCloseNow := !qi.hasInputEnder && qi.expectedTurns == 0
	qi.mu.Unlock()
	if shouldCloseNow {
		_ = qi.Close()
	}
}

func shouldWaitForFirstResultBeforeEndInput(options *Options) bool {
	if options == nil {
		return false
	}
	if len(options.Hooks) > 0 {
		return true
	}
	for _, cfg := range options.McpServers {
		if _, ok := cfg.(*shared.McpSdkServerConfig); ok {
			return true
		}
	}
	return false
}

func streamCloseTimeout() time.Duration {
	const defaultTimeoutMs = 60000
	raw := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT")
	if raw == "" {
		return time.Duration(defaultTimeoutMs) * time.Millisecond
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(defaultTimeoutMs) * time.Millisecond
	}
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

// createQueryTransport creates a transport for one-shot queries with prompt as CLI argument.
func createQueryTransport(_ string, options *Options) (Transport, error) {
	var cliPath string
	if options != nil && options.CLIPath != nil && *options.CLIPath != "" {
		cliPath = *options.CLIPath
	} else {
		var err error
		cliPath, err = discovery.FindCLI()
		if err != nil {
			return nil, err
		}
	}

	// Query iterator sends the user prompt via SendMessage() on first Next() call,
	// so use the standard transport here to avoid sending duplicate prompts.
	return transport.New(cliPath, options, "sdk-go", Version), nil
}
