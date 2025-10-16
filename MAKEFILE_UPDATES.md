# Makefile Updates for Hook System

## 新增 Make 目标

### 1. Hook 系统测试

#### `test-hooks`
测试 Hook 系统所有组件

```bash
make test-hooks
```

**执行内容**:
- 运行所有 `TestHook*` 测试
- 运行所有 `TestPermission*` 测试
- 运行 `internal/query` 包的所有测试

**输出示例**:
```
=== Testing Hook System ===
go test -v -run "TestHook" ./...
✅ Hook system tests passed
```

### 2. Hook 示例运行

#### `example-hooks`
运行 Hook 使用示例

```bash
make example-hooks
```

**执行内容**:
- 运行 `examples/12_hooks/main.go` 演示程序
- 展示 Hook API 的实际使用方式

**输出示例**:
```
=== Hook Examples ===
Note: Hook examples demonstrate API usage patterns
Hook runtime is integrated in the SDK
✅ Hook examples completed
```

### 3. Hook 集成测试

#### `ci-hook-integration`
CI/CD 环境中的 Hook 集成测试

```bash
make ci-hook-integration
```

**执行内容**:
- 运行 `internal/query` 包测试（hook processor, control protocol）
- 运行所有 Hook 和 Permission API 测试

**输出示例**:
```
=== Hook System Integration ===
go test -v ./internal/query/...
✅ Hook integration tests passed
```

---

## 更新的现有目标

### `ci`
CI 管道现在包含 Hook 测试

```bash
make ci
```

**更新**:
- 添加 `test-hooks` 到 CI 流程
- 确保 Hook 系统在 CI 中被验证

**新流程**:
```
deps-verify → test-race → test-hooks → check → examples → sdk-test
```

### `release-check`
发布前检查现在包含 Hook 测试

```bash
make release-check
```

**更新**:
- 添加 `test-hooks` 到发布检查流程
- 确保 Hook 系统在发布前被完全验证

**新流程**:
```
test → test-hooks → check → examples → sdk-test → api-check → module-check
```

### `api-check`
API 检查现在展示 Hook 系统 API

```bash
make api-check
```

**更新**:
- 添加 Hook 系统 API 文档输出
- 展示 `HookEvent`, `HookCallback`, `PermissionResult` 等类型

**新输出**:
```
=== Hook System API ===
Hook types available
Hook callbacks available
Permission types available
```

---

## 使用场景

### 开发期间

```bash
# 快速测试 Hook 相关代码
make test-hooks

# 运行示例查看效果
make example-hooks
```

### 提交前验证

```bash
# 运行完整的 CI 流程
make ci
```

### 发布准备

```bash
# 检查发布就绪状态
make release-check
```

### 集成测试

```bash
# 仅运行 Hook 集成测试
make ci-hook-integration
```

---

## 与现有工作流集成

### 现有目标继续工作

所有现有的 Make 目标保持不变：

```bash
make test           # 运行所有测试（包含 Hook 测试）
make test-race      # 竞态检测（包含 Hook）
make test-cover     # 覆盖率测试（包含 Hook）
make examples       # 构建所有示例（包含 examples/12_hooks）
```

### Hook 测试自动包含在全局测试中

运行 `make test` 会自动测试 Hook 系统，因为：
- Hook 测试位于主包和 `internal/query` 包
- `make test` 运行 `go test ./...` 包含所有包

---

## 目标依赖关系

```
all → test → (包含 hook 测试)

ci → deps-verify + test-race + test-hooks + check + examples + sdk-test

release-check → test + test-hooks + check + examples + sdk-test + api-check + module-check

ci-hook-integration → (独立运行 hook 测试)
```

---

## 快速参考

| 目标 | 用途 | 耗时 |
|------|------|------|
| `test-hooks` | Hook 系统单元测试 | ~2秒 |
| `example-hooks` | Hook 示例演示 | ~1秒 |
| `ci-hook-integration` | Hook 集成测试 | ~2秒 |
| `ci` | 完整 CI 流程（含 Hook） | ~30秒 |
| `release-check` | 发布检查（含 Hook） | ~40秒 |

---

## 测试覆盖

Hook 系统测试覆盖：

```
✅ Hook API 层测试              (hooks_test.go, permissions_test.go)
✅ Hook 处理器测试              (hook_processor_test.go)
✅ Control Protocol 测试        (control_protocol.go 单元测试)
✅ Transport 集成测试           (transport_test.go)
✅ 端到端示例                   (examples/12_hooks)
```

总计测试数量：**52+ 测试**

---

## 注意事项

### 1. 测试隔离

`test-hooks` 目标专门测试 Hook 功能，可独立运行而不影响其他测试。

### 2. CI 集成

在 CI 环境中，建议使用 `make ci` 或 `make ci-hook-integration` 确保 Hook 系统正常工作。

### 3. 发布前

**必须运行**: `make release-check` 确保所有功能（包括 Hook）都经过验证。

### 4. 向后兼容

所有现有的 Make 目标行为保持不变，新目标是**额外的**工具。

---

## 完整目标列表

运行 `make help` 查看所有可用目标：

```bash
make help
```

Hook 相关目标会以醒目方式显示：

```
test-hooks           Test hook system components
example-hooks        Run hook examples
ci-hook-integration  Run Hook system integration tests
```

---

## 总结

Makefile 已成功更新以支持 Hook 系统开发和测试：

✅ 新增 3 个 Hook 专用目标  
✅ 更新 3 个现有目标以包含 Hook  
✅ 保持向后兼容  
✅ 集成到 CI/CD 流程  
✅ 支持独立 Hook 测试

开发者现在拥有完整的工具链来开发、测试和验证 Hook 系统功能。
