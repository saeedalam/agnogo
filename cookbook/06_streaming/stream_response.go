//go:build ignore

// Streaming — stream responses word by word (simulated).
//
// For real token-level SSE streaming, use agent.RunStreamReal() with a
// provider that implements StreamProvider (e.g. OpenAI).
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/06_streaming/stream_response.go
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
		Instructions: "You are a storyteller. Write a very short story (4-5 sentences) about a robot learning to cook.",
		Debug:        &debug,
	})

	session := agnogo.NewSession("demo")

	fmt.Println("--- Streaming response (word by word) ---")

	ch := agent.RunStream(context.Background(), session, "Tell me a story")
	for chunk := range ch {
		if chunk.Error != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", chunk.Error)
			break
		}
		if chunk.Done {
			fmt.Println("\n\n--- Stream complete ---")
			break
		}
		fmt.Print(chunk.Text)
	}
}
