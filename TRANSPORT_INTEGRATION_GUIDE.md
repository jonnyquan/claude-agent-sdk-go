# Transport 集成指南 - Python SDK vs Go SDK

## 📐 架构对比

### Python SDK 架构

```
┌─────────────────────────────────────────┐
│           Client (InternalClient)       │
│  - 用户接口                              │
│  - 创建 Query 和 Transport              │
└──────────────┬──────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────┐
│              Query                      │
│  - Control Protocol 处理                │
│  - Hook 回调执行                        │
│  - Permission 请求处理                  │
│  - 消息路由（control vs regular）       │
└──────────────┬──────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────┐
│         Transport (Subprocess)          │
│  - CLI 进程管理                         │
│  - read_messages() - 原始 JSON 读取     │
│  - write() - 原始数据写入               │
│  - 不处理消息类型，只做 I/O             │
└─────────────────────────────────────────┘
```

### Go SDK 当前架构

```
┌─────────────────────────────────────────┐
│              Client                     │
│  - 用户接口                              │
│  - 创建 Transport                       │
└──────────────┬──────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────┐
│            Transport                    │
│  - CLI 进程管理                         │
│  - ReceiveMessages() - 解析并返回消息   │
│  - SendMessage() - 发送消息             │
│  - ❌ 目前包含所有逻辑（需要分离）      │
└─────────────────────────────────────────┘
```

### Go SDK 目标架构（推荐）

```
┌─────────────────────────────────────────┐
│              Client                     │
│  - 用户接口                              │
│  - 创建 Transport + ControlProtocol     │
└──────────────┬──────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────┐
│         ControlProtocol + HookProcessor │
│  - Control Protocol 处理                │
│  - Hook 回调执行                        │
│  - Permission 请求处理                  │
│  - 消息路由（control vs regular）       │
└──────────────┬──────────────────────────┘
               │
               ↓
┌─────────────────────────────────────────┐
│            Transport                    │
│  - CLI 进程管理                         │
│  - 原始读/写接口                        │
│  - 不处理消息类型，只做 I/O             │
└─────────────────────────────────────────┘
```

---

## 🔍 Python SDK 详细实现

### 1. Transport 层 - 纯 I/O

**职责**: 只负责原始数据的读写

```python
class SubprocessCLITransport(Transport):
    async def write(self, data: str) -> None:
        """写入原始数据（通常是 JSON + newline）"""
        await self._stdin_stream.send(data)
    
    def read_messages(self) -> AsyncIterator[dict[str, Any]]:
        """读取并解析 JSON，不处理消息类型"""
        async for line in self._stdout_stream:
            data = json.loads(line)
            yield data  # 只解析 JSON，不路由
```

**关键点**:
- ✅ Transport 不知道消息类型
- ✅ Transport 不区分 control_request 和普通消息
- ✅ Transport 只是"哑管道"

### 2. Query 层 - 智能路由

**职责**: 读取 Transport 的消息并路由

```python
class Query:
    async def _read_messages(self) -> None:
        """从 Transport 读取消息并路由"""
        async for message in self.transport.read_messages():
            msg_type = message.get("type")
            
            # 路由控制响应
            if msg_type == "control_response":
                response = message.get("response", {})
                request_id = response.get("request_id")
                # 唤醒等待的请求
                if request_id in self.pending_control_responses:
                    event = self.pending_control_responses[request_id]
                    self.pending_control_results[request_id] = response
                    event.set()
                continue
            
            # 路由控制请求（来自 CLI）
            elif msg_type == "control_request":
                request = message
                # 在后台任务中处理
                self._tg.start_soon(self._handle_control_request, request)
                continue
            
            # 普通消息发送给 Client
            await self._message_send.send(message)
```

**关键点**:
- ✅ Query 是消息路由器
- ✅ 控制消息在 Query 内部处理
- ✅ 普通消息通过 channel 发送给 Client

### 3. Hook 处理

**职责**: 在 Query 中处理来自 CLI 的 hook 请求

```python
async def _handle_control_request(self, request: SDKControlRequest) -> None:
    """处理来自 CLI 的控制请求"""
    subtype = request["request"]["subtype"]
    
    if subtype == "hook_callback":
        # 获取 callback ID
        callback_id = request["request"]["callback_id"]
        callback = self.hook_callbacks.get(callback_id)
        
        # 调用用户的 hook 函数
        hook_output = await callback(
            request["request"]["input"],
            request["request"].get("tool_use_id"),
            {"signal": None}
        )
        
        # 发送响应回 CLI
        response = {
            "type": "control_response",
            "response": {
                "subtype": "success",
                "request_id": request["request_id"],
                "response": hook_output
            }
        }
        await self.transport.write(json.dumps(response) + "\n")
```

### 4. 初始化流程

```python
# Client 创建 Query
query = Query(
    transport=transport,
    is_streaming_mode=True,
    hooks=self._convert_hooks(options.hooks)
)

# 启动消息读取
await query.start()  # 启动 _read_messages 任务

# 发送初始化请求
await query.initialize()  # 发送 hooks 配置到 CLI

# Query.initialize() 实现
async def initialize(self) -> dict[str, Any]:
    # 构建 hooks 配置
    hooks_config = {}
    for event, matchers in self.hooks.items():
        hooks_config[event] = []
        for matcher in matchers:
            callback_ids = []
            for callback in matcher["hooks"]:
                callback_id = f"hook_{self.next_callback_id}"
                self.hook_callbacks[callback_id] = callback
                callback_ids.append(callback_id)
            hooks_config[event].append({
                "matcher": matcher["matcher"],
                "hookCallbackIds": callback_ids
            })
    
    # 发送初始化请求到 CLI
    request = {
        "subtype": "initialize",
        "hooks": hooks_config
    }
    response = await self._send_control_request(request)
    return response
```

---

## 🎯 Go SDK 集成方案

### 方案 A: 最小改动（推荐）

**思路**: 在 Transport 内部集成 ControlProtocol，保持外部接口不变

#### 修改 Transport 结构

```go
type Transport struct {
    // ... 现有字段 ...
    
    // Hook 支持（仅在流式模式）
    controlProtocol *query.ControlProtocol
    hookProcessor   *query.HookProcessor
    isStreamingMode bool
    
    // 用于分离普通消息和控制消息
    regularMsgChan chan shared.Message
    rawStdoutChan  chan []byte  // 原始 stdout 数据
}
```

#### Connect 时初始化

```go
func (t *Transport) Connect(ctx context.Context) error {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    // ... 现有代码 ...
    
    // 检查是否是流式模式（有 hooks 或需要双向通信）
    t.isStreamingMode = !t.closeStdin
    
    if t.isStreamingMode && t.options != nil && len(t.options.Hooks) > 0 {
        // 创建 hook processor
        t.hookProcessor = query.NewHookProcessor(ctx, t.options)
        
        // 创建 control protocol
        t.controlProtocol = query.NewControlProtocol(
            ctx,
            t.hookProcessor,
            t.createWriteFn(),
        )
        
        // 发送初始化请求
        if _, err := t.controlProtocol.Initialize(); err != nil {
            return fmt.Errorf("failed to initialize control protocol: %w", err)
        }
    }
    
    // ... 启动进程等现有代码 ...
    
    return nil
}
```

#### 修改消息读取逻辑

```go
func (t *Transport) handleStdout(ctx context.Context) {
    scanner := bufio.NewScanner(t.stdout)
    
    for scanner.Scan() {
        line := scanner.Text()
        
        // 解析 JSON 获取消息类型
        var rawMsg map[string]any
        if err := json.Unmarshal([]byte(line), &rawMsg); err != nil {
            // 错误处理
            continue
        }
        
        msgType, _ := rawMsg["type"].(string)
        
        // 路由控制消息
        if t.controlProtocol != nil && 
           (msgType == "control_request" || 
            msgType == "control_response" || 
            msgType == "control_cancel_request") {
            // 处理控制消息
            if err := t.controlProtocol.HandleIncomingMessage(msgType, []byte(line)); err != nil {
                // 记录错误但继续
                t.errChan <- fmt.Errorf("control protocol error: %w", err)
            }
            continue  // 不发送到普通消息通道
        }
        
        // 普通消息继续原有处理
        msg, err := t.parser.ProcessLine(line)
        if err != nil {
            t.errChan <- err
            continue
        }
        
        t.msgChan <- msg
    }
}
```

#### 创建 writeFn

```go
func (t *Transport) createWriteFn() func([]byte) error {
    return func(data []byte) error {
        t.mu.RLock()
        stdin := t.stdin
        connected := t.connected
        t.mu.RUnlock()
        
        if !connected || stdin == nil {
            return fmt.Errorf("transport not connected")
        }
        
        _, err := stdin.Write(data)
        return err
    }
}
```

### 方案 B: 完全分离（类似 Python）

**思路**: 创建独立的 Query 层，Transport 只做 I/O

这需要更大的重构，但架构更清晰。暂时不推荐。

---

## 📋 集成检查清单

### 第一阶段：基础集成 ✅

- [x] 创建 ControlProtocol 类型
- [x] 创建 HookProcessor 类型
- [x] 实现消息路由逻辑
- [x] 实现 hook 回调执行
- [x] 单元测试通过

### 第二阶段：Transport 集成 ⚠️

- [ ] 修改 Transport 结构体
- [ ] 在 Connect 中初始化 ControlProtocol
- [ ] 修改 handleStdout 路由逻辑
- [ ] 创建 writeFn
- [ ] 处理流式模式检测

### 第三阶段：测试 ⚠️

- [ ] 修改现有 Transport 测试
- [ ] 添加 hook 集成测试
- [ ] 添加 permission 集成测试
- [ ] E2E 测试

---

## 🔧 具体实现步骤

### 步骤 1: 修改 Transport 结构（5 分钟）

```go
// 在 internal/subprocess/transport.go

import (
    "github.com/jonnyquan/claude-agent-sdk-go/internal/query"
)

type Transport struct {
    // ... 所有现有字段保持不变 ...
    
    // Hook 支持 - 新增字段
    controlProtocol *query.ControlProtocol
    hookProcessor   *query.HookProcessor
    isStreamingMode bool
}
```

### 步骤 2: 修改 Connect 方法（15 分钟）

```go
func (t *Transport) Connect(ctx context.Context) error {
    t.mu.Lock()
    defer t.mu.Unlock()

    if t.connected {
        return fmt.Errorf("transport already connected")
    }

    // ... 所有现有的 CLI 启动代码保持不变 ...

    // ===== 新增: Hook 初始化 =====
    // 只在流式模式且有 hooks 时初始化
    t.isStreamingMode = !t.closeStdin
    
    if t.isStreamingMode && t.options != nil && len(t.options.Hooks) > 0 {
        // 创建 hook processor
        t.hookProcessor = query.NewHookProcessor(ctx, t.options)
        
        // 创建 control protocol
        writeFn := func(data []byte) error {
            t.mu.RLock()
            defer t.mu.RUnlock()
            if !t.connected || t.stdin == nil {
                return fmt.Errorf("transport not connected")
            }
            _, err := t.stdin.Write(data)
            return err
        }
        
        t.controlProtocol = query.NewControlProtocol(ctx, t.hookProcessor, writeFn)
        
        // 发送初始化请求到 CLI
        if _, err := t.controlProtocol.Initialize(); err != nil {
            return fmt.Errorf("failed to initialize control protocol: %w", err)
        }
    }
    // ===== Hook 初始化结束 =====

    // ... 现有的启动 goroutine 代码保持不变 ...
    
    t.connected = true
    return nil
}
```

### 步骤 3: 修改 handleStdout 方法（20 分钟）

```go
func (t *Transport) handleStdout(ctx context.Context) {
    defer t.wg.Done()

    scanner := bufio.NewScanner(t.stdout)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return
        default:
        }

        line := scanner.Text()
        if line == "" {
            continue
        }

        // ===== 新增: 路由控制消息 =====
        // 先解析 JSON 获取消息类型
        var rawMsg map[string]any
        if err := json.Unmarshal([]byte(line), &rawMsg); err != nil {
            // 解析失败，继续原有处理
            msg, parseErr := t.parser.ProcessLine(line)
            if parseErr != nil {
                t.errChan <- parseErr
                continue
            }
            t.msgChan <- msg
            continue
        }
        
        msgType, _ := rawMsg["type"].(string)
        
        // 如果是控制消息，路由到 control protocol
        if t.controlProtocol != nil {
            switch msgType {
            case "control_request", "control_response", "control_cancel_request":
                if err := t.controlProtocol.HandleIncomingMessage(msgType, []byte(line)); err != nil {
                    // 记录错误但不中断
                    t.errChan <- fmt.Errorf("control protocol error: %w", err)
                }
                continue  // 不要发送到普通消息通道
            }
        }
        // ===== 控制消息路由结束 =====

        // 普通消息继续原有处理
        msg, err := t.parser.ProcessLine(line)
        if err != nil {
            t.errChan <- err
            continue
        }

        t.msgChan <- msg
    }

    // ... 现有的错误处理代码保持不变 ...
}
```

### 步骤 4: 添加 Close 清理（5 分钟）

```go
func (t *Transport) Close() error {
    t.mu.Lock()
    defer t.mu.Unlock()

    if !t.connected {
        return nil
    }

    // ===== 新增: 清理 control protocol =====
    if t.controlProtocol != nil {
        if err := t.controlProtocol.Close(); err != nil {
            // 记录错误但继续清理
        }
        t.controlProtocol = nil
        t.hookProcessor = nil
    }
    // ===== 清理结束 =====

    // ... 所有现有的清理代码保持不变 ...
    
    t.connected = false
    return nil
}
```

---

## ⚡ 快速集成（完整代码）

### 完整的 Transport 修改

<details>
<summary>点击查看完整代码</summary>

```go
// 在 import 中添加
import (
    "github.com/jonnyquan/claude-agent-sdk-go/internal/query"
)

// 在 Transport struct 中添加字段
type Transport struct {
    // ... 所有现有字段 ...
    
    // Hook support
    controlProtocol *query.ControlProtocol
    hookProcessor   *query.HookProcessor
    isStreamingMode bool
}

// Connect 方法中添加（在 t.connected = true 之前）
if !t.closeStdin && t.options != nil && len(t.options.Hooks) > 0 {
    t.isStreamingMode = true
    t.hookProcessor = query.NewHookProcessor(ctx, t.options)
    
    writeFn := func(data []byte) error {
        t.mu.RLock()
        defer t.mu.RUnlock()
        if !t.connected || t.stdin == nil {
            return fmt.Errorf("transport not connected")
        }
        _, err := t.stdin.Write(data)
        return err
    }
    
    t.controlProtocol = query.NewControlProtocol(ctx, t.hookProcessor, writeFn)
    
    if _, err := t.controlProtocol.Initialize(); err != nil {
        return fmt.Errorf("failed to initialize control protocol: %w", err)
    }
}

// handleStdout 方法中修改（在 scanner.Scan() 循环内）
line := scanner.Text()
if line == "" {
    continue
}

var rawMsg map[string]any
if err := json.Unmarshal([]byte(line), &rawMsg); err == nil {
    if t.controlProtocol != nil {
        msgType, _ := rawMsg["type"].(string)
        switch msgType {
        case "control_request", "control_response", "control_cancel_request":
            if err := t.controlProtocol.HandleIncomingMessage(msgType, []byte(line)); err != nil {
                t.errChan <- fmt.Errorf("control protocol error: %w", err)
            }
            continue
        }
    }
}

// 普通消息处理
msg, err := t.parser.ProcessLine(line)
if err != nil {
    t.errChan <- err
    continue
}
t.msgChan <- msg

// Close 方法中添加（在开始位置）
if t.controlProtocol != nil {
    t.controlProtocol.Close()
    t.controlProtocol = nil
    t.hookProcessor = nil
}
```

</details>

---

## 🧪 测试策略

### 单元测试

```go
func TestTransport_WithHooks(t *testing.T) {
    ctx := context.Background()
    
    hookCalled := false
    testHook := func(input HookInput, toolUseID *string, ctx HookContext) (HookJSONOutput, error) {
        hookCalled = true
        return NewPreToolUseOutput(PermissionDecisionAllow, "", nil), nil
    }
    
    options := shared.NewOptions()
    options.Hooks = map[string][]any{
        string(HookEventPreToolUse): {
            HookMatcher{
                Matcher: "Bash",
                Hooks: []HookCallback{testHook},
            },
        },
    }
    
    transport := New(cliPath, options, false, "sdk-go-client", Version)
    
    // 验证 hook processor 被创建
    err := transport.Connect(ctx)
    assert.NoError(t, err)
    assert.NotNil(t, transport.hookProcessor)
    assert.NotNil(t, transport.controlProtocol)
}
```

---

## 📊 总结对比

| 方面 | Python SDK | Go SDK (当前) | Go SDK (集成后) |
|------|-----------|---------------|----------------|
| **架构** | 三层：Client→Query→Transport | 两层：Client→Transport | 三层：Client→Transport(含CP) |
| **Transport 职责** | 纯 I/O | 所有逻辑 | I/O + 消息路由 |
| **Hook 处理** | 在 Query 层 | ❌ 未实现 | 在 Transport 内的 CP |
| **消息路由** | Query._read_messages | ❌ 无 | Transport.handleStdout |
| **代码改动** | N/A | N/A | 最小（~50 行） |

---

## ✅ 集成后的效果

用户代码完全一样：

```go
func main() {
    securityHook := func(input HookInput, ...) (HookJSONOutput, error) {
        // ... hook 逻辑 ...
        return NewPreToolUseOutput(PermissionDecisionAllow, "", nil), nil
    }
    
    err := WithClient(ctx, func(client Client) error {
        return client.Query(ctx, "List files")
    },
        WithHook(HookEventPreToolUse, HookMatcher{
            Matcher: "Bash",
            Hooks: []HookCallback{securityHook},
        }),
    )
}
```

内部自动工作：
1. ✅ Client 创建 Transport 时传入 hooks
2. ✅ Transport.Connect 初始化 ControlProtocol
3. ✅ Transport 发送初始化请求到 CLI
4. ✅ CLI 在工具调用前发送 hook_callback 请求
5. ✅ Transport 路由到 ControlProtocol
6. ✅ ControlProtocol 调用用户的 hook 函数
7. ✅ 返回结果给 CLI
8. ✅ CLI 根据结果决定是否执行工具

---

**预计工作时间**: 1-2 小时  
**代码改动量**: ~50-80 行（最小改动）  
**风险等级**: 低（不影响现有功能）

