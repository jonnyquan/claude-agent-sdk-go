package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
)

// Define structured output schemas
type UserProfile struct {
	Name     string   `json:"name"`
	Age      int      `json:"age"`
	Skills   []string `json:"skills"`
	Location string   `json:"location"`
}

type WeatherInfo struct {
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Humidity    int     `json:"humidity"`
}

type MathResult struct {
	Expression string  `json:"expression"`
	Result     float64 `json:"result"`
	Steps      []string `json:"steps"`
}

func main() {
	ctx := context.Background()

	fmt.Println("🎯 Claude SDK - New Structured Outputs API")
	fmt.Println("=========================================")

	// Example 1: User Profile Extraction
	fmt.Println("\n👤 Example 1: User Profile Extraction")
	fmt.Println("-----------------------------------")
	extractUserProfile(ctx)

	// Example 2: Weather Information
	fmt.Println("\n🌤️ Example 2: Weather Information")
	fmt.Println("--------------------------------")
	getWeatherInfo(ctx)

	// Example 3: Math Problem Solving
	fmt.Println("\n🧮 Example 3: Math Problem Solving")
	fmt.Println("---------------------------------")
	solveMathProblem(ctx)

	// Example 4: Multiple Structured Queries
	fmt.Println("\n🔄 Example 4: Multiple Structured Queries")
	fmt.Println("----------------------------------------")
	multipleStructuredQueries(ctx)

	fmt.Println("\n🎊 Structured Outputs Demo Complete!")
}

func extractUserProfile(ctx context.Context) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Person's full name",
			},
			"age": map[string]interface{}{
				"type":        "integer",
				"description": "Person's age in years",
				"minimum":     0,
				"maximum":     150,
			},
			"skills": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "List of professional skills",
			},
			"location": map[string]interface{}{
				"type":        "string",
				"description": "Current city and country",
			},
		},
		"required": []string{"name", "age", "skills", "location"},
	}

	messages, err := claudesdk.Query(ctx,
		"Extract profile information: John Smith is a 28-year-old software engineer from Toronto, Canada. He specializes in Go, Python, and Docker.",
		claudesdk.WithSystemPrompt("Extract user profile information from the given text and return it in the specified JSON format."),
		claudesdk.WithOutputFormat(schema),
	)
	
	if err != nil {
		log.Printf("Failed to query for user profile: %v", err)
		return
	}

	processStructuredOutput[UserProfile](ctx, messages, "User Profile")
}

func getWeatherInfo(ctx context.Context) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "City name",
			},
			"temperature": map[string]interface{}{
				"type":        "number",
				"description": "Temperature in Celsius",
			},
			"condition": map[string]interface{}{
				"type":        "string",
				"description": "Weather condition",
				"enum":        []string{"sunny", "cloudy", "rainy", "snowy", "windy"},
			},
			"humidity": map[string]interface{}{
				"type":        "integer",
				"description": "Humidity percentage",
				"minimum":     0,
				"maximum":     100,
			},
		},
		"required": []string{"city", "temperature", "condition", "humidity"},
	}

	// Use WithClient for this example
	err := claudesdk.WithClient(ctx, func(client claudesdk.Client) error {
		if err := client.Query(ctx, "What's the weather like in Paris today? Make up realistic data."); err != nil {
			return err
		}

		result := client.ReceiveResponse(ctx)
		processStructuredOutput[WeatherInfo](ctx, result, "Weather Info")
		return nil
	},
		claudesdk.WithSystemPrompt("Provide weather information in the specified JSON format. You can create realistic example data."),
		claudesdk.WithOutputFormat(schema),
	)

	if err != nil {
		log.Printf("Weather query failed: %v", err)
	}
}

func solveMathProblem(ctx context.Context) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "The mathematical expression",
			},
			"result": map[string]interface{}{
				"type":        "number",
				"description": "The calculated result",
			},
			"steps": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Step-by-step solution",
			},
		},
		"required": []string{"expression", "result", "steps"},
	}

	messages, err := claudesdk.Query(ctx,
		"Solve this math problem step by step: (15 + 25) × 3 - 18",
		claudesdk.WithSystemPrompt("Solve the math problem and show your work step by step in JSON format."),
		claudesdk.WithOutputFormat(schema),
		claudesdk.WithModel("claude-3-sonnet-20241022"),
	)

	if err != nil {
		log.Printf("Math problem query failed: %v", err)
		return
	}

	processStructuredOutput[MathResult](ctx, messages, "Math Solution")
}

func multipleStructuredQueries(ctx context.Context) {
	// Define a simple schema for short responses
	simpleSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type":        "string",
				"description": "Brief answer to the question",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Category of the question",
			},
		},
		"required": []string{"answer", "category"},
	}

	questions := []string{
		"What's the capital of Japan?",
		"Name one benefit of Go programming language",
		"What year was Claude AI founded?",
	}

	type SimpleAnswer struct {
		Answer   string `json:"answer"`
		Category string `json:"category"`
	}

	for i, question := range questions {
		fmt.Printf("  Question %d: %s\n", i+1, question)
		
		messages, err := claudesdk.Query(ctx, question,
			claudesdk.WithSystemPrompt("Answer the question briefly and categorize it."),
			claudesdk.WithOutputFormat(simpleSchema),
		)
		
		if err != nil {
			log.Printf("Query %d failed: %v", i+1, err)
			continue
		}

		// Process the structured response
		processStructuredOutput[SimpleAnswer](ctx, messages, fmt.Sprintf("Answer %d", i+1))
	}
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

func processStructuredOutput[T any](ctx context.Context, messages claudesdk.MessageIterator, label string) {
	for {
		msg, err := messages.Next(ctx)
		if err != nil {
			if err == claudesdk.ErrNoMoreMessages {
				break
			}
			log.Printf("Error reading message: %v", err)
			break
		}

		if assistantMsg, ok := msg.(*claudesdk.AssistantMessage); ok {
			// For now, just show regular text response
			content := extractTextContent(assistantMsg.Content)
			fmt.Printf("  💬 %s: %s\n", label, content)
		} else if resultMsg, ok := msg.(*claudesdk.ResultMessage); ok {
			// Check if the result message has structured output
			if resultMsg.StructuredOutput != nil {
				fmt.Printf("  📊 %s (Structured):\n", label)
				
				// Parse the structured output
				var result T
				if jsonBytes, err := json.Marshal(resultMsg.StructuredOutput); err == nil {
					if err := json.Unmarshal(jsonBytes, &result); err == nil {
						prettyJSON, _ := json.MarshalIndent(result, "    ", "  ")
						fmt.Printf("    %s\n", prettyJSON)
					} else {
						fmt.Printf("    Error parsing structured output: %v\n", err)
						fmt.Printf("    Raw: %v\n", resultMsg.StructuredOutput)
					}
				}
			} else {
				fmt.Printf("  📋 %s (Result): %v\n", label, resultMsg.Result)
			}
		}
	}
}
