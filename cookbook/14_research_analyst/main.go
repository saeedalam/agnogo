// AI Research Analyst
//
// An agent that researches any topic and writes a report.
// Start simple, then watch it grow.
//
//   go run main.go "What makes Go better than Python for AI agents?"

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

func main() {
	ctx := context.Background()

	// What should we research?
	question := "What makes Go better than Python for building AI agents?"
	if len(os.Args) > 1 {
		question = strings.Join(os.Args[1:], " ")
	}

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║        AI Research Analyst               ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Question:", question)
	fmt.Println()

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 1: The Agents
	// ─────────────────────────────────────────────────────────────
	//
	// Every good research team has specialists. Ours has four:
	// a planner who thinks before acting, researchers who dig for
	// facts, an editor who polishes, and a formatter who delivers.

	// The planner thinks step-by-step before diving in.
	// If your model supports native reasoning (O1, Claude thinking),
	// agnogo auto-detects it. Otherwise, it uses chain-of-thought.
	planner := agnogo.Agent(
		`You are a senior research strategist. Before researching anything,
		 you create a battle plan: what sources to check, what questions
		 to answer, what structure the final report should have.`,
		agnogo.WithReasoningConfig(agnogo.ReasoningConfig{
			Enabled:  true,
			Mode:     agnogo.ReasoningAuto,
			MinSteps: 2,
			MaxSteps: 4,
		}),
	)

	// The researcher hunts for facts. Reliable() means:
	// - hallucination guard catches fabricated dates/prices/stats
	// - confidence scoring flags uncertain answers
	// - cost budget prevents runaway API spending
	researcher := agnogo.Agent(
		`You are a research analyst. Given a research topic, write a thorough
		 analysis with facts and evidence. Output ONLY the analysis — no
		 meta-commentary. If you don't know something, say so.`,
		agnogo.Reliable(),
		agnogo.WithBudget(agnogo.CostBudget{
			MaxPerRun: 1.00, // $1 max per research call
		}),
	)

	// The editor improves whatever they're given.
	editor := agnogo.Agent(
		`You are an editor. You receive a draft report and output an improved
		 version. Fix errors, fill gaps, sharpen arguments. Output ONLY the
		 improved report — no commentary about the changes you made.`,
	)

	// Two formatters — the router picks one based on user preference.
	briefWriter := agnogo.Agent(
		`You write executive briefs. 5 bullet points, 3 recommendations,
		 one page max. Busy people love you.`,
	)
	fullWriter := agnogo.Agent(
		`You are a report writer. You receive raw research data and rewrite
		 it as a polished, detailed analytical report. Output ONLY the final
		 report — no commentary, no meta-text, no "here is your report".
		 Use markdown: ## sections, bullet points, comparisons, evidence.`,
	)

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 2: The Pipeline
	// ─────────────────────────────────────────────────────────────
	//
	// The pipeline tells our agents WHEN and HOW to work together.
	// Read it top to bottom — it's the story of a research project.

	pipeline := agnogo.NewWorkflowEngine("research",
		agnogo.WfSequence("story",

			// Act 1: Think before you act.
			// The planner uses reasoning to map out a strategy.
			// This produces 2-4 structured thinking steps before
			// any research begins.
			agnogo.WfStep("plan", planner),

			// Act 2: Cast a wide net.
			// Three researchers work THE SAME question simultaneously.
			// One looks at it from a technical angle, one from the
			// industry angle, one from the community angle.
			// They run in parallel — 3x faster than sequential.
			agnogo.WfParallel("research",
				agnogo.WfStep("technical", researcher),
				agnogo.WfStep("industry", researcher),
				agnogo.WfStep("community", researcher),
			),

			// Act 3: Connect the dots.
			// A pure Go function — no LLM call, no cost, no latency.
			// It merges the three research streams into one document.
			agnogo.WfFunc("merge", mergeResearch),

			// Act 4: Polish it.
			// The editor makes one pass to improve quality.
			// We could loop more, but diminishing returns.
			agnogo.WfLoop("polish",
				agnogo.WfStep("edit", editor),
				func(_ *agnogo.StepOutput, i int) bool { return i >= 1 },
			),

			// Act 5: Is it good enough?
			// If the report is thin (< 500 chars), go deeper.
			// Otherwise, move on. Simple quality gate.
			agnogo.WfCondition("quality-gate",
				func(_ context.Context, in *agnogo.StepInput) bool {
					return len(in.PrevContent) < 500
				},
				agnogo.WfStep("go-deeper", researcher),
			),

			// Act 6: Human in the loop.
			// The pipeline PAUSES here. You review the draft.
			// If you approve, it continues. If not, it stops.
			// This is how you keep humans in control.
			agnogo.WfStep("review", editor).WithConfirmation(),

			// Act 7: Deliver in your preferred format.
			// The router checks if you've told us your preference.
			// First run: defaults to "full". After that: remembers.
			agnogo.WfRoute("deliver",
				pickFormat,
				map[string]agnogo.StepRunner{
					"brief": agnogo.WfStep("brief", briefWriter),
					"full":  agnogo.WfStep("full", fullWriter),
				},
			),
		),
	)

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 3: Memory
	// ─────────────────────────────────────────────────────────────
	//
	// First conversation: the agent knows nothing about you.
	// Second conversation: it remembers your name, your role,
	// what you researched last time, and what format you prefer.
	// That's the Learning Machine.

	memory := agnogo.NewLearningMachine(nil)
	memory.AddStore(agnogo.NewUserProfileStore())     // who you are
	memory.AddStore(agnogo.NewSessionContextStore())   // what happened
	memory.AddStore(agnogo.NewEntityMemoryStore())     // what you researched

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 4: Go
	// ─────────────────────────────────────────────────────────────

	session := agnogo.NewSession("research-" + fmt.Sprintf("%d", time.Now().Unix()))

	// Optional: attach an image for visual analysis.
	// session.AddMediaMessage("user", "Analyze this chart too:",
	//     []agnogo.Image{agnogo.ImageFromFile("chart.png")}, nil, nil)

	fmt.Println("Running pipeline...")
	fmt.Println()
	start := time.Now()

	output, err := pipeline.RunWorkflow(ctx, session, question)

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 5: The Human Gate
	// ─────────────────────────────────────────────────────────────
	//
	// The pipeline paused at "review". Let's handle it.

	var paused *agnogo.ErrWorkflowPaused
	if errors.As(err, &paused) {
		fmt.Println("────────────────────────────────────────")
		fmt.Println("REVIEW NEEDED")
		fmt.Println("────────────────────────────────────────")
		fmt.Println()

		// Show a preview of what each step produced
		if paused.Paused.CompletedOutputs != nil {
			for name, out := range paused.Paused.CompletedOutputs {
				if out != nil && out.Content != "" {
					preview := out.Content
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					fmt.Printf("[%s]\n%s\n\n", name, preview)
				}
			}
		}

		fmt.Print("Approve? (y/n) ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')

		if strings.TrimSpace(strings.ToLower(answer)) == "y" {
			output, err = pipeline.ResumeWorkflow(ctx, session, paused.Paused, true, "")
		} else {
			fmt.Println("Cancelled.")
			return
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %v\n", err)
		os.Exit(1)
	}

	// ─────────────────────────────────────────────────────────────
	// CHAPTER 6: The Report
	// ─────────────────────────────────────────────────────────────

	elapsed := time.Since(start)
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("RESEARCH REPORT")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()
	fmt.Println(output.Content)
	fmt.Println()
	fmt.Printf("Completed in %s (%d steps)\n", elapsed.Round(time.Millisecond), countSteps(output))

	// Show reasoning steps if any
	// (These come from the planner agent's chain-of-thought)
	if output.Response != nil && len(output.Response.ReasoningSteps) > 0 {
		fmt.Println()
		fmt.Println("Reasoning trace:")
		for _, step := range output.Response.ReasoningSteps {
			fmt.Printf("  [%.0f%%] %s\n", step.Confidence*100, step.Title)
		}
	}

	// The learning machine has already extracted:
	// - Your profile (if you mentioned your name/role)
	// - Session context (summary of what was researched)
	// - Entity memories (facts about topics discussed)
	// Next time you run this, it'll remember.
	_ = memory
}

// ─────────────────────────────────────────────────────────────────────
// Supporting functions
// ─────────────────────────────────────────────────────────────────────

// mergeResearch combines parallel research streams into one document.
// This is a WfFunc — pure Go, no LLM call, no cost.
func mergeResearch(_ context.Context, in *agnogo.StepInput) (*agnogo.StepOutput, error) {
	sources := []struct{ key, title string }{
		{"technical", "Technical Analysis"},
		{"industry", "Industry Landscape"},
		{"community", "Community Perspective"},
	}

	var parts []string
	for _, s := range sources {
		out := in.GetOutput(s.key)
		if out != nil && out.Content != "" {
			parts = append(parts, fmt.Sprintf("## %s\n\n%s", s.title, out.Content))
		}
	}

	if len(parts) == 0 {
		return &agnogo.StepOutput{Content: "No data gathered.", Success: true}, nil
	}

	return &agnogo.StepOutput{
		Content: strings.Join(parts, "\n\n---\n\n"),
		Success: true,
	}, nil
}

// pickFormat checks if the user has a preferred report format.
// First run: defaults to "full". The learning machine remembers
// for next time.
func pickFormat(_ context.Context, in *agnogo.StepInput) string {
	if in.Session != nil {
		if pref := in.Session.GetMemory("report_format"); pref != "" {
			return pref
		}
	}
	return "full"
}

func countSteps(out *agnogo.StepOutput) int {
	if out == nil {
		return 0
	}
	n := 1
	for _, child := range out.Nested {
		n += countSteps(child)
	}
	return n
}
