# SDK Debug Logging - CLI 输出与解析对比

## 🎯 目的

在 SDK 中添加详细日志，用于对比：
1. **CLI 实际返回的原始 JSON** 
2. **SDK 解析后的 Go 数据结构**

帮助诊断解析问题和数据不匹配问题。

---

## 📝 添加的日志位置

### 1. ProcessLine - 原始 CLI 输出
**文件：** `internal/parser/json.go`  
**行号：** ~44-45

```go
func (p *Parser) ProcessLine(line string) ([]shared.Message, error) {
    // ...
    // Debug: Log raw CLI output
    log.Printf("[SDK-Parser] 📥 Raw CLI line: %s", line)
    // ...
}
```

**输出示例：**
```
[SDK-Parser] 📥 Raw CLI line: {"type":"user","message":{"content":"Hello"}}
```

---

### 2. ProcessLine - 解析结果总结
**文件：** `internal/parser/json.go`  
**行号：** ~67-71

```go
// Debug: Log parsed messages
log.Printf("[SDK-Parser] 📤 Parsed %d message(s) from line", len(messages))
for i, msg := range messages {
    log.Printf("[SDK-Parser]   Message #%d: type=%T", i, msg)
}
```

**输出示例：**
```
[SDK-Parser] 📤 Parsed 1 message(s) from line
[SDK-Parser]   Message #0: type=*shared.UserMessage
```

---

### 3. ParseMessage - 解析输入
**文件：** `internal/parser/json.go`  
**行号：** ~79-88

```go
func (p *Parser) ParseMessage(data map[string]any) (shared.Message, error) {
    // Debug: Log raw data structure
    dataJSON, _ := json.Marshal(data)
    log.Printf("[SDK-Parser] 🔍 ParseMessage input: %s", string(dataJSON))
    
    msgType, ok := data["type"].(string)
    // ...
    log.Printf("[SDK-Parser] 🔍 Message type: %s", msgType)
    // ...
}
```

**输出示例：**
```
[SDK-Parser] 🔍 ParseMessage input: {"type":"user","message":{"content":"Hello"}}
[SDK-Parser] 🔍 Message type: user
```

---

### 4. ParseMessage - 解析输出
**文件：** `internal/parser/json.go`  
**行号：** ~109-115

```go
// Debug: Log parsed result
if err != nil {
    log.Printf("[SDK-Parser] ❌ ParseMessage error: %v", err)
} else if msg != nil {
    parsedJSON, _ := json.Marshal(msg)
    log.Printf("[SDK-Parser] ✅ ParseMessage output: %s", string(parsedJSON))
}
```

**输出示例：**
```
[SDK-Parser] ✅ ParseMessage output: {"Content":"Hello"}
```

---

### 5. parseUserMessage - 用户消息详情
**文件：** `internal/parser/json.go`  
**行号：** ~177-208

```go
func (p *Parser) parseUserMessage(data map[string]any) (*shared.UserMessage, error) {
    log.Printf("[SDK-Parser] 👤 Parsing UserMessage...")
    
    // ...
    log.Printf("[SDK-Parser] 👤 UserMessage content type: %T", content)
    
    switch c := content.(type) {
    case string:
        log.Printf("[SDK-Parser] 👤 UserMessage has string content: %q", c)
        // ...
    case []any:
        log.Printf("[SDK-Parser] 👤 UserMessage has %d content block(s)", len(c))
        // ...
        for i, blockData := range c {
            // ...
            log.Printf("[SDK-Parser] 👤   Block #%d: type=%T", i, block)
        }
    }
}
```

**输出示例：**
```
[SDK-Parser] 👤 Parsing UserMessage...
[SDK-Parser] 👤 UserMessage content type: string
[SDK-Parser] 👤 UserMessage has string content: "Hello"
```

---

### 6. parseAssistantMessage - 助手消息详情
**文件：** `internal/parser/json.go`  
**行号：** ~220-249

```go
func (p *Parser) parseAssistantMessage(data map[string]any) (*shared.AssistantMessage, error) {
    log.Printf("[SDK-Parser] 🤖 Parsing AssistantMessage...")
    
    // ...
    log.Printf("[SDK-Parser] 🤖 AssistantMessage has %d content block(s)", len(contentArray))
    log.Printf("[SDK-Parser] 🤖 AssistantMessage model: %s", model)
    
    for i, blockData := range contentArray {
        // ...
        log.Printf("[SDK-Parser] 🤖   Block #%d: type=%T", i, block)
    }
    
    log.Printf("[SDK-Parser] 🤖 AssistantMessage parsed successfully")
}
```

**输出示例：**
```
[SDK-Parser] 🤖 Parsing AssistantMessage...
[SDK-Parser] 🤖 AssistantMessage has 2 content block(s)
[SDK-Parser] 🤖 AssistantMessage model: claude-3-5-sonnet-20241022
[SDK-Parser] 🤖   Block #0: type=*shared.TextBlock
[SDK-Parser] 🤖   Block #1: type=*shared.ToolUseBlock
[SDK-Parser] 🤖 AssistantMessage parsed successfully
```

---

### 7. parseResultMessage - 结果消息详情
**文件：** `internal/parser/json.go`  
**行号：** ~271, 327-328

```go
func (p *Parser) parseResultMessage(data map[string]any) (*shared.ResultMessage, error) {
    log.Printf("[SDK-Parser] ✅ Parsing ResultMessage...")
    
    // ...
    
    log.Printf("[SDK-Parser] ✅ ResultMessage parsed: subtype=%s, session_id=%s, is_error=%v", 
        result.Subtype, result.SessionID, result.IsError)
}
```

**输出示例：**
```
[SDK-Parser] ✅ Parsing ResultMessage...
[SDK-Parser] ✅ ResultMessage parsed: subtype=success, session_id=session_abc123, is_error=false
```

---

## 🔍 完整日志流程示例

### 场景：CLI 返回一个 UserMessage

```
[SDK-Parser] 📥 Raw CLI line: {"type":"user","message":{"content":"What is 2+2?"}}
[SDK-Parser] 🔍 ParseMessage input: {"type":"user","message":{"content":"What is 2+2?"}}
[SDK-Parser] 🔍 Message type: user
[SDK-Parser] 👤 Parsing UserMessage...
[SDK-Parser] 👤 UserMessage content type: string
[SDK-Parser] 👤 UserMessage has string content: "What is 2+2?"
[SDK-Parser] ✅ ParseMessage output: {"Content":"What is 2+2?"}
[SDK-Parser] 📤 Parsed 1 message(s) from line
[SDK-Parser]   Message #0: type=*shared.UserMessage
```

**对比点：**
1. ✅ 原始 CLI: `"content":"What is 2+2?"`
2. ✅ 解析识别: content type = `string`
3. ✅ 最终结构: `{"Content":"What is 2+2?"}`

---

### 场景：CLI 返回 AssistantMessage with ToolUse

```
[SDK-Parser] 📥 Raw CLI line: {"type":"assistant","message":{"content":[{"type":"text","text":"I'll help"},{"type":"tool_use","id":"tool_123","name":"Read","input":{}}],"model":"claude-3-5-sonnet-20241022"}}
[SDK-Parser] 🔍 ParseMessage input: {"type":"assistant","message":{...}}
[SDK-Parser] 🔍 Message type: assistant
[SDK-Parser] 🤖 Parsing AssistantMessage...
[SDK-Parser] 🤖 AssistantMessage has 2 content block(s)
[SDK-Parser] 🤖 AssistantMessage model: claude-3-5-sonnet-20241022
[SDK-Parser] 🤖   Block #0: type=*shared.TextBlock
[SDK-Parser] 🤖   Block #1: type=*shared.ToolUseBlock
[SDK-Parser] 🤖 AssistantMessage parsed successfully
[SDK-Parser] ✅ ParseMessage output: {"Content":[{"Type":"text","Text":"I'll help"},{"Type":"tool_use","ID":"tool_123","Name":"Read","Input":{}}],"Model":"claude-3-5-sonnet-20241022"}
[SDK-Parser] 📤 Parsed 1 message(s) from line
[SDK-Parser]   Message #0: type=*shared.AssistantMessage
```

**对比点：**
1. ✅ 原始 CLI: 2 个 content blocks
2. ✅ 解析识别: TextBlock + ToolUseBlock
3. ✅ 字段映射: `"id":"tool_123"` → `"ID":"tool_123"` (正确)
4. ✅ 最终结构: Content 数组包含两个正确的 block

---

### 场景：CLI 返回 ResultMessage

```
[SDK-Parser] 📥 Raw CLI line: {"type":"result","subtype":"success","duration_ms":2000,"duration_api_ms":1800,"is_error":false,"num_turns":5,"session_id":"session_xyz","result":"Task completed"}
[SDK-Parser] 🔍 ParseMessage input: {"type":"result","subtype":"success",...}
[SDK-Parser] 🔍 Message type: result
[SDK-Parser] ✅ Parsing ResultMessage...
[SDK-Parser] ✅ ResultMessage parsed: subtype=success, session_id=session_xyz, is_error=false
[SDK-Parser] ✅ ParseMessage output: {"Subtype":"success","DurationMs":2000,"DurationAPIMs":1800,"IsError":false,"NumTurns":5,"SessionID":"session_xyz","Result":"Task completed"}
[SDK-Parser] 📤 Parsed 1 message(s) from line
[SDK-Parser]   Message #0: type=*shared.ResultMessage
```

**对比点：**
1. ✅ 原始 CLI: `"result":"Task completed"` (string 类型)
2. ✅ 解析识别: session_id = session_xyz
3. ✅ 最终结构: `"Result":"Task completed"` (正确的 string 类型)

---

## 🐛 如何使用这些日志诊断问题

### 问题 1：字段丢失

**症状：** Server 没有收到某个字段

**诊断步骤：**
1. 查看 `📥 Raw CLI line` - CLI 是否返回了该字段？
2. 查看 `🔍 ParseMessage input` - 字段是否存在于输入中？
3. 查看 `✅ ParseMessage output` - 字段是否存在于输出中？

**示例：**
```
// 如果 CLI 返回了 parent_tool_use_id，但解析输出中没有：
[SDK-Parser] 📥 Raw CLI line: {"type":"user","message":{"content":"Hi","parent_tool_use_id":"parent_123"}}
[SDK-Parser] 🔍 ParseMessage input: {"type":"user","message":{"content":"Hi","parent_tool_use_id":"parent_123"}}
[SDK-Parser] 👤 Parsing UserMessage...
[SDK-Parser] ✅ ParseMessage output: {"Content":"Hi"}  // ❌ parent_tool_use_id 丢失！
```

**结论：** 解析逻辑没有处理 `parent_tool_use_id` 字段

---

### 问题 2：类型不匹配

**症状：** 字段类型错误

**诊断步骤：**
1. 对比原始 JSON 中的类型
2. 对比解析后 JSON 中的类型

**示例：**
```
// CLI 返回 string，但解析成了 map：
[SDK-Parser] 📥 Raw CLI line: {"type":"result","result":"Success message"}
[SDK-Parser] ✅ ParseMessage output: {"Result":{"message":"Success message"}}  // ❌ 类型错误
```

**结论：** ResultMessage.Result 的类型定义错误（应该是 `*string` 而不是 `*map[string]any`）

---

### 问题 3：ContentBlock 类型错误

**症状：** ContentBlock 解析为错误的类型

**诊断步骤：**
1. 查看 `👤/🤖 Block #X: type=...` - 确认解析的类型
2. 对比原始 JSON 中的 `"type"` 字段

**示例：**
```
[SDK-Parser] 📥 Raw CLI line: {...,"content":[{"type":"tool_use","id":"tool_456",...}]}
[SDK-Parser] 🤖   Block #0: type=*shared.ToolUseBlock  // ✅ 正确
```

---

### 问题 4：字符串 vs 数组混淆

**症状：** UserMessage 字符串内容被当作数组处理

**诊断步骤：**
1. 查看 `👤 UserMessage content type: ...` - 确认识别的类型
2. 对比 Server 端的处理逻辑

**示例：**
```
[SDK-Parser] 📥 Raw CLI line: {"type":"user","message":{"content":"Hello"}}
[SDK-Parser] 👤 UserMessage content type: string  // ✅ 正确识别
[SDK-Parser] 👤 UserMessage has string content: "Hello"  // ✅ 正确处理
```

---

## 📊 日志图标说明

| 图标 | 含义 | 阶段 |
|------|------|------|
| 📥 | 原始输入 | CLI 返回的原始 JSON |
| 📤 | 解析结果 | 解析完成的消息数量 |
| 🔍 | 详细分析 | ParseMessage 的输入和类型 |
| ✅ | 成功输出 | ParseMessage 的最终输出 |
| ❌ | 错误 | 解析错误 |
| 👤 | 用户消息 | UserMessage 解析 |
| 🤖 | 助手消息 | AssistantMessage 解析 |
| ✅ | 结果消息 | ResultMessage 解析 |

---

## 🎯 使用建议

### 在开发环境启用
这些日志默认使用 Go 的标准 `log` 包，会输出到标准错误。

### 过滤日志
```bash
# 只看 SDK 解析日志
./server 2>&1 | grep '\[SDK-Parser\]'

# 只看原始 CLI 输出
./server 2>&1 | grep '📥 Raw CLI line'

# 只看解析结果
./server 2>&1 | grep '✅ ParseMessage output'
```

### 对比分析
1. 复制 `📥 Raw CLI line` 的 JSON
2. 复制 `✅ ParseMessage output` 的 JSON
3. 使用 JSON diff 工具对比

---

## ⚠️ 注意事项

### 性能影响
这些日志会：
- ✅ 增加少量 CPU 开销（JSON 序列化）
- ✅ 增加日志输出量
- ⚠️ **不建议在生产环境启用**

### 建议使用场景
- ✅ 开发调试
- ✅ 问题诊断
- ✅ 集成测试
- ❌ 生产环境（考虑使用环境变量控制）

---

## 🔧 未来改进

### 可选的增强
1. **环境变量控制**
   ```go
   if os.Getenv("SDK_DEBUG_PARSER") == "1" {
       log.Printf("[SDK-Parser] ...")
   }
   ```

2. **日志级别**
   - TRACE: 所有详细日志
   - DEBUG: 输入输出对比
   - INFO: 只记录错误

3. **结构化日志**
   - 使用 JSON 格式
   - 方便机器解析和分析

---

## 📝 总结

添加的日志提供了：
1. ✅ **CLI 原始输出** - 查看 Agent 实际返回的内容
2. ✅ **解析过程** - 跟踪每个解析步骤
3. ✅ **最终结构** - 确认 Go 数据结构
4. ✅ **类型信息** - 验证类型识别正确性

**使用这些日志可以快速定位：**
- 字段丢失问题
- 类型不匹配问题
- ContentBlock 解析问题
- 字符串 vs 数组混淆问题

**现在运行 Server，日志会自动显示 CLI 输出与解析结果的完整对比！** 🎉
