//go:build ignore

// Simple agent — interactive chat.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/01_basics/simple_agent.go
//
// Or with a specific provider:
//
//	agent := agnogo.Agent("...", agnogo.WithOpenAI())
//	agent := agnogo.Agent("...", agnogo.WithAnthropic())
//	agent := agnogo.Agent("...", agnogo.WithOllama())
package main

import "github.com/saeedalam/agnogo"

func main() {
	agent := agnogo.Agent("You are a helpful assistant. Be concise.", agnogo.Debug)
	agent.CLI()
}
