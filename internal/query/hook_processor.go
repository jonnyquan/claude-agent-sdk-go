package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/jonnyquan/claude-agent-sdk-go/internal/shared"
)

// HookProcessor manages hook callbacks and processes control protocol messages.
type HookProcessor struct {
	// Hook callbacks indexed by event type
	hooks map[shared.HookEvent][]shared.HookMatcher

	// Map callback IDs to actual callback functions
	hookCallbacks map[string]shared.HookCallback

	// Precomputed matcher configs to preserve deterministic callback-ID binding.
	matcherConfigs map[shared.HookEvent][]shared.HookMatcherConfig

	// Tool permission callback
	canUseTool shared.CanUseToolCallback

	// Counter for generating callback IDs
	nextCallbackID int64

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Context for cancellation
	ctx context.Context
}

// NewHookProcessor creates a new hook processor.
func NewHookProcessor(ctx context.Context, options *shared.Options) *HookProcessor {
	hp := &HookProcessor{
		hooks:          make(map[shared.HookEvent][]shared.HookMatcher),
		hookCallbacks:  make(map[string]shared.HookCallback),
		matcherConfigs: make(map[shared.HookEvent][]shared.HookMatcherConfig),
		nextCallbackID: 0,
		ctx:            ctx,
	}

	// Load hooks from options
	if options != nil && len(options.Hooks) > 0 {
		hp.loadHooksFromOptions(options)
	}

	// Wire CanUseTool callback from options
	if options != nil && options.CanUseTool != nil {
		hp.canUseTool = options.CanUseTool
	}

	return hp
}

// loadHooksFromOptions loads hook configurations from Options.
func (hp *HookProcessor) loadHooksFromOptions(options *shared.Options) {
	hp.mu.Lock()
	defer hp.mu.Unlock()

	for eventKey, matchers := range options.Hooks {
		event := shared.HookEvent(eventKey)

		for _, matcherAny := range matchers {
			if matcher, ok := matcherAny.(shared.HookMatcher); ok {
				hp.hooks[event] = append(hp.hooks[event], matcher)
				callbackIDs := make([]string, 0, len(matcher.Hooks))

				// Register callbacks
				for _, callback := range matcher.Hooks {
					callbackID := hp.generateCallbackID()
					hp.hookCallbacks[callbackID] = callback
					callbackIDs = append(callbackIDs, callbackID)
				}

				cfg := shared.HookMatcherConfig{
					Matcher:         matcher.Matcher,
					HookCallbackIDs: callbackIDs,
				}
				if matcher.Timeout != nil {
					cfg.Timeout = matcher.Timeout
				}
				hp.matcherConfigs[event] = append(hp.matcherConfigs[event], cfg)
			}
		}
	}
}

// generateCallbackID generates a unique callback ID.
func (hp *HookProcessor) generateCallbackID() string {
	id := atomic.AddInt64(&hp.nextCallbackID, 1)
	return fmt.Sprintf("hook_%d", id-1)
}

// BuildInitializeConfig builds the hooks configuration for CLI initialization.
func (hp *HookProcessor) BuildInitializeConfig() map[string][]shared.HookMatcherConfig {
	hp.mu.RLock()
	defer hp.mu.RUnlock()

	if len(hp.matcherConfigs) == 0 {
		return nil
	}

	config := make(map[string][]shared.HookMatcherConfig)

	for event, matchers := range hp.matcherConfigs {
		eventKey := string(event)
		if len(matchers) > 0 {
			out := make([]shared.HookMatcherConfig, len(matchers))
			copy(out, matchers)
			config[eventKey] = out
		}
	}

	return config
}

// ProcessHookCallback processes a hook callback request from CLI.
func (hp *HookProcessor) ProcessHookCallback(
	request *shared.HookCallbackRequest,
) (shared.HookJSONOutput, error) {
	hp.mu.RLock()
	callback, exists := hp.hookCallbacks[request.CallbackID]
	hp.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no hook callback found for ID: %s", request.CallbackID)
	}

	// Prepare hook context
	hookCtx := shared.HookContext{
		Context: hp.ctx,
		Signal:  nil, // TODO: Add abort signal support
	}

	// Call the user's hook callback
	output, err := callback(request.Input, request.ToolUseID, hookCtx)
	if err != nil {
		return nil, fmt.Errorf("hook callback error: %w", err)
	}

	return output, nil
}

// ProcessCanUseTool processes a tool permission request from CLI.
func (hp *HookProcessor) ProcessCanUseTool(
	request *shared.CanUseToolRequest,
) (*shared.PermissionResponse, error) {
	if hp.canUseTool == nil {
		return nil, fmt.Errorf("canUseTool callback is not provided")
	}

	// Prepare permission context
	permCtx := shared.ToolPermissionContext{
		Context:        hp.ctx,
		Signal:         nil, // TODO: Add abort signal support
		Suggestions:    convertPermissionSuggestions(request.PermissionSuggestions),
		ToolUseID:      request.ToolUseID,
		AgentID:        request.AgentID,
		BlockedPath:    request.BlockedPath,
		DecisionReason: request.DecisionReason,
		Title:          request.Title,
		DisplayName:    request.DisplayName,
		Description:    request.Description,
	}

	// Call the permission callback
	result, err := hp.canUseTool(request.ToolName, request.Input, permCtx)
	if err != nil {
		return nil, fmt.Errorf("permission callback error: %w", err)
	}

	// Convert result to response format
	response := &shared.PermissionResponse{}

	switch r := result.(type) {
	case *shared.PermissionResultAllow:
		response.Behavior = "allow"
		// Python parity: always emit updatedInput (or original if nil).
		if r.UpdatedInput != nil {
			response.UpdatedInput = r.UpdatedInput
		} else {
			response.UpdatedInput = request.Input
		}
		if r.UpdatedPermissions != nil {
			response.UpdatedPermissions = convertPermissionUpdates(r.UpdatedPermissions)
		}

	case *shared.PermissionResultDeny:
		response.Behavior = "deny"
		// Python parity: always emit message (even when empty); only emit
		// interrupt when true. PermissionResponse uses omitempty on both,
		// but omitempty on a string drops empty values, so guard the
		// downstream marshaler by routing through a typed dict so message
		// is always present.
		response.Message = r.Message
		response.Interrupt = r.Interrupt
		// Stash via SuppressMessageOmit so MarshalJSON keeps "message".
		response.AlwaysEmitMessage = true

	default:
		return nil, fmt.Errorf("invalid permission result type: %T", result)
	}

	return response, nil
}

// SetCanUseToolCallback sets the tool permission callback.
func (hp *HookProcessor) SetCanUseToolCallback(callback shared.CanUseToolCallback) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.canUseTool = callback
}

// Helper functions

// convertPermissionSuggestions deserializes a list of raw suggestion dicts
// (as received from the CLI's permission_request control message) into
// PermissionUpdate values. Mirrors Python SDK's
// `[PermissionUpdate.from_dict(s) for s in suggestions]` from #920.
func convertPermissionSuggestions(suggestions []any) []shared.PermissionUpdate {
	if len(suggestions) == 0 {
		return nil
	}
	updates := make([]shared.PermissionUpdate, 0, len(suggestions))
	for _, raw := range suggestions {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t, _ := item["type"].(string)
		if t == "" {
			continue
		}
		updates = append(updates, shared.PermissionUpdateFromDict(item))
	}
	return updates
}

// convertPermissionUpdates serializes a list of PermissionUpdate values into
// the wire-format dict list the control protocol expects.
//
// Mirrors Python's PermissionUpdate.to_dict variant-aware filtering: each
// type only carries the fields meaningful for that variant. Fields set on
// the wrong variant are dropped silently to match Python (and to avoid
// confusing the CLI's strict TypeScript-side validator).
func convertPermissionUpdates(updates []shared.PermissionUpdate) []any {
	result := make([]any, len(updates))
	for i, update := range updates {
		data := map[string]any{
			"type": string(update.Type),
		}
		// Destination applies to all variants.
		if update.Destination != nil {
			data["destination"] = string(*update.Destination)
		}
		switch update.Type {
		case shared.PermissionUpdateTypeAddRules,
			shared.PermissionUpdateTypeReplaceRules,
			shared.PermissionUpdateTypeRemoveRules:
			if update.Rules != nil {
				rules := make([]map[string]any, len(update.Rules))
				for j, rule := range update.Rules {
					rules[j] = map[string]any{
						"toolName":    rule.ToolName,
						"ruleContent": rule.RuleContent,
					}
				}
				data["rules"] = rules
			}
			if update.Behavior != nil {
				data["behavior"] = *update.Behavior
			}
		case shared.PermissionUpdateTypeSetMode:
			if update.Mode != nil {
				data["mode"] = *update.Mode
			}
		case shared.PermissionUpdateTypeAddDirectories,
			shared.PermissionUpdateTypeRemoveDirectories:
			if update.Directories != nil {
				data["directories"] = update.Directories
			}
		}
		result[i] = data
	}
	return result
}

// MarshalHookOutput marshals hook output to JSON, handling special cases.
// Go doesn't have keyword conflicts like Python's async_/continue_,
// so we can use the fields directly.
func MarshalHookOutput(output shared.HookJSONOutput) ([]byte, error) {
	return json.Marshal(output)
}
