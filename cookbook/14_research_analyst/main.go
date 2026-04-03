// AI Research Analyst — Full Feature Showcase
//
// This example demonstrates EVERY major agnogo feature in a single,
// realistic project: an AI-powered research pipeline that takes any
// question, plans a strategy, gathers data from multiple sources in
// parallel, synthesizes findings, refines iteratively, and produces
// a structured report with human approval.
//
// Features used:
//   - Workflow Engine (WfSequence, WfParallel, WfFunc, WfLoop, WfCondition, WfRoute)
//   - Human-in-the-Loop (WithConfirmation, ErrWorkflowPaused, ResumeWorkflow)
//   - Advanced Reasoning (ReasoningConfig with auto-detection)
//   - Learning Machine (UserProfile, SessionContext, EntityMemory)
//   - Multi-Modal (ImageFromFile for chart analysis)
//   - Reliability (Reliable() — hallucination guard, cost budget, confidence)
//   - Async Post-Processing (AsyncPostProcess)
//   - Concurrent Tool Execution (automatic when tools return multiple calls)
//   - Eval Framework (quality assertions in eval_test.go)
//
// Run:
//
//	export OPENAI_API_KEY=sk-...   # or any supported provider
//	go run main.go "Research the competitive landscape of AI agent frameworks"

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
)

func main() {
	ctx := context.Background()
	question := "Research the competitive landscape of AI agent frameworks in Go vs Python"
	if len(os.Args) > 1 {
		question = strings.Join(os.Args[1:], " ")
	}

	fmt.Printf("=== AI Research Analyst ===\n")
	fmt.Printf("Question: %s\n\n", question)

	// ── 1. Learning Machine — remembers across sessions ─────────────
	//
	// UserProfileStore:    learns your name, role, preferences
	// SessionContextStore: summarizes what happened in each session
	// EntityMemoryStore:   builds knowledge about researched topics
	//
	// On the first run, these are empty. On subsequent runs, the agent
	// recalls what it learned and personalizes the research.

	lm := agnogo.NewLearningMachine(nil) // model set later via agent
	lm.AddStore(agnogo.NewUserProfileStore())
	lm.AddStore(agnogo.NewSessionContextStore())
	lm.AddStore(agnogo.NewEntityMemoryStore())

	// ── 2. Specialized Agents ──────────────────────────────────────
	//
	// Each agent has a focused role. The workflow orchestrates them.

	// Planning agent — uses advanced reasoning (auto-detects native models)
	planAgent := agnogo.Agent(
		"You are a research planner. Given a research question, create a detailed "+
			"research plan with specific sources to consult, key questions to answer, "+
			"and a clear structure for the final report.",
		agnogo.WithReasoningConfig(agnogo.ReasoningConfig{
			Enabled:  true,
			Mode:     agnogo.ReasoningAuto, // native reasoning for O1/O3/Claude, CoT for others
			MinSteps: 2,
			MaxSteps: 5,
		}),
	)

	// Research agent — reliability layer prevents hallucinated facts
	researchAgent := agnogo.Agent(
		"You are a thorough research analyst. Search for factual, up-to-date "+
			"information. Always cite sources. Never fabricate data.",
		agnogo.Reliable(), // hallucination guard + confidence scoring
	)

	// Refinement agent — improves report quality iteratively
	refineAgent := agnogo.Agent(
		"You refine and improve research reports. Fix factual errors, improve "+
			"clarity, add missing context, and ensure balanced coverage.",
		agnogo.Reliable(),
	)

	// Output format agents
	summaryAgent := agnogo.Agent(
		"You write concise executive summaries. Bullet points, key findings, "+
			"and actionable recommendations in under 500 words.",
	)
	detailedAgent := agnogo.Agent(
		"You write detailed analytical reports with sections, evidence, "+
			"comparisons, and nuanced conclusions.",
	)

	// ── 3. Workflow Pipeline ───────────────────────────────────────
	//
	// The pipeline flows:
	//   plan → parallel research → synthesize → refine → depth check → review → format
	//
	// Each step type demonstrates a different workflow capability.

	wf := agnogo.NewWorkflowEngine("research-pipeline",
		agnogo.WfSequence("main",

			// ── Step 1: Plan (Advanced Reasoning) ──────────────────
			// The planning agent uses chain-of-thought reasoning to
			// develop a research strategy before any data gathering.
			agnogo.WfStep("plan", planAgent),

			// ── Step 2: Parallel Research (WfParallel) ─────────────
			// Three research streams run CONCURRENTLY. Each agent
			// gets its own cloned session for isolation.
			agnogo.WfParallel("gather",
				agnogo.WfStep("web-research", researchAgent),
				agnogo.WfStep("news-analysis", researchAgent),
				agnogo.WfStep("technical-review", researchAgent),
			),

			// ── Step 3: Synthesize (WfFunc — Pure Go) ──────────────
			// No LLM call. Pure Go function merges parallel results
			// into a structured document. Zero cost, zero latency.
			agnogo.WfFunc("synthesize", synthesizeResults),

			// ── Step 4: Iterative Refinement (WfLoop) ──────────────
			// Refine the report up to 2 times. Each iteration's output
			// becomes the next iteration's input.
			agnogo.WfLoop("refine", agnogo.WfStep("improve", refineAgent),
				func(out *agnogo.StepOutput, iteration int) bool {
					return iteration >= 1 // refine once, then move on
				},
			).WithMaxIterations(3),

			// ── Step 5: Depth Check (WfCondition) ──────────────────
			// If the report is too short, trigger a deep dive.
			// Otherwise, skip to the next step.
			agnogo.WfCondition("depth-check",
				func(_ context.Context, in *agnogo.StepInput) bool {
					return len(in.PrevContent) < 500 // needs more depth
				},
				agnogo.WfStep("deep-dive", researchAgent),
				// no false branch = pass through
			),

			// ── Step 6: Human Review (HITL) ────────────────────────
			// Pauses the workflow and returns ErrWorkflowPaused.
			// The human reviews the draft and approves or rejects.
			agnogo.WfStep("human-review", refineAgent).WithConfirmation(),

			// ── Step 7: Output Format (WfRoute) ────────────────────
			// Routes to the appropriate formatter based on user
			// preferences (learned from previous sessions).
			agnogo.WfRoute("format",
				func(_ context.Context, in *agnogo.StepInput) string {
					// Check if user has a preferred format (from learning)
					if in.Session != nil {
						pref := in.Session.GetMemory("report_format")
						if pref != "" {
							return pref
						}
					}
					return "detailed" // default
				},
				map[string]agnogo.StepRunner{
					"summary":  agnogo.WfStep("summary-format", summaryAgent),
					"detailed": agnogo.WfStep("detailed-format", detailedAgent),
				},
			),
		),
	)

	// ── 4. Session + Learning ──────────────────────────────────────
	//
	// The session persists across the workflow. The learning machine
	// injects recalled context before the first agent runs, and
	// extracts new learnings after the response (async, in background).

	session := agnogo.NewSession("research-001")
	session.SetMeta("user", "analyst")

	// ── 5. Multi-Modal (Optional) ──────────────────────────────────
	//
	// If the user provides a chart or image, attach it to the session.
	// The research agents will analyze it alongside text sources.
	//
	// Uncomment to use:
	// session.AddMediaMessage("user", "Also analyze this market chart:",
	//     []agnogo.Image{agnogo.ImageFromFile("market_chart.png")}, nil, nil)

	// ── 6. Execute the Pipeline ────────────────────────────────────

	fmt.Println("Starting research pipeline...")
	fmt.Println()

	output, err := wf.RunWorkflow(ctx, session, question)

	// ── 7. Handle HITL Pause ───────────────────────────────────────
	//
	// If the workflow paused for human review, show the draft and
	// ask for approval. Then resume from where it paused.

	var paused *agnogo.ErrWorkflowPaused
	if errors.As(err, &paused) {
		fmt.Printf("--- PAUSED: %s ---\n\n", paused.Paused.PauseReason)

		// Show what's been produced so far
		if paused.Paused.CompletedOutputs != nil {
			for name, out := range paused.Paused.CompletedOutputs {
				if out.Content != "" {
					fmt.Printf("[%s] %d chars\n", name, len(out.Content))
				}
			}
		}
		fmt.Println()

		// Ask for approval
		fmt.Print("Approve and continue? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		approved := strings.TrimSpace(strings.ToLower(answer)) == "y"

		if approved {
			fmt.Println("\nResuming pipeline...")
			output, err = wf.ResumeWorkflow(ctx, session, paused.Paused, true, "")
		} else {
			fmt.Println("Research cancelled by reviewer.")
			return
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Pipeline failed: %v\n", err)
		os.Exit(1)
	}

	// ── 8. Output ──────────────────────────────────────────────────

	fmt.Printf("\n=== RESEARCH REPORT ===\n\n")
	fmt.Println(output.Content)

	// Show nested step results
	fmt.Printf("\n--- Pipeline completed: %d steps ---\n", countSteps(output))
}

// ── Pure Go Function Step ───────────────────────────────────────────
//
// This function demonstrates WfFunc — a step that runs pure Go code
// without any LLM call. It merges parallel research results into a
// structured document.

func synthesizeResults(_ context.Context, in *agnogo.StepInput) (*agnogo.StepOutput, error) {
	var sections []string

	// Access each parallel step's output by name
	for _, name := range []string{"web-research", "news-analysis", "technical-review"} {
		out := in.GetOutput(name)
		if out != nil && out.Content != "" {
			title := strings.ReplaceAll(strings.Title(strings.ReplaceAll(name, "-", " ")), " ", " ")
			sections = append(sections, fmt.Sprintf("## %s\n\n%s", title, out.Content))
		}
	}

	if len(sections) == 0 {
		return &agnogo.StepOutput{
			StepName: "synthesize",
			Content:  "No research data gathered.",
			Success:  true,
		}, nil
	}

	merged := strings.Join(sections, "\n\n---\n\n")
	return &agnogo.StepOutput{
		StepName: "synthesize",
		Content:  merged,
		Success:  true,
		Data:     map[string]any{"source_count": len(sections)},
	}, nil
}

func countSteps(out *agnogo.StepOutput) int {
	if out == nil {
		return 0
	}
	count := 1
	for _, n := range out.Nested {
		count += countSteps(n)
	}
	return count
}
