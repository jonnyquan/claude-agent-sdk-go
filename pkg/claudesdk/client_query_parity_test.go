package claudesdk

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockTransport struct {
	mu            sync.Mutex
	msgChan       chan Message
	errChan       chan error
	sentMessages  []StreamMessage
	sendErr       error
	endInputErr   error
	endInputCalls int
	connectCalled int
	closeCalled   int
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		msgChan: make(chan Message, 16),
		errChan: make(chan error, 16),
	}
}

func (m *mockTransport) Connect(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectCalled++
	return nil
}

func (m *mockTransport) SendMessage(_ context.Context, message StreamMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

func (m *mockTransport) EndInput(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endInputCalls++
	if m.endInputErr != nil {
		return m.endInputErr
	}
	return nil
}

func (m *mockTransport) ReceiveMessages(context.Context) (<-chan Message, <-chan error) {
	return m.msgChan, m.errChan
}

func (m *mockTransport) Interrupt(context.Context) error {
	return nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled++
	return nil
}

func (m *mockTransport) RewindFiles(context.Context, string) error {
	return nil
}

func (m *mockTransport) GetMCPStatus(context.Context) (map[string]any, error) {
	return map[string]any{}, nil
}

func (m *mockTransport) SetPermissionMode(context.Context, string) error {
	return nil
}

func (m *mockTransport) SetModel(context.Context, *string) error {
	return nil
}

func (m *mockTransport) GetServerInfo() map[string]any {
	return map[string]any{}
}

func (m *mockTransport) snapshotSentMessages() []StreamMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]StreamMessage, len(m.sentMessages))
	copy(out, m.sentMessages)
	return out
}

func (m *mockTransport) closeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

func (m *mockTransport) endInputCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.endInputCalls
}

func allowAllPermissionCallback(string, map[string]any, ToolPermissionContext) (PermissionResult, error) {
	return NewPermissionAllow(nil, nil), nil
}

func TestQueryRejectsCanUseToolInOneShotMode(t *testing.T) {
	t.Parallel()

	_, err := Query(context.Background(), "hello", WithCanUseTool(allowAllPermissionCallback))
	if err == nil {
		t.Fatal("expected error when can_use_tool is used with Query()")
	}
	if !strings.Contains(err.Error(), "requires streaming mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryWithTransportRejectsCanUseToolInOneShotMode(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	_, err := QueryWithTransport(
		context.Background(),
		"hello",
		transport,
		WithCanUseTool(allowAllPermissionCallback),
	)
	if err == nil {
		t.Fatal("expected error when can_use_tool is used with QueryWithTransport()")
	}
	if !strings.Contains(err.Error(), "requires streaming mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryRejectsUnsupportedPluginType(t *testing.T) {
	t.Parallel()

	_, err := Query(
		context.Background(),
		"hello",
		WithPlugins(PluginConfig{Type: PluginType("unsupported"), Path: "/tmp/plugin"}),
	)
	if err == nil {
		t.Fatal("expected error for unsupported plugin type")
	}
	if !strings.Contains(err.Error(), "unsupported plugin type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueryIteratorStopsWhenStreamEnds(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	transport.msgChan <- &ResultMessage{Subtype: "success", SessionID: "default"}
	close(transport.msgChan)

	iter, err := QueryWithTransport(context.Background(), "hello", transport)
	if err != nil {
		t.Fatalf("QueryWithTransport returned error: %v", err)
	}

	msg, err := iter.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}
	if _, ok := msg.(*ResultMessage); !ok {
		t.Fatalf("expected ResultMessage, got %T", msg)
	}

	_, err = iter.Next(context.Background())
	if !errors.Is(err, ErrNoMoreMessages) {
		t.Fatalf("expected ErrNoMoreMessages, got %v", err)
	}
	if transport.closeCount() != 1 {
		t.Fatalf("expected transport Close() to be called once, got %d", transport.closeCount())
	}
}

func TestQueryWithTransportEndsInputAfterPrompt(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	transport.msgChan <- &ResultMessage{Subtype: "success", SessionID: "default"}
	close(transport.msgChan)

	iter, err := QueryWithTransport(context.Background(), "hello", transport)
	if err != nil {
		t.Fatalf("QueryWithTransport returned error: %v", err)
	}

	_, err = iter.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() returned error: %v", err)
	}

	sent := transport.snapshotSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected one sent message, got %d", len(sent))
	}
	if sent[0].SessionID != "" {
		t.Fatalf("expected empty session_id for one-shot query prompt, got %q", sent[0].SessionID)
	}

	if transport.endInputCount() != 1 {
		t.Fatalf("expected EndInput() once, got %d", transport.endInputCount())
	}
}

func TestReceiveResponseStopsAtResultMessage(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	assistant := &AssistantMessage{
		Content: []ContentBlock{&TextBlock{Text: "hello"}},
		Model:   "claude-sonnet-4-5",
	}
	result := &ResultMessage{Subtype: "success", SessionID: "default"}
	later := &AssistantMessage{
		Content: []ContentBlock{&TextBlock{Text: "should-not-be-read"}},
		Model:   "claude-sonnet-4-5",
	}
	transport.msgChan <- assistant
	transport.msgChan <- result
	transport.msgChan <- later

	client := NewClientWithTransport(transport)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	iter := client.ReceiveResponse(context.Background())
	if iter == nil {
		t.Fatal("expected non-nil iterator")
	}

	msg1, err := iter.Next(context.Background())
	if err != nil {
		t.Fatalf("first Next() returned error: %v", err)
	}
	if _, ok := msg1.(*AssistantMessage); !ok {
		t.Fatalf("expected AssistantMessage, got %T", msg1)
	}

	msg2, err := iter.Next(context.Background())
	if err != nil {
		t.Fatalf("second Next() returned error: %v", err)
	}
	if _, ok := msg2.(*ResultMessage); !ok {
		t.Fatalf("expected ResultMessage, got %T", msg2)
	}

	_, err = iter.Next(context.Background())
	if !errors.Is(err, ErrNoMoreMessages) {
		t.Fatalf("expected ErrNoMoreMessages after result, got %v", err)
	}
}

func TestReceiveResponseWhenNotConnectedReturnsError(t *testing.T) {
	t.Parallel()

	client := NewClient()
	iter := client.ReceiveResponse(context.Background())
	if iter == nil {
		t.Fatal("expected non-nil iterator")
	}

	_, err := iter.Next(context.Background())
	if err == nil {
		t.Fatal("expected not connected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not connected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReceiveMessagesWhenNotConnectedReturnsSystemErrorMessage(t *testing.T) {
	t.Parallel()

	client := NewClient()
	msgCh := client.ReceiveMessages(context.Background())

	msg, ok := <-msgCh
	if !ok {
		t.Fatal("expected one error system message when not connected")
	}

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected SystemMessage, got %T", msg)
	}
	if sys.Subtype != "error" {
		t.Fatalf("expected subtype 'error', got %q", sys.Subtype)
	}
	if got, _ := sys.Data["error"].(string); got == "" || !strings.Contains(strings.ToLower(got), "not connected") {
		t.Fatalf("unexpected error payload: %#v", sys.Data)
	}

	if _, stillOpen := <-msgCh; stillOpen {
		t.Fatal("expected channel to close after single error message")
	}
}

func TestClientValidateOptionsRejectsCanUseToolConflict(t *testing.T) {
	t.Parallel()

	client := &ClientImpl{
		options: NewOptions(
			WithCanUseTool(allowAllPermissionCallback),
			WithPermissionPromptToolName("custom-tool"),
		),
	}

	err := client.validateOptions()
	if err == nil {
		t.Fatal("expected validation error for can_use_tool + permission_prompt_tool_name conflict")
	}
	if !strings.Contains(err.Error(), "cannot be used with permission_prompt_tool_name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientTransportOptionsAutoSetStdioForCanUseTool(t *testing.T) {
	t.Parallel()

	options := NewOptions(WithCanUseTool(allowAllPermissionCallback))
	client := &ClientImpl{options: options}

	transportOptions := client.transportOptions()
	if transportOptions == nil || transportOptions.PermissionPromptToolName == nil {
		t.Fatal("expected permission_prompt_tool_name to be set")
	}
	if got := *transportOptions.PermissionPromptToolName; got != "stdio" {
		t.Fatalf("expected permission_prompt_tool_name=stdio, got %q", got)
	}

	if options.PermissionPromptToolName != nil {
		t.Fatal("transportOptions should not mutate original options")
	}
}

func TestClientConnectSendsInitialPrompts(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	client := NewClientWithTransport(transport)

	initial := StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": "hello",
		},
		SessionID: "default",
	}

	if err := client.Connect(context.Background(), initial); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	sent := transport.snapshotSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected one initial message, got %d", len(sent))
	}
	if got := sent[0].SessionID; got != "default" {
		t.Fatalf("expected session_id default, got %q", got)
	}
}

func TestClientQueryStreamDefaultsSessionID(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	client := NewClientWithTransport(transport)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	input := make(chan StreamMessage, 1)
	input <- StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": "hello",
		},
	}
	close(input)

	if err := client.QueryStream(context.Background(), input); err != nil {
		t.Fatalf("QueryStream() returned error: %v", err)
	}

	sent := transport.snapshotSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected one sent message, got %d", len(sent))
	}
	if sent[0].SessionID != "default" {
		t.Fatalf("expected default session_id, got %q", sent[0].SessionID)
	}
}

func TestClientQueryStreamReturnsSendError(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	transport.sendErr = errors.New("send failed")

	client := NewClientWithTransport(transport)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	input := make(chan StreamMessage, 1)
	input <- StreamMessage{Type: "user", Message: map[string]any{"role": "user", "content": "hello"}}
	close(input)

	err := client.QueryStream(context.Background(), input)
	if err == nil {
		t.Fatal("expected send error")
	}
	if !strings.Contains(err.Error(), "send failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientQueryStreamRejectsNilMessages(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	client := NewClientWithTransport(transport)
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	if err := client.QueryStream(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil messages stream")
	}
}

func TestQueryStreamWithTransportPreservesSessionIDAndStopsAfterResults(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	input := make(chan StreamMessage, 2)
	input <- StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": "first",
		},
	}
	input <- StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": "second",
		},
	}
	close(input)

	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if len(transport.snapshotSentMessages()) == 2 {
				transport.msgChan <- &ResultMessage{Subtype: "success", SessionID: "default"}
				transport.msgChan <- &ResultMessage{Subtype: "success", SessionID: "default"}
				close(transport.msgChan)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	iter, err := QueryStreamWithTransport(context.Background(), input, transport)
	if err != nil {
		t.Fatalf("QueryStreamWithTransport() returned error: %v", err)
	}

	if _, err := iter.Next(context.Background()); err != nil {
		t.Fatalf("first Next() returned error: %v", err)
	}
	if _, err := iter.Next(context.Background()); err != nil {
		t.Fatalf("second Next() returned error: %v", err)
	}

	sent := transport.snapshotSentMessages()
	if len(sent) != 2 {
		t.Fatalf("expected 2 sent messages, got %d", len(sent))
	}
	for i, msg := range sent {
		if msg.SessionID != "" {
			t.Fatalf("expected sent message %d session_id to remain empty, got %q", i, msg.SessionID)
		}
	}

	_, err = iter.Next(context.Background())
	if !errors.Is(err, ErrNoMoreMessages) {
		t.Fatalf("expected ErrNoMoreMessages, got %v", err)
	}
	if transport.closeCount() != 1 {
		t.Fatalf("expected transport Close() once, got %d", transport.closeCount())
	}
	if transport.endInputCount() != 1 {
		t.Fatalf("expected EndInput() once, got %d", transport.endInputCount())
	}
}

func TestQueryStreamWithTransportRejectsNilMessages(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	_, err := QueryStreamWithTransport(context.Background(), nil, transport)
	if err == nil {
		t.Fatal("expected error for nil messages stream")
	}
}

func TestQueryStreamWithTransportWaitsForFirstResultBeforeEndInputWhenHooksPresent(t *testing.T) {
	t.Parallel()

	transport := newMockTransport()
	input := make(chan StreamMessage, 1)
	input <- StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": "first",
		},
	}
	close(input)

	hookOption := WithHooks(map[string][]HookMatcher{
		string(HookEventPreToolUse): {
			{
				Matcher: nil,
				Hooks:   []HookCallback{},
			},
		},
	})

	iter, err := QueryStreamWithTransport(context.Background(), input, transport, hookOption)
	if err != nil {
		t.Fatalf("QueryStreamWithTransport() returned error: %v", err)
	}

	firstMsgCh := make(chan Message, 1)
	firstErrCh := make(chan error, 1)
	go func() {
		msg, nextErr := iter.Next(context.Background())
		if nextErr != nil {
			firstErrCh <- nextErr
			return
		}
		firstMsgCh <- msg
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(transport.snapshotSentMessages()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(transport.snapshotSentMessages()) != 1 {
		t.Fatal("expected one message to be sent")
	}
	if got := transport.endInputCount(); got != 0 {
		t.Fatalf("expected EndInput() not called before first result, got %d", got)
	}

	transport.msgChan <- &ResultMessage{Subtype: "success", SessionID: "default"}
	close(transport.msgChan)

	select {
	case err := <-firstErrCh:
		t.Fatalf("first Next() returned error: %v", err)
	case msg := <-firstMsgCh:
		if _, ok := msg.(*ResultMessage); !ok {
			t.Fatalf("expected ResultMessage, got %T", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first Next() result")
	}
	if _, err := iter.Next(context.Background()); !errors.Is(err, ErrNoMoreMessages) {
		t.Fatalf("expected ErrNoMoreMessages, got %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if transport.endInputCount() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected EndInput() once after first result, got %d", transport.endInputCount())
}

func TestConfigureStreamingQueryOptionsCanUseToolConflict(t *testing.T) {
	t.Parallel()

	options := NewOptions(
		WithCanUseTool(allowAllPermissionCallback),
		WithPermissionPromptToolName("custom"),
	)
	_, err := configureStreamingQueryOptions(options)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "cannot be used with permission_prompt_tool_name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureStreamingQueryOptionsAutoSetsStdio(t *testing.T) {
	t.Parallel()

	options := NewOptions(WithCanUseTool(allowAllPermissionCallback))
	configured, err := configureStreamingQueryOptions(options)
	if err != nil {
		t.Fatalf("configureStreamingQueryOptions() returned error: %v", err)
	}
	if configured == nil || configured.PermissionPromptToolName == nil {
		t.Fatal("expected permission prompt tool name to be configured")
	}
	if got := *configured.PermissionPromptToolName; got != "stdio" {
		t.Fatalf("expected stdio permission prompt tool, got %q", got)
	}
}
