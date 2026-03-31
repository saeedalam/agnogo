//go:build ignore

// Batch processing — run multiple tasks concurrently with Batch.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/09_easy/batch.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

func main() {
	agent := agnogo.Agent("Answer concisely in one sentence.")

	tasks := []agnogo.WorkerTask{
		{ID: "1", Message: "What is Go?"},
		{ID: "2", Message: "What is Rust?"},
		{ID: "3", Message: "What is Python?"},
	}

	results := agnogo.Batch(context.Background(), agent, tasks, 3)
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("Task %s: error: %v\n", r.TaskID, r.Err)
			continue
		}
		fmt.Printf("Task %s: %s\n", r.TaskID, r.Response.Text)
	}
}
