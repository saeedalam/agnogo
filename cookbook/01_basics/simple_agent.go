//go:build ignore

// Simple agent — the hello world of agnogo.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/01_basics/simple_agent.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.VerboseDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant. Be concise — answer in 1-2 sentences.",
		Debug:        &debug,
	})

	session := agnogo.NewSession("demo")
	resp, err := agent.Run(context.Background(), session, "What is the capital of Sweden?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + resp.Text)

	if resp.Metrics != nil && resp.Metrics.TotalTokens > 0 {
		fmt.Printf("\nTokens used: %d\n", resp.Metrics.TotalTokens)
	}
}
