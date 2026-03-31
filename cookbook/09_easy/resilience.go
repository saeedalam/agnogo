//go:build ignore

// Resilience — Fallback + CircuitBreaker for reliable LLM calls.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/resilience.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	primary := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4o")
	fallback := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")

	model := agnogo.CircuitBreaker(
		agnogo.Fallback(primary, fallback),
		agnogo.WithFailureThreshold(3),
	)

	agent := agnogo.Agent("You are helpful.", agnogo.WithModel(model))
	answer, err := agent.Ask(context.Background(), "Hello!")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(answer)
}
