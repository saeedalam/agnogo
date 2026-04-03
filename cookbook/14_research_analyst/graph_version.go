// Graph Version — Same Research Pipeline Using the Graph API
//
// This file shows the ALTERNATIVE approach using agnogo's Graph API
// instead of the WorkflowEngine. Use Graph for simpler, linear flows.
// Use WorkflowEngine for complex pipelines with parallel steps, HITL,
// and structured data flow.
//
// Run:
//   go run graph_version.go "Research topic"

//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
)

func graphMain() {
	ctx := context.Background()
	question := "Research AI agent frameworks"
	if len(os.Args) > 1 {
		question = strings.Join(os.Args[1:], " ")
	}

	// Create agents
	researchAgent := agnogo.Agent(
		"You are a research analyst. Provide thorough, factual analysis.",
		agnogo.Reliable(),
	)
	refineAgent := agnogo.Agent(
		"You refine and improve research reports.",
	)
	formatAgent := agnogo.Agent(
		"You format reports into clear, professional documents.",
	)

	// ── Graph Pipeline ─────────────────────────────────────────────
	//
	// Graph API is simpler than WorkflowEngine:
	//   - Nodes are agents or functions
	//   - Edges define flow with optional conditions
	//   - State is shared via GraphState
	//
	// Trade-off: no parallel steps, no HITL, no structured data flow.
	// Use for straightforward linear/branching flows.

	g := agnogo.NewGraph()

	// Node 1: Research (LLM agent)
	g.AddNode("research", researchAgent)

	// Node 2: Quality check (pure Go function — no LLM call)
	g.AddFuncNode("check-quality", func(_ context.Context, state *agnogo.GraphState) error {
		resp := state.GetStr("last_response")
		isGood := len(resp) > 500
		state.Set("quality_passed", isGood)
		state.Set("word_count", len(strings.Fields(resp)))
		fmt.Printf("  Quality check: %d words (pass: %v)\n",
			len(strings.Fields(resp)), isGood)
		return nil
	})

	// Node 3: Refine (LLM agent — only reached if quality is low)
	g.AddNode("refine", refineAgent)

	// Node 4: Format (LLM agent — final output)
	g.AddNode("format", formatAgent)

	// Define the flow
	g.SetEntry("research")
	g.SetEnd("format")

	// research → check-quality (always)
	g.AddEdge("research", "check-quality", nil)

	// check-quality → refine (if quality is low)
	g.AddEdge("check-quality", "refine", func(_ context.Context, s *agnogo.GraphState) bool {
		return !s.GetBool("quality_passed")
	})

	// check-quality → format (if quality is good)
	g.AddEdge("check-quality", "format", func(_ context.Context, s *agnogo.GraphState) bool {
		return s.GetBool("quality_passed")
	})

	// refine → format (after refinement)
	g.AddEdge("refine", "format", nil)

	// Run
	fmt.Printf("=== Graph Pipeline ===\n")
	fmt.Printf("Question: %s\n\n", question)

	session := agnogo.NewSession("graph-research")
	resp, err := g.Run(ctx, session, question)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== REPORT ===\n")
	fmt.Println(resp.Text)
}
