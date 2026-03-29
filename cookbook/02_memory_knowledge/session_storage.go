//go:build ignore

// Session storage — persist conversations across restarts.
//
// This example uses the built-in MemoryStorage (in-memory).
// In production, swap with: storage/postgres, storage/sqlite, storage/redis, storage/mysql.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/02_memory_knowledge/session_storage.go
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
	store := agnogo.NewMemoryStorage()
	debug := agnogo.DefaultDebug()

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a helpful assistant. Remember context from earlier in the conversation.",
		Storage:      store,
		Debug:        &debug,
	})

	ctx := context.Background()

	// First conversation turn — creates and saves session automatically
	fmt.Println("--- Turn 1 ---")
	resp, _ := agent.RunWithStorage(ctx, "session-abc", "My favorite color is blue.")
	fmt.Println(resp.Text)

	// Second turn — loads session, has full history
	fmt.Println("\n--- Turn 2 ---")
	resp, _ = agent.RunWithStorage(ctx, "session-abc", "What's my favorite color?")
	fmt.Println(resp.Text)

	// List all sessions
	sessions, _ := store.List(ctx, 10)
	fmt.Printf("\n--- %d sessions in storage ---\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  Session %s: %d messages\n", s.ID, len(s.History))
	}

	// Add knowledge to the store
	store.AddKnowledge(ctx, "faq-hours", "We are open Monday-Friday 9-17.")
	store.AddKnowledge(ctx, "faq-parking", "Free parking is available.")
	entries, _ := store.ListKnowledge(ctx)
	fmt.Printf("\n--- %d knowledge entries ---\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s: %s\n", e.Key, e.Content)
	}
}
