//go:build ignore

// MCP Filesystem Agent — uses MCP protocol to browse and read local files.
//
// This example connects to the official MCP filesystem server as a subprocess
// and lets the agent browse directories and read files.
//
// Setup:
//
//	npm install -g @modelcontextprotocol/server-filesystem  # or use npx
//	source ../../.env && go run main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/mcp"
)

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Println("Run: source ../../.env && go run main.go")
		os.Exit(1)
	}

	ctx := context.Background()

	// Connect to the MCP filesystem server.
	// It will be started as a subprocess communicating over stdio.
	fmt.Println("🔌 Connecting to MCP filesystem server...")
	mcpClient, err := mcp.Connect(ctx,
		"npx", "-y", "@modelcontextprotocol/server-filesystem", ".",
	)
	if err != nil {
		fmt.Printf("❌ Failed to connect to MCP server: %v\n", err)
		fmt.Println("   Make sure npx is installed (comes with Node.js)")
		os.Exit(1)
	}
	defer mcpClient.Close()

	fmt.Printf("✅ Connected! Discovered %d tools: %s\n\n",
		mcpClient.ToolCount(),
		strings.Join(mcpClient.ToolNames(), ", "),
	)

	// Create an agent with MCP tools + reliability.
	agent := agnogo.Agent(
		`You are a helpful file explorer. You can browse directories and read files.
When asked about files, use your tools to actually look at the filesystem.
Always use tools — never guess file contents or directory listings.`,
		agnogo.WithOpenAI("gpt-4.1-mini"),
		agnogo.Tools(mcpClient.ToolDefs()...),
		agnogo.Reliable(),
		agnogo.Debug,
	)

	// Interactive chat loop.
	session := agnogo.NewSession("mcp-demo")
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("💬 Chat with the filesystem agent (type 'quit' to exit)")
	fmt.Println("   Try: 'List the files in the current directory'")
	fmt.Println("   Try: 'Read the README.md file'")
	fmt.Println()

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" {
			break
		}

		resp, err := agent.Run(ctx, session, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("\nAgent: %s\n\n", resp.Text)
		if len(resp.ToolsCalled) > 0 {
			fmt.Printf("  [tools used: %s]\n\n", strings.Join(resp.ToolsCalled, ", "))
		}
	}
}
