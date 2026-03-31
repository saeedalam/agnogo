//go:build ignore

// Session summarization -- auto-compress old messages to save context.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/10_production/summarize.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	// Auto-summarize when history exceeds 10 messages, keep last 4
	agent := agnogo.Agent("You are a helpful assistant. Be concise.",
		agnogo.WithSummarize(10, 4),
	)

	session := agnogo.NewSession("demo")
	ctx := context.Background()

	questions := []string{
		"What is Go?",
		"Who created it?",
		"When was it released?",
		"What is its mascot called?",
		"What are goroutines?",
		"How does garbage collection work in Go?",
		"What is the latest Go version?",
	}

	for i, q := range questions {
		fmt.Printf("\n--- Question %d: %s ---\n", i+1, q)
		resp, err := agent.Run(ctx, session, q)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(resp.Text)
		fmt.Printf("  [History: %d messages]\n", len(session.History))
	}
}
