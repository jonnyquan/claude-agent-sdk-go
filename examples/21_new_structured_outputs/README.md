# Example 21: New Structured Outputs API

This example demonstrates structured outputs using JSON schema validation with the new `pkg/claudesdk` API.

## 🎯 What This Example Shows

1. **User Profile Extraction** - Extract structured data from text
2. **Weather Information** - Generate structured weather data
3. **Math Problem Solving** - Step-by-step solutions in JSON format
4. **Multiple Structured Queries** - Batch processing with structured outputs

## 🚀 Key Features Demonstrated

### Structured Output Configuration
```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{
            "type": "string",
            "description": "Person's full name",
        },
        "age": map[string]interface{}{
            "type": "integer",
            "minimum": 0,
            "maximum": 150,
        },
    },
    "required": []string{"name", "age"},
}

messages, err := claudesdk.Query(ctx, prompt,
    claudesdk.WithOutputFormat(schema),
)
```

### Response Processing
```go
if assistantMsg.StructuredOutput != nil {
    // Parse structured JSON response
    var result MyStruct
    jsonBytes, _ := json.Marshal(assistantMsg.StructuredOutput)
    json.Unmarshal(jsonBytes, &result)
}
```

## 🔧 Running the Example

```bash
cd examples/21_new_structured_outputs
go run main.go
```

## 📋 Structured Output Benefits

### 1. **Guaranteed Format**
JSON schema ensures responses match expected structure:
```go
type UserProfile struct {
    Name     string   `json:"name"`
    Age      int      `json:"age"`
    Skills   []string `json:"skills"`
    Location string   `json:"location"`
}
```

### 2. **Validation**
Schema validation includes:
- Type checking (string, number, array, object)
- Range validation (minimum, maximum)
- Enum validation (specific allowed values)
- Required field validation

### 3. **Better Integration**
Structured outputs make it easy to:
- Parse responses into Go structs
- Validate data integrity
- Build reliable AI-powered applications

## 💡 Use Cases

### Data Extraction
```go
claudesdk.Query(ctx, "Extract contact info from: John Doe, age 30, lives in NYC",
    claudesdk.WithOutputFormat(contactSchema),
)
```

### Classification
```go
claudesdk.Query(ctx, "Classify this text: 'I love this product!'",
    claudesdk.WithOutputFormat(classificationSchema),
)
```

### Structured Generation
```go
claudesdk.Query(ctx, "Generate a user persona for a mobile app",
    claudesdk.WithOutputFormat(personaSchema),
)
```

## 🔄 Migration Notes

The new API makes structured outputs more explicit:

Old way (still works):
```go
import "github.com/jonnyquan/claude-agent-sdk-go"
claudecode.Query(ctx, prompt, claudecode.WithOutputFormat(schema))
```

New way (recommended):
```go
import "github.com/jonnyquan/claude-agent-sdk-go/pkg/claudesdk"
claudesdk.Query(ctx, prompt, claudesdk.WithOutputFormat(schema))
```

## 📚 Schema Examples

### Simple Object
```json
{
  "type": "object",
  "properties": {
    "answer": {"type": "string"},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1}
  },
  "required": ["answer"]
}
```

### Complex Nested
```json
{
  "type": "object",
  "properties": {
    "user": {
      "type": "object", 
      "properties": {
        "name": {"type": "string"},
        "contacts": {
          "type": "array",
          "items": {"type": "string"}
        }
      }
    }
  }
}
```

## ✨ Benefits of New API

1. **Clear Structure** - All structured output functionality in `pkg/claudesdk`
2. **Better Documentation** - Focused package documentation  
3. **Type Safety** - Strong typing with Go structs
4. **Validation** - JSON schema validation built-in

Perfect for building reliable, production-ready AI applications!
