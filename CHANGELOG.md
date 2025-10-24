# Changelog

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
