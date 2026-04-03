package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/saeedalam/agnogo"
)

// mock creates a test agent that returns a fixed response.
func mock(response string) *agnogo.Core {
	return agnogo.New(agnogo.Config{
		Model: &fixedModel{response: response},
	})
}

type fixedModel struct{ response string }

func (m *fixedModel) ChatCompletion(_ context.Context, _ []agnogo.Message, _ []map[string]any) (*agnogo.ModelResponse, error) {
	return &agnogo.ModelResponse{Text: m.response}, nil
}

// TestFullPipeline exercises the complete workflow with mock agents.
// No API key needed — purely local.
func TestFullPipeline(t *testing.T) {
	planner := mock("Research plan: 1. Check web 2. Check news 3. Check technical sources")
	researcher := mock("Go offers goroutines for real parallelism. Python has the GIL.")
	editor := mock("Go's goroutine model provides true parallelism, unlike Python's GIL-limited threading.")
	briefWriter := mock("- Go: real parallelism via goroutines\n- Python: GIL limits threading\n- Verdict: Go for performance-critical agents")
	fullWriter := mock("## Detailed Analysis\n\nGo provides goroutines for concurrent execution...")

	pipeline := agnogo.NewWorkflowEngine("test-research",
		agnogo.WfSequence("story",
			agnogo.WfStep("plan", planner),
			agnogo.WfParallel("research",
				agnogo.WfStep("technical", researcher),
				agnogo.WfStep("industry", researcher),
				agnogo.WfStep("community", researcher),
			),
			agnogo.WfFunc("merge", mergeResearch),
			agnogo.WfLoop("polish",
				agnogo.WfStep("edit", editor),
				func(_ *agnogo.StepOutput, i int) bool { return i >= 1 },
			),
			agnogo.WfCondition("quality-gate",
				func(_ context.Context, in *agnogo.StepInput) bool {
					return len(in.PrevContent) < 500
				},
				agnogo.WfStep("go-deeper", researcher),
			),
			// Skip HITL for automated test
			agnogo.WfRoute("deliver",
				func(_ context.Context, _ *agnogo.StepInput) string { return "full" },
				map[string]agnogo.StepRunner{
					"brief": agnogo.WfStep("brief", briefWriter),
					"full":  agnogo.WfStep("full", fullWriter),
				},
			),
		),
	)

	session := agnogo.NewSession("test-pipeline")
	output, err := pipeline.RunWorkflow(context.Background(), session, "Go vs Python for AI agents")
	if err != nil {
		t.Fatal(err)
	}

	// Verify the pipeline produced output
	if output.Content == "" {
		t.Fatal("pipeline produced empty output")
	}
	if !strings.Contains(output.Content, "Analysis") {
		t.Errorf("expected detailed format output, got: %s", output.Content[:min(100, len(output.Content))])
	}

	// Verify all steps ran
	stepCount := countSteps(output)
	if stepCount < 5 {
		t.Errorf("expected at least 5 steps, got %d", stepCount)
	}
}

// TestPipelineWithHITL verifies the pause/resume flow.
func TestPipelineWithHITL(t *testing.T) {
	agent := mock("draft report content here")

	pipeline := agnogo.NewWorkflowEngine("hitl-test",
		agnogo.WfSequence("main",
			agnogo.WfStep("draft", agent),
			agnogo.WfStep("review", agent).WithConfirmation(),
			agnogo.WfStep("final", mock("final output")),
		),
	)

	session := agnogo.NewSession("hitl-test")

	// First run — should pause at review
	_, err := pipeline.RunWorkflow(context.Background(), session, "test")
	var paused *agnogo.ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected pause, got: %v", err)
	}
	if paused.Paused.PausedAt != "review" {
		t.Errorf("paused at %q, expected %q", paused.Paused.PausedAt, "review")
	}

	// Resume with approval
	output, err := pipeline.ResumeWorkflow(context.Background(), session, paused.Paused, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "final output" {
		t.Errorf("expected 'final output', got %q", output.Content)
	}
}

// TestMergeResearch verifies the pure Go merge function.
func TestMergeResearch(t *testing.T) {
	input := &agnogo.StepInput{
		PrevOutputs: map[string]*agnogo.StepOutput{
			"technical": {StepName: "technical", Content: "Go has goroutines."},
			"industry":  {StepName: "industry", Content: "Market is growing."},
			"community": {StepName: "community", Content: "Developers love Go."},
		},
	}

	output, err := mergeResearch(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.Content, "Technical Analysis") {
		t.Error("missing Technical Analysis section")
	}
	if !strings.Contains(output.Content, "Industry Landscape") {
		t.Error("missing Industry Landscape section")
	}
	if !strings.Contains(output.Content, "Community Perspective") {
		t.Error("missing Community Perspective section")
	}
	if !strings.Contains(output.Content, "goroutines") {
		t.Error("missing content from technical source")
	}
}

// TestPickFormat verifies format selection from memory.
func TestPickFormat(t *testing.T) {
	// No preference set — should default to "full"
	session := agnogo.NewSession("test")
	input := &agnogo.StepInput{Session: session}
	if got := pickFormat(context.Background(), input); got != "full" {
		t.Errorf("default format = %q, want 'full'", got)
	}

	// Set preference — should use it
	session.SetMemory("report_format", "brief")
	if got := pickFormat(context.Background(), input); got != "brief" {
		t.Errorf("with preference = %q, want 'brief'", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
