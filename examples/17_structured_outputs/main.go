package main

import (
	"context"
	"fmt"
	"log"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
	// Define a JSON schema for structured output
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_count": map[string]interface{}{
				"type": "number",
			},
			"has_tests": map[string]interface{}{
				"type": "boolean",
			},
			"test_framework": map[string]interface{}{
				"type": "string",
				"enum": []string{"pytest", "unittest", "nose", "unknown"},
			},
		},
		"required": []string{"file_count", "has_tests"},
	}



	ctx := context.Background()

	// Run query with structured output
	fmt.Println("🔍 Running structured output query...")
	
	// Convert options to variadic options
	optionSlice := []claudecode.Option{
		claudecode.WithOutputFormat(map[string]interface{}{
			"type":   "json_schema",
			"schema": schema,
		}),
		claudecode.WithPermissionMode(claudecode.PermissionModeAcceptEdits),
	}
	
	messages := []claudecode.Message{}
	iterator, err := claudecode.Query(ctx, "Count how many Go files are in this directory and check if there are any test files. Use tools to explore the filesystem.", optionSlice...)

	if err != nil {
		log.Fatalf("❌ Error: %v", err)
	}

	// Process messages from iterator
	for {
		msg, err := iterator.Next(ctx)
		if err != nil {
			if err.Error() == "no more messages" {
				break
			}
			log.Fatalf("❌ Iterator error: %v", err)
		}
		
		fmt.Printf("📨 Message: %s\n", msg.Type())
		messages = append(messages, msg)
		
		if resultMsg, ok := msg.(*claudecode.ResultMessage); ok {
			fmt.Printf("📋 Result: %s\n", resultMsg.Subtype)
			
			// Check for structured output
			if resultMsg.StructuredOutput != nil {
				fmt.Printf("🎯 Structured Output: %+v\n", resultMsg.StructuredOutput)
				
				// Type assert to map to access fields
				if output, ok := resultMsg.StructuredOutput.(map[string]interface{}); ok {
					if fileCount, exists := output["file_count"]; exists {
						fmt.Printf("📁 File Count: %v\n", fileCount)
					}
					if hasTests, exists := output["has_tests"]; exists {
						fmt.Printf("🧪 Has Tests: %v\n", hasTests)
					}
					if framework, exists := output["test_framework"]; exists {
						fmt.Printf("🔧 Test Framework: %v\n", framework)
					}
				}
			}
		}
	}

	fmt.Printf("✅ Query completed with %d messages\n", len(messages))
}
