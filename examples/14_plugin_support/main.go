package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	claudecode "github.com/jonnyquan/claude-agent-sdk-go"
)

func main() {
	// Example 1: Using WithPlugins functional option
	example1()

	// Example 2: Direct Options configuration
	example2()
}

func example1() {
	fmt.Println("=== Example 1: Using WithPlugins ===")

	// Get the plugin directory path
	// This assumes you have a plugin directory structure like:
	// my-plugins/
	//   ├── commands/
	//   │   └── custom-command.js
	//   └── package.json
	pluginDir := getPluginPath()

	ctx := context.Background()
	iter, err := claudecode.Query(
		ctx,
		"List available commands", // This will include commands from the plugin
		claudecode.WithPlugins(
			claudecode.PluginConfig{
				Type: claudecode.PluginTypeLocal,
				Path: pluginDir,
			},
		),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	fmt.Println("Query completed successfully")
	for {
		msg, err := iter.Next(ctx)
		if err != nil {
			if err == claudecode.ErrNoMoreMessages {
				break
			}
			log.Printf("Error: %v", err)
			break
		}
		if userMsg, ok := msg.(*claudecode.AssistantMessage); ok {
			for _, block := range userMsg.Content {
				if textBlock, ok := block.(*claudecode.TextBlock); ok {
					fmt.Printf("Assistant: %s\n", textBlock.Text)
				}
			}
		}
	}
}

func example2() {
	fmt.Println("\n=== Example 2: Multiple Plugins ===")

	// Load multiple plugin directories
	plugin1 := getPluginPath()
	plugin2 := filepath.Join(os.Getenv("HOME"), ".claude/plugins/another-plugin")

	ctx := context.Background()

	// Create options with multiple plugins
	opts := claudecode.NewOptions()
	opts.Plugins = []claudecode.PluginConfig{
		{
			Type: claudecode.PluginTypeLocal,
			Path: plugin1,
		},
		{
			Type: claudecode.PluginTypeLocal,
			Path: plugin2,
		},
	}

	iter, err := claudecode.Query(
		ctx,
		"Use custom plugin functionality",
		claudecode.WithPlugins(opts.Plugins...),
	)

	if err != nil {
		log.Printf("Query error: %v\n", err)
		return
	}
	defer iter.Close()

	fmt.Println("Query with multiple plugins completed successfully")
	for {
		msg, err := iter.Next(ctx)
		if err != nil {
			if err == claudecode.ErrNoMoreMessages {
				break
			}
			log.Printf("Error: %v", err)
			break
		}
		if assistantMsg, ok := msg.(*claudecode.AssistantMessage); ok {
			for _, block := range assistantMsg.Content {
				if textBlock, ok := block.(*claudecode.TextBlock); ok {
					fmt.Printf("Assistant: %s\n", textBlock.Text)
				}
			}
		}
	}
}

func getPluginPath() string {
	// Try to find a plugin directory
	// First check if there's a local example plugin
	examplePlugin := "./example-plugin"
	if stat, err := os.Stat(examplePlugin); err == nil && stat.IsDir() {
		return examplePlugin
	}

	// Check user's home directory
	home := os.Getenv("HOME")
	if home != "" {
		userPlugin := filepath.Join(home, ".claude/plugins/my-plugin")
		if stat, err := os.Stat(userPlugin); err == nil && stat.IsDir() {
			return userPlugin
		}
	}

	// Default to current directory
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "my-plugin")
}

/*
Example Plugin Structure:

my-plugin/
├── package.json
└── commands/
    ├── greet.js
    └── analyze.js

package.json:
{
  "name": "my-custom-plugin",
  "version": "1.0.0",
  "description": "Custom Claude Code plugin",
  "main": "index.js",
  "claude": {
    "commands": [
      {
        "name": "greet",
        "description": "Greet the user",
        "file": "./commands/greet.js"
      },
      {
        "name": "analyze",
        "description": "Analyze data",
        "file": "./commands/analyze.js"
      }
    ]
  }
}

commands/greet.js:
module.exports = async (args) => {
  const name = args.name || 'User';
  return {
    content: [
      {
        type: 'text',
        text: `Hello, ${name}! This is a custom plugin command.`
      }
    ]
  };
};

commands/analyze.js:
module.exports = async (args) => {
  const data = args.data || [];
  return {
    content: [
      {
        type: 'text',
        text: `Analysis complete: Found ${data.length} items.`
      }
    ]
  };
};
*/
