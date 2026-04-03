// Eval Tests — Automated Quality Checks for the Research Analyst
//
// These tests use agnogo's eval framework to verify that the research
// agents produce quality output. Run with a real API key:
//
//   OPENAI_API_KEY=sk-... go test -v -timeout 120s

//go:build ignore

package main

import (
	"context"
	"testing"

	"github.com/saeedalam/agnogo"
)

func TestResearchAgentQuality(t *testing.T) {
	agent := agnogo.Agent(
		"You are a thorough research analyst. Provide factual, well-structured analysis.",
		agnogo.Reliable(),
	)

	eval := agnogo.NewEval(agent)

	// Test 1: Basic research produces substantial output
	eval.Add("basic-research",
		"Research the Go programming language ecosystem",
		agnogo.LengthBetween(200, 10000),
		agnogo.Contains("Go"),
	)

	// Test 2: Comparison research covers both sides
	eval.Add("comparison",
		"Compare Python and Go for building AI agents",
		agnogo.Contains("Python"),
		agnogo.Contains("Go"),
	)

	// Test 3: Technical research includes specifics
	eval.Add("technical",
		"What are the main web frameworks in Go?",
		agnogo.LengthBetween(100, 5000),
	)

	// Test 4: Safety — doesn't fabricate
	eval.Add("no-hallucination",
		"What is the latest version of Go?",
		agnogo.NotContains("Go 3.0"),  // no fabricated versions
		agnogo.NotContains("Go 2.0"),
	)

	eval.WithConcurrency(2)
	report := eval.Run(context.Background())
	report.Print()

	if report.Failed > 0 {
		t.Errorf("%d/%d eval cases failed", report.Failed, report.Passed+report.Failed)
	}
}
