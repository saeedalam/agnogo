//go:build ignore

// CLI agent — interactive terminal chat with built-in commands.
//
// Commands: exit, clear, memory, history, tools
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/07_advanced/cli_agent.go
package main

import (
	"context"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant. Be concise.",
		AutoMemory:   true,
		Debug:        &debug,
	})

	agent.Tool("calculate", "Do math calculations", agnogo.Params{
		"expression": {Type: "string", Desc: "Math expression (e.g. 2+2)", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		// Simple placeholder — in production use tools.Calculator()
		return "42", nil
	})

	// Start interactive CLI
	agent.CLI()
}
