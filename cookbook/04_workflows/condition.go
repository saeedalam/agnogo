//go:build ignore

// Condition workflow — branch based on input evaluation.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/04_workflows/condition.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
)

func main() {
	urgentAgent := agnogo.Agent("You handle URGENT customer issues. Be empathetic, apologize, and promise immediate resolution. Keep it brief.")
	normalAgent := agnogo.Agent("You handle standard customer inquiries. Be helpful and professional. Keep it brief.")

	wf := agnogo.Condition(
		func(ctx context.Context, input string) bool {
			lower := strings.ToLower(input)
			return strings.Contains(lower, "urgent") ||
				strings.Contains(lower, "emergency") ||
				strings.Contains(lower, "broken") ||
				strings.Contains(lower, "down")
		},
		agnogo.Sequential(agnogo.Step("urgent", urgentAgent)),
		agnogo.Sequential(agnogo.Step("normal", normalAgent)),
	)

	session := agnogo.NewSession("demo")
	ctx := context.Background()

	messages := []string{
		"Hi, I have a question about your pricing",
		"URGENT: Our booking system is broken and customers can't book!",
	}

	for _, msg := range messages {
		fmt.Printf("\n--- Input: %s ---\n", msg)
		resp, err := wf.Run(ctx, session, msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
