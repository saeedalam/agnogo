//go:build ignore

// Router workflow — dynamic routing to named workflows.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/04_workflows/router.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	refundAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You handle refund requests. Ask for the order number and process the refund. Be empathetic.",
		Debug:        &debug,
	})

	techAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You handle technical support. Help troubleshoot issues step by step.",
		Debug:        &debug,
	})

	salesAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You handle sales inquiries. Describe products and pricing enthusiastically.",
		Debug:        &debug,
	})

	wf := agnogo.Route(
		func(ctx context.Context, input string) string {
			lower := strings.ToLower(input)
			switch {
			case strings.Contains(lower, "refund") || strings.Contains(lower, "money back"):
				return "refund"
			case strings.Contains(lower, "broken") || strings.Contains(lower, "error") || strings.Contains(lower, "bug"):
				return "tech"
			default:
				return "sales"
			}
		},
		map[string]agnogo.Workflow{
			"refund": agnogo.Sequential(agnogo.Step("r", refundAgent)),
			"tech":   agnogo.Sequential(agnogo.Step("t", techAgent)),
			"sales":  agnogo.Sequential(agnogo.Step("s", salesAgent)),
		},
	)

	session := agnogo.NewSession("demo")
	ctx := context.Background()

	messages := []string{
		"I want a refund for order #12345",
		"I keep getting an error when I try to log in",
		"Tell me about your premium package",
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
