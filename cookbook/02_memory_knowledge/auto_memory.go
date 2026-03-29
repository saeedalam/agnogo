//go:build ignore

// Auto memory — the agent learns facts from conversation. Try:
//   "My name is Erik and my email is erik@test.com"
//   "What's my name?"
//   Type "memory" to see what was learned.
//
//	source .env && go run ./cookbook/02_memory_knowledge/auto_memory.go
package main

import (
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a friendly assistant. Remember personal details users share.",
		AutoMemory:   true,
		Debug:        &debug,
	})

	agent.CLI()
}
