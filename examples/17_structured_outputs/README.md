# Structured Outputs Example

This example demonstrates how to use structured outputs with the Claude Agent SDK for Go to get validated JSON responses that match a specific schema.

## Features Demonstrated

- **JSON Schema Validation**: Define a schema and get guaranteed valid JSON output
- **File System Analysis**: Agent explores the filesystem and returns structured data
- **Type Safety**: Structured output ensures consistent response format

## Key Components

### Schema Definition
```go
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
```

### Output Format Configuration
```go
options := claudecode.NewOptions(
    claudecode.WithOutputFormat(map[string]interface{}{
        "type":   "json_schema",
        "schema": schema,
    }),
)
```

### Structured Output Access
```go
if resultMsg.StructuredOutput != nil {
    if output, ok := resultMsg.StructuredOutput.(map[string]interface{}); ok {
        fileCount := output["file_count"]
        hasTests := output["has_tests"] 
        framework := output["test_framework"]
    }
}
```

## Running the Example

```bash
cd examples/17_structured_outputs
go run main.go
```

## Expected Output

```
🔍 Running structured output query...
📨 Message: assistant
📨 Message: result
📋 Result: success
🎯 Structured Output: map[file_count:42 has_tests:true test_framework:unknown]
📁 File Count: 42
🧪 Has Tests: true
🔧 Test Framework: unknown
✅ Query completed with 2 messages
```

## Use Cases

- **Data Extraction**: Extract specific fields from text or documents
- **Form Processing**: Parse form data into structured objects  
- **API Responses**: Ensure consistent API response formats
- **Configuration Generation**: Generate config files with guaranteed structure
- **Report Generation**: Create structured reports from unstructured data

## Benefits

- **Type Safety**: JSON schema validation ensures correct data types
- **Reliability**: No need to parse free-form text responses
- **Consistency**: Guaranteed response structure across API calls
- **Error Prevention**: Invalid responses are rejected by Claude
- **Integration**: Easy to integrate with existing systems expecting JSON
