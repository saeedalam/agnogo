//go:build ignore

// Auto memory — the agent learns facts from conversation. Try:
//   "My name is Erik and my email is erik@test.com"
//   "What's my name?"
//   Type "memory" to see what was learned.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/02_memory_knowledge/auto_memory.go
package main

import "github.com/saeedalam/agnogo"

func main() {
	agent := agnogo.Agent(
		"You are a friendly assistant. Remember personal details users share.",
		agnogo.Memory,
		agnogo.Debug,
	)
	agent.CLI()
}
