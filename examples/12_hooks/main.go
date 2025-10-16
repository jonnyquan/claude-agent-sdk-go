package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

// Example 1: PreToolUse hook to block certain bash commands
func checkBashCommand(input claudecode.HookInput, toolUseID *string, ctx claudecode.HookContext) (claudecode.HookJSONOutput, error) {
	toolName, _ := input["tool_name"].(string)
	toolInput, _ := input["tool_input"].(map[string]any)
	
	if toolName != "Bash" {
		return claudecode.NewPreToolUseOutput(claudecode.PermissionDecisionAllow, "", nil), nil
	}
	
	command, _ := toolInput["command"].(string)
	
	// Block dangerous commands
	dangerousCommands := []string{"rm -rf", "sudo", "mkfs", "dd if="}
	for _, dangerous := range dangerousCommands {
		if strings.Contains(command, dangerous) {
			return claudecode.HookJSONOutput{
				"reason":         fmt.Sprintf("Command contains dangerous pattern: %s", dangerous),
				"systemMessage":  "⚠️ Dangerous command blocked by security hook",
				"hookSpecificOutput": map[string]any{
					"hookEventName":            "PreToolUse",
					"permissionDecision":       claudecode.PermissionDecisionDeny,
					"permissionDecisionReason": fmt.Sprintf("Security policy blocks commands with: %s", dangerous),
				},
			}, nil
		}
	}
	
	return claudecode.NewPreToolUseOutput(claudecode.PermissionDecisionAllow, "Command approved", nil), nil
}

// Example 2: PostToolUse hook to review tool output
func reviewToolOutput(input claudecode.HookInput, toolUseID *string, ctx claudecode.HookContext) (claudecode.HookJSONOutput, error) {
	toolResponse, _ := input["tool_response"]
	
	responseStr := fmt.Sprintf("%v", toolResponse)
	
	// Check for errors in output
	if strings.Contains(strings.ToLower(responseStr), "error") {
		return claudecode.HookJSONOutput{
			"systemMessage": "⚠️ The command produced an error",
			"reason":        "Tool execution failed - consider checking the command syntax",
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "PostToolUse",
				"additionalContext": "The command encountered an error. You may want to try a different approach.",
			},
		}, nil
	}
	
	return claudecode.NewPostToolUseOutput(""), nil
}

// Example 3: Hook that stops execution on critical errors
func stopOnCriticalError(input claudecode.HookInput, toolUseID *string, ctx claudecode.HookContext) (claudecode.HookJSONOutput, error) {
	toolResponse, _ := input["tool_response"]
	responseStr := fmt.Sprintf("%v", toolResponse)
	
	if strings.Contains(strings.ToLower(responseStr), "critical") {
		log.Println("Critical error detected - stopping execution")
		return claudecode.NewStopOutput("Critical error detected in tool output - execution halted for safety"), nil
	}
	
	continueVal := true
	return claudecode.HookJSONOutput{
		"continue": &continueVal,
	}, nil
}

// Example 4: UserPromptSubmit hook to add custom instructions
func addCustomInstructions(input claudecode.HookInput, toolUseID *string, ctx claudecode.HookContext) (claudecode.HookJSONOutput, error) {
	return claudecode.HookJSONOutput{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": "Always be concise and explain your reasoning step by step.",
		},
	}, nil
}

func examplePreToolUse() {
	fmt.Println("=== PreToolUse Example ===")
	fmt.Println("This example shows how PreToolUse hooks can block dangerous commands.")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Configure hook to check bash commands
	options := []claudecode.Option{
		claudecode.WithAllowedTools("Bash"),
		claudecode.WithHook(claudecode.HookEventPreToolUse, claudecode.HookMatcher{
			Matcher: "Bash",
			Hooks:   []claudecode.HookCallback{checkBashCommand},
		}),
	}
	
	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		fmt.Println("User: Try to run 'rm -rf /' (should be blocked)")
		
		if err := client.Query(ctx, "Run this bash command: rm -rf /"); err != nil {
			return err
		}
		
		// Process responses
		for msg := range client.ReceiveMessages(ctx) {
			if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					if textBlock, ok := block.(*claudecode.TextBlock); ok {
						fmt.Printf("Claude: %s\n", textBlock.Text)
					}
				}
			}
		}
		
		return nil
	}, options...)
	
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	
	fmt.Println("")
}

func examplePostToolUse() {
	fmt.Println("=== PostToolUse Example ===")
	fmt.Println("This example shows how PostToolUse hooks can review tool output.")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	options := []claudecode.Option{
		claudecode.WithAllowedTools("Bash"),
		claudecode.WithHook(claudecode.HookEventPostToolUse, claudecode.HookMatcher{
			Matcher: "Bash",
			Hooks:   []claudecode.HookCallback{reviewToolOutput},
		}),
	}
	
	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		fmt.Println("User: Run a command that will produce an error: ls /nonexistent_directory")
		
		if err := client.Query(ctx, "Run this command: ls /nonexistent_directory"); err != nil {
			return err
		}
		
		for msg := range client.ReceiveMessages(ctx) {
			if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					if textBlock, ok := block.(*claudecode.TextBlock); ok {
						fmt.Printf("Claude: %s\n", textBlock.Text)
					}
				}
			}
		}
		
		return nil
	}, options...)
	
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	
	fmt.Println("")
}

func exampleContinueControl() {
	fmt.Println("=== Continue/Stop Control Example ===")
	fmt.Println("This example shows how to use continue=false with stopReason to halt execution.")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	options := []claudecode.Option{
		claudecode.WithAllowedTools("Bash"),
		claudecode.WithHook(claudecode.HookEventPostToolUse, claudecode.HookMatcher{
			Matcher: "Bash",
			Hooks:   []claudecode.HookCallback{stopOnCriticalError},
		}),
	}
	
	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		fmt.Println("User: Run a command that outputs 'CRITICAL ERROR'")
		
		if err := client.Query(ctx, "Run this bash command: echo 'CRITICAL ERROR: system failure'"); err != nil {
			return err
		}
		
		for msg := range client.ReceiveMessages(ctx) {
			if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					if textBlock, ok := block.(*claudecode.TextBlock); ok {
						fmt.Printf("Claude: %s\n", textBlock.Text)
					}
				}
			}
		}
		
		return nil
	}, options...)
	
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	
	fmt.Println("")
}

func exampleMultipleHooks() {
	fmt.Println("=== Multiple Hooks Example ===")
	fmt.Println("This example shows how to use multiple hooks together.")
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	options := []claudecode.Option{
		claudecode.WithAllowedTools("Bash", "Write"),
		claudecode.WithHook(claudecode.HookEventPreToolUse, claudecode.HookMatcher{
			Matcher: "Bash",
			Hooks:   []claudecode.HookCallback{checkBashCommand},
		}),
		claudecode.WithHook(claudecode.HookEventPostToolUse, claudecode.HookMatcher{
			Matcher: "Bash",
			Hooks:   []claudecode.HookCallback{reviewToolOutput, stopOnCriticalError},
		}),
		claudecode.WithHook(claudecode.HookEventUserPromptSubmit, claudecode.HookMatcher{
			Matcher: "*",
			Hooks:   []claudecode.HookCallback{addCustomInstructions},
		}),
	}
	
	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		fmt.Println("User: What is 2+2?")
		
		if err := client.Query(ctx, "What is 2+2?"); err != nil {
			return err
		}
		
		for msg := range client.ReceiveMessages(ctx) {
			if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					if textBlock, ok := block.(*claudecode.TextBlock); ok {
						fmt.Printf("Claude: %s\n", textBlock.Text)
					}
				}
			}
		}
		
		return nil
	}, options...)
	
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
	
	fmt.Println("")
}

func main() {
	fmt.Println("Claude Agent SDK - Hooks Examples")
	fmt.Println("===================================")
	fmt.Println("")
	
	// Run examples
	// Note: These examples require hook processing logic to be implemented in the SDK
	fmt.Println("⚠️  Hook processing logic is not yet fully implemented in the Go SDK.")
	fmt.Println("These examples demonstrate the API design and usage patterns.")
	fmt.Println("")
	
	// Uncomment to run examples when hook processing is implemented:
	// examplePreToolUse()
	// examplePostToolUse()
	// exampleContinueControl()
	// exampleMultipleHooks()
	
	fmt.Println("Examples completed. See the code for hook usage patterns.")
}
