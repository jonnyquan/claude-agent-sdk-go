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
		nextCallbackID: 0,
		ctx:            ctx,
	}
	
	// Load hooks from options
	if options != nil && len(options.Hooks) > 0 {
		hp.loadHooksFromOptions(options)
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
				
				// Register callbacks
				for _, callback := range matcher.Hooks {
					callbackID := hp.generateCallbackID()
					hp.hookCallbacks[callbackID] = callback
				}
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
	
	if len(hp.hooks) == 0 {
		return nil
	}
	
	config := make(map[string][]shared.HookMatcherConfig)
	
	for event, matchers := range hp.hooks {
		eventKey := string(event)
		var matcherConfigs []shared.HookMatcherConfig
		
		for _, matcher := range matchers {
			var callbackIDs []string
			
			// Find callback IDs for this matcher
			for callbackID, callback := range hp.hookCallbacks {
				// Check if callback belongs to this matcher
				for _, matcherCallback := range matcher.Hooks {
					// Compare function pointers (this is a bit hacky but works for now)
					// In production, we'd need a better way to associate callbacks
					if fmt.Sprintf("%p", callback) == fmt.Sprintf("%p", matcherCallback) {
						callbackIDs = append(callbackIDs, callbackID)
					}
				}
			}
			
			if len(callbackIDs) > 0 {
				matcherConfigs = append(matcherConfigs, shared.HookMatcherConfig{
					Matcher:         matcher.Matcher,
					HookCallbackIDs: callbackIDs,
				})
			}
		}
		
		if len(matcherConfigs) > 0 {
			config[eventKey] = matcherConfigs
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
		Context:     hp.ctx,
		Signal:      nil, // TODO: Add abort signal support
		Suggestions: convertPermissionSuggestions(request.PermissionSuggestions),
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
		// Always return updatedInput (or original if nil)
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
		response.Message = r.Message
		response.Interrupt = r.Interrupt
		
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

func convertPermissionSuggestions(suggestions []any) []shared.PermissionUpdate {
	// TODO: Implement proper conversion from CLI suggestions
	return []shared.PermissionUpdate{}
}

func convertPermissionUpdates(updates []shared.PermissionUpdate) []any {
	result := make([]any, len(updates))
	for i, update := range updates {
		// Convert to map for JSON serialization
		data := map[string]any{
			"type": string(update.Type),
		}
		if update.Destination != nil {
			data["destination"] = string(*update.Destination)
		}
		if len(update.Rules) > 0 {
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
		if update.Mode != nil {
			data["mode"] = *update.Mode
		}
		if len(update.Directories) > 0 {
			data["directories"] = update.Directories
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
