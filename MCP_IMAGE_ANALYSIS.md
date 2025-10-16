# MCP ImageContent 分析

## 发现的情况

### Python SDK
1. **ContentBlock 类型**（Agent SDK 核心）：
   ```python
   ContentBlock = TextBlock | ThinkingBlock | ToolUseBlock | ToolResultBlock
   ```
   ❌ **不包含** ImageBlock

2. **ImageContent**（MCP 协议）：
   ```python
   from mcp.types import ImageContent, TextContent, Tool
   ```
   ✅ 来自 MCP 包，用于 **MCP 工具返回值**

3. **使用场景**：
   ```python
   # 在 MCP 工具的返回值中
   content: list[TextContent | ImageContent] = []
   if item.get("type") == "image":
       content.append(
           ImageContent(
               type="image",
               data=item["data"],
               mimeType=item["mimeType"],
           )
       )
   ```

### Go SDK
1. **ImageBlock** 存在并有完整示例：
   - `examples/13_image_content/main.go`
   - 有测试文件：`message_image_test.go`
   - 作为 ContentBlock 的一部分

2. **ImageBlock 结构**：
   ```go
   type ImageBlock struct {
       Type     string `json:"type"`
       Data     string `json:"data"`
       MimeType string `json:"mimeType"`
   }
   ```

3. **用于 AssistantMessage**：
   ```go
   message := &claudecode.AssistantMessage{
       Content: []claudecode.ContentBlock{
           &claudecode.TextBlock{...},
           &claudecode.ImageBlock{...}, // ✅ 可以包含
       },
   }
   ```

## 关键差异

| 方面 | Python SDK | Go SDK | 对齐？ |
|------|-----------|--------|--------|
| ContentBlock 包含 Image | ❌ No | ✅ Yes | ❌ 不对齐 |
| MCP ImageContent | ✅ 有（mcp.types） | ❓ 需检查 | ❓ 未知 |
| 图像在 AssistantMessage | ❌ 不支持 | ✅ 支持 | ❌ 不对齐 |

## 问题

### 我刚才的修改是否正确？

**删除 ImageBlock 的理由：**
- ✅ Python SDK 的 ContentBlock 不包含 ImageBlock
- ✅ 保持与 Python SDK 的严格对齐

**但是：**
- ❌ Go SDK 有完整的 ImageBlock 实现和示例
- ❌ 删除会导致 example 13 无法编译
- ❌ 可能是 Go SDK 的合理扩展功能

## 可能的解释

### 选项 1：Go SDK 是扩展版本
- Go SDK 提供了比 Python SDK 更多的功能
- ImageBlock 是 Go SDK 的特色功能
- **不应该删除**

### 选项 2：Python SDK 即将支持
- Python SDK 可能正在开发中
- Go SDK 先实现了这个功能
- **不应该删除**

### 选项 3：设计不一致
- Go SDK 实现了规范之外的功能
- 应该通过 MCP ImageContent 而不是 ContentBlock
- **应该修改但不是删除**

## 建议

### 方案 A：恢复 ImageBlock（推荐）

**理由：**
1. Go SDK 有完整的实现和示例
2. 不破坏现有功能
3. 在注释中标注与 Python SDK 的差异

**实现：**
```go
// ImageBlock represents an image content block.
// Note: This is a Go SDK extension. Python SDK does not include ImageBlock
// in ContentBlock types. Python SDK uses MCP ImageContent for images.
type ImageBlock struct {
    Type     string `json:"type"`
    Data     string `json:"data"`
    MimeType string `json:"mimeType"`
}
```

### 方案 B：保持删除但添加 MCP ImageContent 支持

**理由：**
1. 严格遵循 Python SDK 的设计
2. 通过 MCP 协议处理图像

**实现：**
- 删除 ImageBlock 作为 ContentBlock
- 添加 MCP ImageContent 类型
- 更新示例使用 MCP 方式

## 我的判断错误

我之前删除 ImageBlock 是基于：
- ✅ Python SDK 的 ContentBlock 不包含它（这是对的）
- ❌ 但忽略了 Go SDK 的完整实现（这是错的）

**应该：**
1. 恢复 ImageBlock
2. 添加清晰的文档说明这是扩展功能
3. 同时检查是否需要添加 MCP ImageContent 支持

## 下一步行动

需要确认：
1. **是否恢复 ImageBlock？**
2. **是否需要添加 MCP ImageContent 类型？**
3. **如何处理这种 SDK 间的差异？**
