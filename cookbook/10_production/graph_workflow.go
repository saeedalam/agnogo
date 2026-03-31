//go:build ignore

// Graph workflow -- directed graph with branching and cycles.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/10_production/graph_workflow.go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/saeedalam/agnogo"
)

func main() {
	classify := agnogo.Agent("Classify the user message. If it's about a refund, respond with exactly 'REFUND'. Otherwise respond with 'SUPPORT'.")
	refund := agnogo.Agent("You handle refund requests. Be empathetic and process the refund. Keep it brief.")
	support := agnogo.Agent("You handle general support. Be helpful and concise.")

	g := agnogo.NewGraph()
	g.AddNode("classify", classify)
	g.AddNode("refund", refund)
	g.AddNode("support", support)
	g.SetEntry("classify")
	g.SetEnd("refund", "support")

	g.AddEdge("classify", "refund", func(ctx context.Context, state *agnogo.GraphState) bool {
		return strings.Contains(strings.ToUpper(state.GetStr("last_response")), "REFUND")
	})
	g.AddEdge("classify", "support", nil) // default

	session := agnogo.NewSession("demo")

	// Test refund path
	fmt.Println("--- Refund request ---")
	resp, err := g.Run(context.Background(), session, "I want a refund for order #456")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(resp.Text)

	// Test support path
	fmt.Println("\n--- Support request ---")
	session2 := agnogo.NewSession("demo2")
	resp, err = g.Run(context.Background(), session2, "How do I reset my password?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(resp.Text)
}
