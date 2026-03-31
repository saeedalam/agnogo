//go:build ignore

// One-liner — simplest possible agent with a single question.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/one_liner.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	agent := agnogo.Agent("You are a helpful assistant.")
	answer, err := agent.Ask(context.Background(), "What is the capital of France?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(answer)
}
