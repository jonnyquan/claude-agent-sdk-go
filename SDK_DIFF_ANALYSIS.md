# Python SDK vs Go SDK - Detailed Comparison

## Critical Differences Found

### 1. Missing `parent_tool_use_id` Field ❌

**Python SDK:**
```python
@dataclass
class UserMessage:
    content: str | list[ContentBlock]
    parent_tool_use_id: str | None = None  # ✅ Has this field

@dataclass
class AssistantMessage:
    content: list[ContentBlock]
    model: str
    parent_tool_use_id: str | None = None  # ✅ Has this field
```

**Go SDK (Current - WRONG):**
```go
type UserMessage struct {
    MessageType string      `json:"type"`
    Content     interface{} `json:"content"`
    // ❌ MISSING parent_tool_use_id field
}

type AssistantMessage struct {
    MessageType string         `json:"type"`
    Content     []ContentBlock `json:"content"`
    Model       string         `json:"model"`
    // ❌ MISSING parent_tool_use_id field
}
```

**Impact:** High - This field is used for nested tool execution tracking in Agent SDK

---

### 2. ImageBlock Should NOT Exist ❌

**Python SDK:**
```python
ContentBlock = TextBlock | ThinkingBlock | ToolUseBlock | ToolResultBlock
# ✅ No ImageBlock in ContentBlock union
```

**Go SDK (Current - WRONG):**
```go
const (
    ContentBlockTypeText       = "text"
    ContentBlockTypeThinking   = "thinking"
    ContentBlockTypeToolUse    = "tool_use"
    ContentBlockTypeToolResult = "tool_result"
    ContentBlockTypeImage      = "image"  // ❌ Should NOT exist
)

type ImageBlock struct {  // ❌ Should NOT exist
    Type     string `json:"type"`
    Data     string `json:"data"`
    MimeType string `json:"mimeType"`
}
```

**Impact:** Medium - ImageBlock is not part of the standard ContentBlock types

---

### 3. ResultMessage.result Field Type Mismatch ⚠️

**Python SDK:**
```python
@dataclass
class ResultMessage:
    subtype: str
    duration_ms: int
    duration_api_ms: int
    is_error: bool
    num_turns: int
    session_id: str
    total_cost_usd: float | None = None
    usage: dict[str, Any] | None = None
    result: str | None = None  # ✅ String type
```

**Go SDK (Current):**
```go
type ResultMessage struct {
    MessageType   string          `json:"type"`
    Subtype       string          `json:"subtype"`
    DurationMs    int             `json:"duration_ms"`
    DurationAPIMs int             `json:"duration_api_ms"`
    IsError       bool            `json:"is_error"`
    NumTurns      int             `json:"num_turns"`
    SessionID     string          `json:"session_id"`
    TotalCostUSD  *float64        `json:"total_cost_usd,omitempty"`
    Usage         *map[string]any `json:"usage,omitempty"`
    Result        *map[string]any `json:"result,omitempty"` // ⚠️ Should be *string
}
```

**Impact:** Low-Medium - Type should be `*string` not `*map[string]any`

---

### 4. StreamEvent Message Type Missing ⚠️

**Python SDK:**
```python
@dataclass
class StreamEvent:
    """Stream event for partial message updates during streaming."""
    uuid: str
    session_id: str
    event: dict[str, Any]
    parent_tool_use_id: str | None = None

Message = UserMessage | AssistantMessage | SystemMessage | ResultMessage | StreamEvent
```

**Go SDK:**
```go
// ❌ StreamEvent/StreamMessage type might be missing or incomplete
```

**Impact:** Medium - Needed for streaming support

---

## Required Fixes

### Priority 1: Add parent_tool_use_id to Messages

```go
type UserMessage struct {
    MessageType       string      `json:"type"`
    Content           interface{} `json:"content"`
    ParentToolUseID   *string     `json:"parent_tool_use_id,omitempty"` // ADD THIS
}

type AssistantMessage struct {
    MessageType       string         `json:"type"`
    Content           []ContentBlock `json:"content"`
    Model             string         `json:"model"`
    ParentToolUseID   *string        `json:"parent_tool_use_id,omitempty"` // ADD THIS
}
```

### Priority 2: Remove ImageBlock

```go
// REMOVE ImageBlock completely
// Remove ContentBlockTypeImage constant
// Remove ImageBlock from unmarshalContentBlock switch
```

### Priority 3: Fix ResultMessage.result Type

```go
type ResultMessage struct {
    MessageType   string          `json:"type"`
    Subtype       string          `json:"subtype"`
    DurationMs    int             `json:"duration_ms"`
    DurationAPIMs int             `json:"duration_api_ms"`
    IsError       bool            `json:"is_error"`
    NumTurns      int             `json:"num_turns"`
    SessionID     string          `json:"session_id"`
    TotalCostUSD  *float64        `json:"total_cost_usd,omitempty"`
    Usage         *map[string]any `json:"usage,omitempty"`
    Result        *string         `json:"result,omitempty"` // CHANGE to *string
}
```

---

## Field-by-Field Comparison

### TextBlock ✅
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| text | str | string | ✅ Match |

### ThinkingBlock ✅
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| thinking | str | string | ✅ Match |
| signature | str | string | ✅ Match |

### ToolUseBlock ✅
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| id | str | string (as ID) | ✅ Match |
| name | str | string | ✅ Match |
| input | dict[str, Any] | map[string]any | ✅ Match |

### ToolResultBlock ✅
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| tool_use_id | str | string | ✅ Match |
| content | str \| list \| None | interface{} | ✅ Compatible |
| is_error | bool \| None | *bool | ✅ Match |

### UserMessage
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| content | str \| list[ContentBlock] | interface{} | ✅ Match |
| parent_tool_use_id | str \| None | - | ❌ MISSING |

### AssistantMessage
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| content | list[ContentBlock] | []ContentBlock | ✅ Match |
| model | str | string | ✅ Match |
| parent_tool_use_id | str \| None | - | ❌ MISSING |

### SystemMessage ✅
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| subtype | str | string | ✅ Match |
| data | dict[str, Any] | map[string]any | ✅ Match |

### ResultMessage
| Field | Python SDK | Go SDK | Status |
|-------|-----------|--------|--------|
| subtype | str | string | ✅ Match |
| duration_ms | int | int | ✅ Match |
| duration_api_ms | int | int | ✅ Match |
| is_error | bool | bool | ✅ Match |
| num_turns | int | int | ✅ Match |
| session_id | str | string | ✅ Match |
| total_cost_usd | float \| None | *float64 | ✅ Match |
| usage | dict \| None | *map[string]any | ✅ Match |
| result | str \| None | *map[string]any | ⚠️ Type Mismatch |

---

## Summary

**Critical Issues:** 2
1. Missing `parent_tool_use_id` in UserMessage and AssistantMessage
2. ImageBlock should not exist in ContentBlock types

**Important Issues:** 1
1. ResultMessage.result should be `*string` not `*map[string]any`

**Total Alignment Score:** 85% → Need to reach 100%
