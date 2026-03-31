//go:build ignore

// Multi-provider — same agent code, different LLM backends.
//
// Agent() auto-detects the provider from environment variables. Or you can
// explicitly select one with WithOpenAI, WithAnthropic, WithGemini, etc.
//
//	OPENAI_API_KEY=sk-...     go run ./cookbook/08_providers/multi_provider.go
//	ANTHROPIC_API_KEY=sk-...  go run ./cookbook/08_providers/multi_provider.go
//	GEMINI_API_KEY=AIza...    go run ./cookbook/08_providers/multi_provider.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
)

func main() {
	// Auto-detect (uses first available provider from env)
	agent := agnogo.Agent("You are a helpful assistant. Be concise — answer in 1-2 sentences.")

	// Or explicitly select a provider:
	// agent := agnogo.Agent("You are helpful.", agnogo.WithOpenAI())
	// agent := agnogo.Agent("You are helpful.", agnogo.WithAnthropic())
	// agent := agnogo.Agent("You are helpful.", agnogo.WithGemini())
	// agent := agnogo.Agent("You are helpful.", agnogo.WithOllama())

	answer, err := agent.Ask(context.Background(), "What day of the week is it today?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(answer)
}
