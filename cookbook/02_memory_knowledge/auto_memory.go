//go:build ignore

// Auto memory — the agent learns facts from the conversation.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/02_memory_knowledge/auto_memory.go
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
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a friendly assistant. Remember personal details users share.",
		AutoMemory:   true,
		Debug:        &debug,
	})

	session := agnogo.NewSession("user-42")

	// First message — shares personal info
	fmt.Println("--- Message 1: Introducing myself ---")
	resp, _ := agent.Run(context.Background(), session, "Hi! My name is Erik and my email is erik@example.com")
	fmt.Println(resp.Text)

	// Check what was remembered
	fmt.Println("\n--- Memories extracted ---")
	for k, v := range session.Memory {
		fmt.Printf("  %s = %s\n", k, v)
	}

	// Second message — the agent should know the user's name
	fmt.Println("\n--- Message 2: Testing memory ---")
	resp, _ = agent.Run(context.Background(), session, "What's my name?")
	fmt.Println(resp.Text)

	fmt.Printf("\nSession has %d messages in history\n", len(session.History))
}
