//go:build ignore

// Parallel workflow — multiple agents run concurrently, results merged.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/04_workflows/parallel.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/saeedalam/agnogo"
)

func main() {
	weather := agnogo.Agent("You are a weather reporter. Give a brief weather forecast for Stockholm today. 2 sentences max.")
	news := agnogo.Agent("You are a news anchor. Give 3 brief fictional headline items for today. Keep it short.")
	calendar := agnogo.Agent("You are a calendar assistant. Create a sample daily schedule for a busy professional. List 5 items for today.")

	wf := agnogo.Parallel(
		agnogo.Step("weather", weather),
		agnogo.Step("news", news),
		agnogo.Step("calendar", calendar),
	)

	session := agnogo.NewSession("demo")
	resp, err := wf.Run(context.Background(), session, "Give me my morning briefing")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Morning Briefing (3 agents ran in parallel) ---")
	fmt.Println(resp.Text)
}
