package query

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// ControlProtocol manages bidirectional control protocol communication.
type ControlProtocol struct {
	// Hook processor
	hookProcessor *HookProcessor

	// SDK MCP servers for handling mcp_message requests
	sdkMCPServers map[string]shared.McpSDKServer

	// Pending control responses
	pendingResponses map[string]*pendingControlResponse
	responseMu       sync.RWMutex

	// Request counter
	requestCounter int64
	counterMu      sync.Mutex

	// Context
	ctx    context.Context
	cancel context.CancelFunc

	// Write function for sending messages
	writeFn func([]byte) error

	// Initialized flag
	initialized   bool
	initializedMu sync.RWMutex
}

type pendingControlResponse struct {
	ch     chan *shared.ResponsePayload
	err    chan error
	cancel context.CancelFunc
}

// NewControlProtocol creates a new control protocol handler.
func NewControlProtocol(
	ctx context.Context,
	hookProcessor *HookProcessor,
	writeFn func([]byte) error,
	sdkMCPServers map[string]shared.McpSDKServer,
) *ControlProtocol {
	cpCtx, cancel := context.WithCancel(ctx)

	return &ControlProtocol{
		hookProcessor:    hookProcessor,
		sdkMCPServers:    sdkMCPServers,
		pendingResponses: make(map[string]*pendingControlResponse),
		ctx:              cpCtx,
		cancel:           cancel,
		writeFn:          writeFn,
	}
}

// Initialize sends initialization request to CLI.
func (cp *ControlProtocol) Initialize(agents map[string]map[string]any) (map[string]any, error) {
	// Build hooks configuration
	var hooksConfig map[string]any
	if cp.hookProcessor != nil {
		rawConfig := cp.hookProcessor.BuildInitializeConfig()
		if len(rawConfig) > 0 {
			hooksConfig = make(map[string]any, len(rawConfig))
			for k, v := range rawConfig {
				hooksConfig[k] = v
			}
		}
	}

	// Build request
	request := map[string]any{
		"subtype": shared.ControlSubtypeInitialize,
	}
	if hooksConfig != nil && len(hooksConfig) > 0 {
		request["hooks"] = hooksConfig
	}
	if agents != nil && len(agents) > 0 {
		request["agents"] = agents
	}

	// Send and wait for response
	response, err := cp.sendControlRequest(request, initializeTimeout())
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	cp.initializedMu.Lock()
	cp.initialized = true
	cp.initializedMu.Unlock()

	return response, nil
}

func initializeTimeout() time.Duration {
	const minTimeout = 60 * time.Second
	raw := os.Getenv("CLAUDE_CODE_STREAM_CLOSE_TIMEOUT")
	if raw == "" {
		return minTimeout
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return minTimeout
	}
	if ms <= 0 {
		return minTimeout
	}
	timeout := time.Duration(ms) * time.Millisecond
	if timeout < minTimeout {
		return minTimeout
	}
	return timeout
}

// GetMCPStatus queries the MCP status from CLI.
func (cp *ControlProtocol) GetMCPStatus() (map[string]any, error) {
	request := map[string]any{
		"subtype": shared.ControlSubtypeMCPStatus,
	}
	return cp.sendControlRequest(request, 60*time.Second)
}

// SetPermissionMode changes the permission mode during conversation.
func (cp *ControlProtocol) SetPermissionMode(mode string) error {
	request := map[string]any{
		"subtype": shared.ControlSubtypeSetPermissionMode,
		"mode":    mode,
	}
	_, err := cp.sendControlRequest(request, 60*time.Second)
	return err
}

// SetModel changes the AI model during conversation.
func (cp *ControlProtocol) SetModel(model *string) error {
	request := map[string]any{
		"subtype": shared.ControlSubtypeSetModel,
	}
	if model != nil {
		request["model"] = *model
	} else {
		request["model"] = nil
	}
	_, err := cp.sendControlRequest(request, 60*time.Second)
	return err
}

// InterruptControl sends an interrupt control request to CLI.
func (cp *ControlProtocol) InterruptControl() error {
	request := map[string]any{
		"subtype": shared.ControlSubtypeInterrupt,
	}
	_, err := cp.sendControlRequest(request, 60*time.Second)
	return err
}

// IsInitialized returns whether the control protocol is initialized.
func (cp *ControlProtocol) IsInitialized() bool {
	cp.initializedMu.RLock()
	defer cp.initializedMu.RUnlock()
	return cp.initialized
}

// HandleIncomingMessage handles an incoming control message from CLI.
func (cp *ControlProtocol) HandleIncomingMessage(msgType string, data []byte) error {
	switch msgType {
	case shared.ControlTypeResponse:
		return cp.handleControlResponse(data)

	case shared.ControlTypeRequest:
		return cp.handleControlRequest(data)

	case shared.ControlTypeCancelRequest:
		return cp.handleControlCancel(data)

	default:
		return fmt.Errorf("unknown control message type: %s", msgType)
	}
}

// handleControlResponse processes a control response from CLI.
func (cp *ControlProtocol) handleControlResponse(data []byte) error {
	var response shared.ControlResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal control response: %w", err)
	}

	requestID := response.Response.RequestID

	cp.responseMu.RLock()
	pending, exists := cp.pendingResponses[requestID]
	cp.responseMu.RUnlock()

	if !exists {
		// Response for unknown request, ignore
		return nil
	}

	// Send response to waiting goroutine
	if response.Response.Subtype == shared.ControlSubtypeError {
		pending.err <- fmt.Errorf("control request error: %s", response.Response.Error)
	} else {
		pending.ch <- &response.Response
	}

	// Clean up
	cp.responseMu.Lock()
	delete(cp.pendingResponses, requestID)
	cp.responseMu.Unlock()

	return nil
}

// handleControlRequest processes a control request from CLI.
func (cp *ControlProtocol) handleControlRequest(data []byte) error {
	var request shared.ControlRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to unmarshal control request: %w", err)
	}

	// Process in goroutine to avoid blocking message reader
	go cp.processControlRequest(&request)

	return nil
}

// processControlRequest processes a control request and sends response.
func (cp *ControlProtocol) processControlRequest(request *shared.ControlRequest) {
	var responseData map[string]any
	var err error

	switch request.Request.Subtype {
	case shared.ControlSubtypeCanUseTool:
		responseData, err = cp.handleCanUseTool(request.Request.Data)

	case shared.ControlSubtypeHookCallback:
		responseData, err = cp.handleHookCallback(request.Request.Data)

	case shared.ControlSubtypeMCPMessage:
		responseData, err = cp.handleMCPMessage(request.Request.Data)

	default:
		err = fmt.Errorf("unsupported control request subtype: %s", request.Request.Subtype)
	}

	// Build and send response
	var response shared.ControlResponse
	if err != nil {
		response = shared.ControlResponse{
			Type: shared.ControlTypeResponse,
			Response: shared.ResponsePayload{
				Subtype:   shared.ControlSubtypeError,
				RequestID: request.RequestID,
				Error:     err.Error(),
			},
		}
	} else {
		response = shared.ControlResponse{
			Type: shared.ControlTypeResponse,
			Response: shared.ResponsePayload{
				Subtype:   shared.ControlSubtypeSuccess,
				RequestID: request.RequestID,
				Response:  responseData,
			},
		}
	}

	// Send response
	responseBytes, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		// Log error but can't do much else
		return
	}

	// Add newline for line-delimited JSON
	responseBytes = append(responseBytes, '\n')

	if writeErr := cp.writeFn(responseBytes); writeErr != nil {
		// Log error but can't do much else
		return
	}
}

// handleCanUseTool handles tool permission request.
func (cp *ControlProtocol) handleCanUseTool(data map[string]any) (map[string]any, error) {
	// Convert data to CanUseToolRequest
	request := &shared.CanUseToolRequest{
		Subtype: shared.ControlSubtypeCanUseTool,
	}

	if toolName, ok := data["tool_name"].(string); ok {
		request.ToolName = toolName
	}

	if input, ok := data["input"].(map[string]any); ok {
		request.Input = input
	}

	if suggestions, ok := data["permission_suggestions"].([]any); ok {
		request.PermissionSuggestions = suggestions
	}

	// Process through hook processor
	response, err := cp.hookProcessor.ProcessCanUseTool(request)
	if err != nil {
		return nil, err
	}

	// Convert to map
	result := map[string]any{
		"behavior": response.Behavior,
	}

	if response.UpdatedInput != nil {
		result["updatedInput"] = response.UpdatedInput
	}

	if response.UpdatedPermissions != nil {
		result["updatedPermissions"] = response.UpdatedPermissions
	}

	if response.Message != "" {
		result["message"] = response.Message
	}

	if response.Interrupt {
		result["interrupt"] = response.Interrupt
	}

	return result, nil
}

// handleHookCallback handles hook callback request.
func (cp *ControlProtocol) handleHookCallback(data map[string]any) (map[string]any, error) {
	// Convert data to HookCallbackRequest
	request := &shared.HookCallbackRequest{
		Subtype: shared.ControlSubtypeHookCallback,
	}

	if callbackID, ok := data["callback_id"].(string); ok {
		request.CallbackID = callbackID
	}

	if input, ok := data["input"].(map[string]any); ok {
		request.Input = input
	}

	if toolUseID, ok := data["tool_use_id"].(string); ok {
		request.ToolUseID = &toolUseID
	}

	// Process through hook processor
	output, err := cp.hookProcessor.ProcessHookCallback(request)
	if err != nil {
		return nil, err
	}

	// Hook output is already a map[string]any
	return output, nil
}

// handleMCPMessage handles an MCP message request from CLI.
func (cp *ControlProtocol) handleMCPMessage(data map[string]any) (map[string]any, error) {
	serverName, ok := data["server_name"].(string)
	if !ok || serverName == "" {
		return nil, fmt.Errorf("missing server_name for MCP request")
	}

	mcpMessage, ok := data["message"].(map[string]any)
	if !ok || mcpMessage == nil {
		return nil, fmt.Errorf("missing message for MCP request")
	}

	// Find the SDK MCP server
	server, exists := cp.sdkMCPServers[serverName]
	if !exists {
		// Return JSONRPC error response
		return map[string]any{
			"mcp_response": map[string]any{
				"jsonrpc": "2.0",
				"id":      mcpMessage["id"],
				"error": map[string]any{
					"code":    -32601,
					"message": fmt.Sprintf("Server '%s' not found", serverName),
				},
			},
		}, nil
	}

	// Marshal the JSONRPC message for the server
	reqBytes, err := json.Marshal(mcpMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP message: %w", err)
	}

	// Route to the SDK MCP server
	respBytes, err := server.HandleJSONRPC(cp.ctx, reqBytes)
	if err != nil {
		// Return JSONRPC error response
		return map[string]any{
			"mcp_response": map[string]any{
				"jsonrpc": "2.0",
				"id":      mcpMessage["id"],
				"error": map[string]any{
					"code":    -32603,
					"message": err.Error(),
				},
			},
		}, nil
	}

	// Parse the response
	var mcpResponse map[string]any
	if err := json.Unmarshal(respBytes, &mcpResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MCP response: %w", err)
	}

	return map[string]any{
		"mcp_response": mcpResponse,
	}, nil
}

// handleControlCancel handles control cancellation request.
func (cp *ControlProtocol) handleControlCancel(data []byte) error {
	// TODO: Implement cancellation support
	return nil
}

// sendControlRequest sends a control request and waits for response.
func (cp *ControlProtocol) sendControlRequest(
	request map[string]any,
	timeout time.Duration,
) (map[string]any, error) {
	// Generate unique request ID
	requestID := cp.generateRequestID()

	// Create pending response
	pending := &pendingControlResponse{
		ch:  make(chan *shared.ResponsePayload, 1),
		err: make(chan error, 1),
	}

	// Register pending response
	cp.responseMu.Lock()
	cp.pendingResponses[requestID] = pending
	cp.responseMu.Unlock()

	// Build control request
	controlReq := shared.ControlRequest{
		Type:      shared.ControlTypeRequest,
		RequestID: requestID,
		Request: shared.RequestPayload{
			Subtype: request["subtype"].(string),
			Data:    request,
		},
	}

	// Marshal and send
	reqBytes, err := json.Marshal(controlReq)
	if err != nil {
		cp.responseMu.Lock()
		delete(cp.pendingResponses, requestID)
		cp.responseMu.Unlock()
		return nil, fmt.Errorf("failed to marshal control request: %w", err)
	}

	// Add newline for line-delimited JSON
	reqBytes = append(reqBytes, '\n')

	if err := cp.writeFn(reqBytes); err != nil {
		cp.responseMu.Lock()
		delete(cp.pendingResponses, requestID)
		cp.responseMu.Unlock()
		return nil, fmt.Errorf("failed to send control request: %w", err)
	}

	// Wait for response with timeout
	ctx, cancel := context.WithTimeout(cp.ctx, timeout)
	defer cancel()
	pending.cancel = cancel

	select {
	case response := <-pending.ch:
		return response.Response, nil

	case err := <-pending.err:
		return nil, err

	case <-ctx.Done():
		cp.responseMu.Lock()
		delete(cp.pendingResponses, requestID)
		cp.responseMu.Unlock()
		return nil, fmt.Errorf("control request timeout: %s", request["subtype"])
	}
}

// generateRequestID generates a unique request ID.
func (cp *ControlProtocol) generateRequestID() string {
	cp.counterMu.Lock()
	defer cp.counterMu.Unlock()

	cp.requestCounter++
	return fmt.Sprintf("req_%d_%d", cp.requestCounter, time.Now().UnixNano())
}

// RewindFiles sends a rewind_files request to the CLI.
// Requires file checkpointing to be enabled via the EnableFileCheckpointing option.
func (cp *ControlProtocol) RewindFiles(userMessageID string) error {
	request := map[string]any{
		"subtype":         shared.ControlSubtypeRewindFiles,
		"user_message_id": userMessageID,
	}

	_, err := cp.sendControlRequest(request, 60*time.Second)
	return err
}

// FailPendingRequests signals all pending control requests to fail with the given error.
// This is called when a fatal error occurs in the message reader to propagate errors
// immediately instead of waiting for timeouts.
func (cp *ControlProtocol) FailPendingRequests(err error) {
	cp.responseMu.Lock()
	defer cp.responseMu.Unlock()

	for _, pending := range cp.pendingResponses {
		select {
		case pending.err <- err:
		default:
			// Channel already has an error or is closed
		}
	}
}

// Close closes the control protocol handler.
func (cp *ControlProtocol) Close() error {
	cp.cancel()

	// Cancel all pending responses
	cp.responseMu.Lock()
	for _, pending := range cp.pendingResponses {
		if pending.cancel != nil {
			pending.cancel()
		}
	}
	cp.pendingResponses = make(map[string]*pendingControlResponse)
	cp.responseMu.Unlock()

	return nil
}
