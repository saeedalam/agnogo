//go:build ignore

// Streaming — stream responses chunk by chunk using AskStream.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/06_streaming/stream_response.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
)

func main() {
	agent := agnogo.Agent("You are a storyteller.")

	fmt.Println("--- Streaming response ---")

	ch := agent.AskStream(context.Background(), "Tell me a short story about a robot learning to cook.")
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
