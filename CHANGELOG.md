# Changelog

## 0.1.6

### Features

- **Budget Control**: Add `max_budget_usd` option to limit API costs
  - New `MaxBudgetUSD` field in Options (`*float64`)
  - New `WithMaxBudgetUSD(budget float64)` functional option
  - CLI passes `--max-budget-usd` flag
  - Returns `error_max_budget_usd` when budget is exceeded
  - Includes cost tracking in ResultMessage

- **Thinking Token Limit**: Enable `max_thinking_tokens` option
  - Existing `MaxThinkingTokens` field now fully functional
  - Uncommented CLI flag passing (`--max-thinking-tokens`)
  - Controls extended thinking depth and reasoning cost
  - Helps balance speed, cost, and quality

### Examples

- Add `examples/16_max_budget_usd/` demonstrating budget control
  - 4 scenarios: no limit, reasonable, tight, thinking tokens
  - Cost tracking and error handling examples
  - Comprehensive README with best practices

### Bug Fixes

- **System prompt defaults**: Fixed issue where a default system prompt was being used when none was specified. The SDK now correctly uses an empty system prompt by default, giving users full control over agent behavior
- **CLI path discovery**: Added `~/.claude/local/claude` to the list of standard CLI search paths for improved Claude Code discovery

### Testing

- Add `options_test.go` with budget control tests
  - TestWithMaxBudgetUSD
  - TestWithMaxThinkingTokens  
  - TestWithMaxBudgetUSDAndMaxThinkingTokens

### Documentation

- Update option documentation with cost control notes
- Add budget exceeded error handling examples
- Document timing: budget checked after each API call

### Alignment

- **100% Feature Parity** with Python SDK 0.1.6
  - Both SDKs now have max_budget_usd
  - Both SDKs now have max_thinking_tokens fully enabled

## 0.1.5

### Major Features

- **SDK MCP Server Support** 🎉: Added full support for in-process MCP servers, achieving 100% feature parity with Python SDK
  - `CreateSDKMcpServer()` function to create in-process MCP servers
  - `ToolDef` type for defining tools with type-safe schemas and handlers
  - `Tool()` convenience function for concise tool creation
  - Support for both text and image content in tool results
  - `TextContent` and `ImageContent` types for tool return values
  - Direct access to application state from tool handlers
  - Zero external dependencies - pure Go implementation
  - Full context.Context support for cancellation and timeouts
  - Complete MCP protocol implementation in `internal/mcp` package

- **Plugin support**: Added the ability to load Claude Code plugins programmatically through the SDK
  - New `PluginConfig` type with `Type` and `Path` fields
  - New `WithPlugins()` functional option for configuration
  - CLI automatically passes `--plugin-dir` flags for each configured plugin
  - Currently supports `PluginTypeLocal` for local filesystem plugins

### Examples

- Added comprehensive SDK MCP Server example (`examples/15_sdk_mcp_server/`)
  - Simple text tools
  - Image content generation
  - Multiple tools in one server
  - Application state access
  - Context timeout handling

- Added plugin support example (`examples/14_plugin_support/`)
  - Basic plugin loading
  - Multiple plugins configuration
  - Plugin structure documentation

### Documentation

- Added `SDK_DEEP_COMPARISON_REPORT.md` (1166 lines)
  - Complete Python vs Go SDK comparison
  - Architecture analysis
  - Feature-by-feature comparison
  - Use case recommendations

### Breaking Changes

None - fully backward compatible

## 0.1.4 (Previous)

### Features

- **Plugin support**: Added the ability to load Claude Code plugins programmatically through the SDK. Plugins can be specified using the new `Plugins` field in `Options` with a `PluginConfig` type that supports loading local plugins by path. This enables SDK applications to extend functionality with custom commands and capabilities defined in plugin directories
  - New `PluginConfig` type with `Type` and `Path` fields
  - New `WithPlugins()` functional option for configuration
  - CLI automatically passes `--plugin-dir` flags for each configured plugin
  - Currently supports `PluginTypeLocal` for local filesystem plugins

## 0.1.4

### Features

- **Skip version check**: Added `CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK` environment variable to allow users to disable the Claude Code version check. Set this environment variable to skip the minimum version validation when the SDK connects to Claude Code. (Only recommended if you already have Claude Code 2.0.0 or higher installed, otherwise some functionality may break)
- SDK MCP server tool calls can now return image content blocks
- **Windows command line length limit handling**: Added automatic detection and handling of Windows command line length limits. When command line exceeds platform limits (8000 chars on Windows), large arguments like `--agents` are automatically written to temporary files and referenced via `@filepath` syntax.

### Bug Fixes

- Improved Windows compatibility by preventing command line overflow errors when using large agent configurations

## 0.1.3

### Features

- Initial release with full Python SDK feature parity
- Client API for bidirectional streaming
- Query API for one-shot requests
- Hook system for extension points
- MCP server support
- Comprehensive agent configuration
