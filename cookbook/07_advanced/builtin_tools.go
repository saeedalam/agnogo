//go:build ignore

// Built-in tools — use pre-built tools from the tools package.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/07_advanced/builtin_tools.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
	"github.com/saeedalam/agnogo/tools"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.VerboseDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant with access to a calculator and JSON tools. Use them when appropriate.",
		Debug:        &debug,
	})

	// Add built-in tools
	agent.AddTools(tools.Calculator()...)
	agent.AddTools(tools.JSON()...)

	session := agnogo.NewSession("demo")
	ctx := context.Background()

	questions := []string{
		"What is the square root of 144?",
		"Calculate 15 factorial",
		`Parse this JSON and tell me the name: {"name": "Erik", "age": 30, "city": "Stockholm"}`,
	}

	for _, q := range questions {
		fmt.Printf("\n--- Q: %s ---\n", q)
		resp, err := agent.Run(ctx, session, q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
