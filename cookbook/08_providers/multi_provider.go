//go:build ignore

// Multi-provider — same agent code, different LLM backends.
//
// Set the API key for whichever provider you want to test:
//
//	OPENAI_API_KEY=sk-...     go run ./cookbook/08_providers/multi_provider.go openai
//	ANTHROPIC_API_KEY=sk-...  go run ./cookbook/08_providers/multi_provider.go anthropic
//	GEMINI_API_KEY=AIza...    go run ./cookbook/08_providers/multi_provider.go gemini
//	go run ./cookbook/08_providers/multi_provider.go ollama    # requires local Ollama
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/anthropic"
	"github.com/saeedalam/agnogo/providers/gemini"
	"github.com/saeedalam/agnogo/providers/ollama"
	"github.com/saeedalam/agnogo/providers/openai"
)

func getProvider(name string) agnogo.ModelProvider {
	switch name {
	case "openai":
		return openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	case "anthropic":
		return anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-5-20250514")
	case "gemini":
		return gemini.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
	case "ollama":
		return ollama.New("llama3.1")
	default:
		fmt.Fprintf(os.Stderr, "Unknown provider: %s\nUsage: go run multi_provider.go [openai|anthropic|gemini|ollama]\n", name)
		os.Exit(1)
		return nil
	}
}

func main() {
	provider := "openai"
	if len(os.Args) > 1 {
		provider = os.Args[1]
	}

	fmt.Printf("--- Using provider: %s ---\n\n", provider)

	model := getProvider(provider)
	debug := agnogo.VerboseDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant. Be concise — answer in 1-2 sentences.",
		Debug:        &debug,
	})

	agent.Tool("get_time", "Get the current date", nil,
		func(ctx context.Context, args map[string]string) (string, error) {
			return "2026-03-29, Saturday", nil
		})

	session := agnogo.NewSession("demo")
	resp, err := agent.Run(context.Background(), session, "What day is it today? Use the get_time tool.")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n" + resp.Text)

	if resp.Metrics != nil {
		fmt.Printf("\nMetrics: %d model calls, %d tool calls", resp.Metrics.ModelCalls, resp.Metrics.ToolCalls)
		if resp.Metrics.TotalTokens > 0 {
			fmt.Printf(", %d tokens", resp.Metrics.TotalTokens)
		}
		fmt.Printf(", %s\n", resp.Metrics.Duration)
	}
}
