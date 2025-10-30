# Plugin Support Example

This example demonstrates how to load and use Claude Code plugins with the Go SDK.

## What are Plugins?

Claude Code plugins extend the functionality of Claude Code by adding custom commands and capabilities. Plugins can:

- Add custom slash commands
- Provide domain-specific tools
- Integrate with external services
- Customize Claude's behavior

## Usage

### Basic Plugin Loading

```go
import claudecode "github.com/jonnyquan/claude-agent-sdk-go"

result, err := claudecode.Query(
    ctx,
    "Your prompt here",
    claudecode.NewOptions(
        claudecode.WithPlugins(
            claudecode.PluginConfig{
                Type: claudecode.PluginTypeLocal,
                Path: "/path/to/plugin",
            },
        ),
    ),
)
```

### Multiple Plugins

```go
opts := claudecode.NewOptions()
opts.Plugins = []claudecode.PluginConfig{
    {
        Type: claudecode.PluginTypeLocal,
        Path: "/path/to/plugin1",
    },
    {
        Type: claudecode.PluginTypeLocal,
        Path: "/path/to/plugin2",
    },
}

result, err := claudecode.Query(ctx, "Your prompt", opts)
```

## Plugin Structure

A basic plugin directory structure:

```
my-plugin/
├── package.json          # Plugin manifest
└── commands/             # Custom commands
    ├── command1.js
    └── command2.js
```

### package.json Example

```json
{
  "name": "my-custom-plugin",
  "version": "1.0.0",
  "description": "Custom Claude Code plugin",
  "claude": {
    "commands": [
      {
        "name": "my-command",
        "description": "Description of the command",
        "file": "./commands/my-command.js"
      }
    ]
  }
}
```

### Command Implementation

```javascript
// commands/my-command.js
module.exports = async (args) => {
  // Command logic here
  return {
    content: [
      {
        type: 'text',
        text: 'Command output'
      }
    ]
  };
};
```

## Plugin Discovery

The SDK looks for plugins in:

1. Paths specified via `WithPlugins()` option
2. Custom plugin directories you configure

## CLI Mapping

When you configure plugins in the SDK:

```go
claudecode.WithPlugins(
    claudecode.PluginConfig{Type: claudecode.PluginTypeLocal, Path: "/path/to/plugin1"},
    claudecode.PluginConfig{Type: claudecode.PluginTypeLocal, Path: "/path/to/plugin2"},
)
```

The SDK automatically passes these to the CLI as:

```bash
claude --plugin-dir /path/to/plugin1 --plugin-dir /path/to/plugin2 [other options]
```

## Running the Example

```bash
cd examples/14_plugin_support
go run main.go
```

## Notes

- Currently only `PluginTypeLocal` is supported for local filesystem plugins
- Plugins must follow the Claude Code plugin structure
- Plugin paths must exist and be accessible
- Multiple plugins can be loaded simultaneously

## Related Documentation

- [Claude Code Plugin Documentation](https://docs.claude.com/plugins)
- [Creating Custom Plugins](https://docs.claude.com/plugins/creating)
- SDK Options API Reference
