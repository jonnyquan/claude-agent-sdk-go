# Changelog

## 0.1.77

### New Features

Mass parity sync with Python SDK 0.1.77 (skipping releases 0.1.20–0.1.76 in
the changelog; the highlights below cover the cumulative additions). The Go
SDK now tracks the same shape as Python for types, options, the wire
protocol, and the SessionStore subsystem.

#### Types and content blocks

- **ServerToolUseBlock / ServerToolResultBlock**: Surface server-executed
  tool calls (advisor, web_search, web_fetch, code_execution, etc.) and their
  results that were previously dropped.
- **HookEventMessage**: Routed `system/hook_started` and `system/hook_response`
  frames to a dedicated message type; emitted when `IncludeHookEvents` is true.
- **MirrorErrorMessage**: SDK-synthesized when a `SessionStore.Append` call
  fails so consumers can detect mirror gaps without aborting the session.
- **DeferredToolUse on ResultMessage**: Carries the deferred tool call when a
  PreToolUse hook returns `permissionDecision: "defer"`.
- **APIErrorStatus on ResultMessage**: HTTP status (e.g. 429, 500, 529) of a
  failing API call, safe to log.
- **ServerToolName constants**: Discriminator constants for known server tool
  names.

#### Options

- **Skills option**: New `Options.Skills` with `SkillsAll() / SkillsList(...) /
  SkillsNone()` constructors. Auto-injects `Skill` / `Skill(name)` into
  `AllowedTools` and defaults `SettingSources` to `[user, project]` when set.
- **IncludeHookEvents**: Enables hook lifecycle events in the message stream.
- **StrictMcpConfig**: Restricts MCP servers to those passed in `McpServers`,
  ignoring project / user / global / plugin sources.
- **SessionStore + SessionStoreFlush + LoadTimeoutMs**: Mirror transcripts to
  an external store; flush eagerly or batched; control resume Load timeouts.
- **ThinkingDisplay**: New `summarized` / `omitted` setting on
  `ThinkingConfig` (forwarded as `--thinking-display`).
- **EffortXHigh**: New Opus 4.7-only effort level (falls back to `high`
  elsewhere).
- **SandboxNetworkConfig**: Added `AllowedDomains`, `DeniedDomains`,
  `AllowManagedDomainsOnly`, `AllowMachLookup` for sandbox network policy.
- **SessionID**: Forwarded to the CLI as `--session-id` for caller-supplied
  session UUIDs.

#### Permissions

- **ToolPermissionContext enrichment**: New `BlockedPath`, `DecisionReason`,
  `Title`, `DisplayName`, `Description` fields propagated from
  `permission_request` control messages.
- **PermissionUpdateFromDict**: Constructs a `PermissionUpdate` from the
  control protocol dict format so suggestions can be inspected and echoed
  back without losing structural fidelity.
- **PermissionDecisionDefer**: New `"defer"` decision constant.
- **PostToolUseHookSpecificOutput.UpdatedToolOutput**: Replaces the tool
  output for any tool (not just MCP).

#### SessionStore subsystem

- **SessionStore protocol**: New `SessionStore` interface, `SessionKey`,
  `SessionStoreEntry`, `SessionStoreListEntry`, `SessionSummaryEntry`,
  `SessionListSubkeysKey` types, and `UnimplementedSessionStore` for the
  optional methods.
- **InMemorySessionStore**: Reference adapter for testing and development.
- **FoldSessionSummary / SummaryEntryToSDKInfo**: Incremental summary
  derivation so adapters can maintain `ListSessionSummaries` cheaply.
- **ProjectKeyForDirectory / FilePathToSessionKey**: Helpers for mapping
  on-disk transcripts to store keys.
- **\*FromStore async helpers**: `ListSessionsFromStore`,
  `GetSessionInfoFromStore`, `GetSessionMessagesFromStore`,
  `ListSubagentsFromStore`, `GetSubagentMessagesFromStore`.
- **\*ViaStore mutation helpers**: `RenameSessionViaStore`,
  `TagSessionViaStore`, `DeleteSessionViaStore`.
- **Subagent transcript helpers**: `ListSubagents` and `GetSubagentMessages`
  for reading subagent transcripts from disk.
- **Cascading `DeleteSession`**: Now removes the sibling
  `<sessionID>/subagents/` directory alongside the JSONL file.

#### Transport / Protocol

- **Skills sent on initialize**: `InitializeWithSkills` forwards an explicit
  skill allowlist so the CLI can filter which skills are loaded.
- **`--include-hook-events`, `--strict-mcp-config`, `--session-mirror`,
  `--thinking-display`, `--session-id`**: New CLI flags wired into the
  command builder.
- **`--setting-sources=` empty list**: Now emitted as a single argument so
  the CLI correctly disables all filesystem settings (Python parity).
- **Atexit cleanup**: Live CLI subprocesses are tracked in a global set and
  receive SIGTERM when the parent receives SIGINT/SIGTERM, preventing
  orphans (Python SDK parity).
- **Error result text replacement**: When the CLI emits a result with
  `is_error=true` and exits non-zero, `ProcessError.Error()` now includes the
  structured error text (e.g. "Reached maximum number of turns") instead of
  the bare exit code.
- **transcript_mirror frame routing**: Subprocess and alt transport now peel
  `transcript_mirror` frames off stdout (they are not yielded to consumers)
  and hand them to the configured `TranscriptMirrorBatcher`. Result messages
  trigger an explicit flush before being yielded so consumers can rely on
  the SessionStore being up to date for each turn.
- **OTEL trace context**: TRACEPARENT/TRACESTATE in the parent process env
  are inherited naturally by the subprocess. Callers using OTEL libraries
  that do not write the active span's context to env vars can populate
  `Options.ExtraEnv` explicitly per-query.

#### Client / Query orchestration

- `Client.Connect` and `Query` now run the full SessionStore lifecycle:
  `ValidateSessionStoreOptions` → `MaterializeResumeSession` →
  `ApplyMaterializedOptions` → `NewTranscriptMirrorBatcher` →
  transport.SetMirrorBatcher / SetMaterializedCleanup → `Connect`. Skipped
  when a custom transport is supplied.
- `MaterializedResume.Cleanup` is invoked automatically on transport `Close`
  and on `Connect` failure paths so the temp credentials directory does not
  leak.

#### Testing helpers

- **`RunSessionStoreConformance`**: 14-contract behavioral test harness for
  custom `SessionStore` adapters. Mirrors Python SDK's
  `claude_agent_sdk.testing.run_session_store_conformance`. Exposed as
  `claudesdk.RunSessionStoreConformance(t, factory, claudesdk.ConformanceOptions{})`
  with optional skip-list for adapters that don't implement
  `ListSessions` / `ListSessionSummaries` / `Delete` / `ListSubkeys`.
- Reference adapter `InMemorySessionStore` passes all 14 contracts.

#### Public API additions

- Re-exports for `ThinkingDisplay`, `ImportSessionOptions`,
  `RunSessionStoreConformance`, `ConformanceOptions` in `pkg/claudesdk`.

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.1.133 (was 2.1.4).
- Updated SDK version to 0.1.77 (was 0.1.19).

## 0.1.19

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.1.1
- **CI improvements**: Jobs requiring secrets now skip when running from forks (#451)
- Fixed YAML syntax error in create-release-tag workflow (#429)

## 0.1.18

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.0.74

## 0.1.17

### New Features

- **UserMessage UUID field**: Added `UUID` field to `UserMessage` response type, making it easier to use the `RewindFiles()` method by providing direct access to message identifiers needed for file checkpointing

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.0.70

## 0.1.16

### Bug Fixes

- **Rate limit detection**: Fixed parsing of the `Error` field in `AssistantMessage`, enabling applications to detect and handle API errors like rate limits. Previously, the `Error` field was defined but never populated from CLI responses

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.0.68

## 0.1.15

### New Features

- **File checkpointing and rewind**: Added `EnableFileCheckpointing` option to `Options` and `RewindFiles(ctx, userMessageID)` method to `Client`. This enables reverting file changes made during a session back to a specific checkpoint, useful for exploring different approaches or recovering from unwanted modifications

## 0.1.14

### New Features

- **Tools Option**: Added `tools` option to Options for controlling the base set of available tools
  - Array of tool names to specify which tools should be available (e.g., `[]string{"Read", "Edit", "Bash"}`)
  - Empty array `[]string{}` to disable all built-in tools
  - Preset object `ToolsPreset{Type: "preset", Preset: "claude_code"}` to use the default Claude Code toolset
  - New `WithTools(tools ToolsOption)` functional option

- **SDK Beta Support**: Added `betas` option for enabling Anthropic API beta features
  - New `SdkBeta` type with `SdkBetaContext1M` constant for extended context window
  - New `Betas` field in Options (`[]SdkBeta`)
  - New `WithBetas(betas ...SdkBeta)` functional option

- **File Checkpointing**: Added file checkpointing support for tracking file changes
  - New `EnableFileCheckpointing` field in Options
  - New `WithEnableFileCheckpointing(enable bool)` functional option
  - New `RewindFiles(ctx, userMessageID)` method on Client interface
  - Enables rewinding tracked files to their state at any user message

### Bug Fixes

- **Faster Error Handling**: Added `FailPendingRequests()` method to control protocol for propagating CLI errors to pending requests immediately instead of waiting for timeout

### Internal/Other Changes

- Updated SDK version to 0.1.14
- Updated bundled Claude CLI to version 2.0.62

### Alignment

- **100% Feature Parity** with Python SDK 0.1.14
  - Both SDKs now have tools option support
  - Both SDKs now have SDK beta support
  - Both SDKs now have file checkpointing and rewind_files support
  - Both SDKs now have faster error propagation for pending requests

## 0.1.10

### Features

- **Sandbox Configuration Support**: Added sandbox settings for bash command isolation
  - New `SandboxSettings` type for configuring sandbox behavior
  - New `SandboxNetworkConfig` type for network settings in sandbox
  - New `SandboxIgnoreViolations` type for specifying violations to ignore
  - New `WithSandbox(sandbox *SandboxSettings)` functional option
  - Sandbox settings are merged into CLI settings when provided
  - Supports all sandbox options: enabled, autoAllowBashIfSandboxed, excludedCommands, network, etc.

### Internal/Other Changes

- Updated bundled Claude CLI to version 2.0.57
- Changed `HookMatcher.Timeout` type from `*int` to `*float64` for consistency with Python SDK

## 0.1.9

### Features

- **Structured Outputs Support**: Agents can now return validated JSON matching your schema
  - New `OutputFormat` field in Options (`map[string]interface{}`)
  - New `WithOutputFormat(format map[string]interface{})` functional option
  - CLI passes `--json-schema` flag with schema definition
  - New `StructuredOutput` field in `ResultMessage`
  - Enables JSON schema validation for agent responses
  - Compatible with Claude's structured output API

- **Claude CLI Auto-Bundling**: Claude Code CLI is now optionally bundled with the SDK
  - New `findBundledCLI()` function for automatic CLI discovery
  - Added `_bundled/` directory structure for CLI binaries
  - New `BundledCLIVersion` constant (currently "2.0.50")
  - Bundled CLI takes priority over system-wide installations
  - Cross-platform support for bundled binaries
  - New `scripts/download_cli.go` utility for CLI downloading

- **Enhanced Hook System**: Added timeout support for hook execution
  - New `Timeout` field in `HookMatcher` (`*int`)
  - Timeout passed to CLI via control protocol
  - Per-matcher timeout configuration
  - Improved hook reliability and control

- **Enhanced Error Handling**: Added error field support for assistant messages  
  - New `AssistantMessageError` type with error constants
  - New `Error` field in `AssistantMessage` (`*AssistantMessageError`)
  - Error types: authentication_failed, billing_error, rate_limit, invalid_request, server_error, unknown
  - Better error reporting and handling

- **Fallback Model Support**: Automatic fallback model handling (already supported in Options)
  - Enhanced `FallbackModel` field usage
  - Improved reliability when primary model is unavailable
  - Parity with TypeScript/Python SDKs

### Technical Improvements

- Enhanced CLI command building with JSON schema support
- Improved bundled CLI discovery with multiple fallback paths
- Updated hook processing to include timeout configuration
- Extended message types with structured output and error support

### Documentation

- Updated all type exports with new structured output types
- Added CLI bundling documentation and utilities
- Enhanced hook timeout configuration examples

### Alignment

- **100% Feature Parity** with Python SDK 0.1.9
  - Both SDKs now have structured outputs support
  - Both SDKs now support CLI bundling
  - Both SDKs now have enhanced hook timeout support
  - Both SDKs now have improved error handling

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
