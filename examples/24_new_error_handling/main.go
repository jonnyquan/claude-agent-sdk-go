package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

func main() {
	ctx := context.Background()

	fmt.Println("🚨 Claude SDK - New Error Handling API")
	fmt.Println("====================================")

	// Example 1: Basic Error Handling
	fmt.Println("\n⚠️ Example 1: Basic Error Handling")
	fmt.Println("--------------------------------")
	basicErrorHandling(ctx)

	// Example 2: Timeout Handling
	fmt.Println("\n⏰ Example 2: Timeout Handling")
	fmt.Println("-----------------------------")
	timeoutHandling(ctx)

	// Example 3: Context Cancellation
	fmt.Println("\n🛑 Example 3: Context Cancellation")
	fmt.Println("---------------------------------")
	cancellationHandling(ctx)

	// Example 4: Graceful Error Recovery
	fmt.Println("\n🔄 Example 4: Graceful Error Recovery")
	fmt.Println("-----------------------------------")
	gracefulErrorRecovery(ctx)

	// Example 5: Advanced Error Inspection
	fmt.Println("\n🔍 Example 5: Advanced Error Inspection")
	fmt.Println("-------------------------------------")
	advancedErrorInspection(ctx)

	fmt.Println("\n🎊 Error Handling Demo Complete!")
}

func basicErrorHandling(ctx context.Context) {
	// Example with invalid configuration
	messages, err := claudesdk.Query(ctx, "Hello Claude!",
		claudesdk.WithModel("invalid-model-name"),
		claudesdk.WithCwd("/nonexistent/path"),
	)

	if err != nil {
		fmt.Printf("  ❌ Expected error occurred: %v\n", err)
		
		// Handle specific error types
		if isConnectionError(err) {
			fmt.Printf("     🔌 This appears to be a connection error\n")
		} else if isConfigurationError(err) {
			fmt.Printf("     ⚙️  This appears to be a configuration error\n")
		} else {
			fmt.Printf("     ❓ Unknown error type\n")
		}
		return
	}

	// If no error, process messages
	processMessages(ctx, messages, "Basic Error Handling")
}

func timeoutHandling(ctx context.Context) {
	// Create a context with very short timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fmt.Printf("  ⏱️ Setting 5-second timeout for query...\n")

	messages, err := claudesdk.Query(timeoutCtx, "Write a very long story about space exploration with detailed descriptions",
		claudesdk.WithSystemPrompt("Write detailed, lengthy responses"),
	)

	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
		
		if isTimeoutError(err) {
			fmt.Printf("     ⏰ This was a timeout error - the operation took too long\n")
			fmt.Printf("     💡 Suggestion: Increase timeout or simplify the request\n")
		}
		return
	}

	// Process with timeout context
	processMessagesWithTimeout(timeoutCtx, messages, "Timeout Handling")
}

func cancellationHandling(ctx context.Context) {
	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)

	// Start the query
	go func() {
		// Cancel after 3 seconds to simulate user cancellation
		time.Sleep(3 * time.Second)
		fmt.Printf("  🛑 User cancelled the operation\n")
		cancel()
	}()

	messages, err := claudesdk.Query(cancelCtx, "Explain quantum computing in great detail")

	if err != nil {
		fmt.Printf("  ❌ Query was cancelled: %v\n", err)
		
		if isCancelledError(err) {
			fmt.Printf("     🛑 Operation was cancelled by user\n")
			fmt.Printf("     💡 This is normal for user-initiated cancellations\n")
		}
		return
	}

	processMessages(cancelCtx, messages, "Cancellation Handling")
}

func gracefulErrorRecovery(ctx context.Context) {
	// Attempt multiple strategies with error recovery
	strategies := []struct {
		name   string
		config func() (claudesdk.MessageIterator, error)
	}{
		{
			"Primary model", 
			func() (claudesdk.MessageIterator, error) {
				return claudesdk.Query(ctx, "What is 2+2?",
					claudesdk.WithModel("claude-3-sonnet-20241022"),
				)
			},
		},
		{
			"Fallback model",
			func() (claudesdk.MessageIterator, error) {
				return claudesdk.Query(ctx, "What is 2+2?",
					claudesdk.WithModel("claude-3-haiku-20240307"),
				)
			},
		},
		{
			"Basic configuration",
			func() (claudesdk.MessageIterator, error) {
				return claudesdk.Query(ctx, "What is 2+2?")
			},
		},
	}

	for i, strategy := range strategies {
		fmt.Printf("  🔄 Trying strategy %d: %s\n", i+1, strategy.name)
		
		messages, err := strategy.config()
		if err != nil {
			fmt.Printf("     ❌ Strategy %d failed: %v\n", i+1, err)
			continue
		}

		fmt.Printf("     ✅ Strategy %d succeeded!\n", i+1)
		processMessages(ctx, messages, "Recovery Strategy")
		return
	}

	fmt.Printf("  ❌ All strategies failed\n")
}

func advancedErrorInspection(ctx context.Context) {
	// Use WithClient to demonstrate different error scenarios
	scenarios := []struct {
		name string
		fn   func(claudesdk.Client) error
	}{
		{
			"Invalid query",
			func(client claudesdk.Client) error {
				return client.Query(ctx, "") // Empty query
			},
		},
		{
			"Session error",
			func(client claudesdk.Client) error {
				return client.QueryWithSession(ctx, "Hello", "invalid-session-id")
			},
		},
		{
			"Valid operation",
			func(client claudesdk.Client) error {
				return client.Query(ctx, "Hello!")
			},
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("  🧪 Testing scenario %d: %s\n", i+1, scenario.name)

		err := claudesdk.WithClient(ctx, scenario.fn,
			claudesdk.WithSystemPrompt("You are a helpful assistant"),
		)

		if err != nil {
			fmt.Printf("     ❌ Error: %v\n", err)
			
			// Detailed error analysis
			analyzeError(err)
		} else {
			fmt.Printf("     ✅ Success\n")
		}
	}
}

func analyzeError(err error) {
	fmt.Printf("       🔍 Error Analysis:\n")
	
	// Check error type
	if isConnectionError(err) {
		fmt.Printf("         📡 Connection Error: Check network and CLI availability\n")
	} else if isConfigurationError(err) {
		fmt.Printf("         ⚙️  Configuration Error: Check settings and options\n")
	} else if isTimeoutError(err) {
		fmt.Printf("         ⏰ Timeout Error: Operation took too long\n")
	} else if isCancelledError(err) {
		fmt.Printf("         🛑 Cancelled Error: Operation was cancelled\n")
	} else {
		fmt.Printf("         ❓ Unknown Error Type\n")
	}

	// Check if it's a wrapped error
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		fmt.Printf("         🎁 Wrapped Error: %v\n", unwrapped)
	}

	// Convert to string for pattern matching
	errStr := err.Error()
	if contains(errStr, "not found") {
		fmt.Printf("         📁 Resource Not Found: Check paths and file existence\n")
	} else if contains(errStr, "permission") {
		fmt.Printf("         🔐 Permission Error: Check access permissions\n")
	} else if contains(errStr, "authentication") {
		fmt.Printf("         🔑 Authentication Error: Check API keys and credentials\n")
	}
}

func processMessages(ctx context.Context, messages claudesdk.MessageIterator, label string) {
	fmt.Printf("  📝 %s Response:\n", label)
	
	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			fmt.Printf("     ❌ Error reading message: %v\n", err)
			break
		}

		if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
			textContent := extractTextContent(assistantMsg.Content)
			content := textContent
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("     🤖 %s\n", content)
		}
	}
}

func processMessagesWithTimeout(ctx context.Context, messages claudesdk.MessageIterator, label string) {
	fmt.Printf("  📝 %s Response (with timeout):\n", label)
	
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("     ⏰ Message processing timed out\n")
			return
		default:
			msg, err := messages.Next(ctx)
			if err != nil {
				if err == claudesdk.ErrNoMoreMessages {
					break
				}
				fmt.Printf("     ❌ Error reading message: %v\n", err)
				break
			}

			if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
				content := extractTextContent(assistantMsg.Content)
				if len(content) > 100 {
					content = content[:100] + "..."
				}
				fmt.Printf("     🤖 %s\n", content)
			}
		}
	}
}

// Helper functions for error classification
func isConnectionError(err error) bool {
	return contains(err.Error(), "connection") || 
		   contains(err.Error(), "network") ||
		   contains(err.Error(), "CLI not found")
}

func isConfigurationError(err error) bool {
	return contains(err.Error(), "invalid") ||
		   contains(err.Error(), "config") ||
		   contains(err.Error(), "option")
}

func isTimeoutError(err error) bool {
	return contains(err.Error(), "timeout") ||
		   contains(err.Error(), "deadline exceeded") ||
		   errors.Is(err, context.DeadlineExceeded)
}

func isCancelledError(err error) bool {
	return contains(err.Error(), "cancel") ||
		   errors.Is(err, context.Canceled)
}

func extractTextContent(content []claudesdk.ContentBlock) string {
	var text strings.Builder
	for _, block := range content {
		if textBlock, ok := block.(*claudesdk.TextBlock); ok {
			text.WriteString(textBlock.Text)
			text.WriteString(" ")
		}
	}
	return strings.TrimSpace(text.String())
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr ||
		      containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
