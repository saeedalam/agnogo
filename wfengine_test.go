package agnogo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testAgent creates a mock agent that returns a fixed response.
func testAgent(response string) *Core {
	return New(Config{Model: &mockModel{responses: []ModelResponse{{Text: response}}}})
}

// testAgentMulti creates a mock agent with multiple responses (one per call).
func testAgentMulti(responses ...string) *Core {
	mrs := make([]ModelResponse, len(responses))
	for i, r := range responses {
		mrs[i] = ModelResponse{Text: r}
	}
	return New(Config{Model: &mockModel{responses: mrs}})
}

// ── Sequential Data Flow ────────────────────────────────────────────

func TestStepsSequentialDataFlow(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("step_a", testAgent("output-A")),
			WfStep("step_b", testAgent("output-B")),
			WfStep("step_c", testAgent("output-C")),
		),
	)

	session := NewSession("seq-flow")
	output, err := wf.RunWorkflow(ctx(), session, "start")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "output-C" {
		t.Errorf("content = %q, want %q", output.Content, "output-C")
	}
	if len(output.Nested) != 3 {
		t.Errorf("nested = %d, want 3", len(output.Nested))
	}
	// Verify each nested step
	for i, name := range []string{"step_a", "step_b", "step_c"} {
		if output.Nested[i].StepName != name {
			t.Errorf("nested[%d].StepName = %q, want %q", i, output.Nested[i].StepName, name)
		}
	}
}

// ── FuncStep ────────────────────────────────────────────────────────

func TestStepsFuncNode(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("extract", testAgent("raw-data")),
			WfFunc("transform", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				prev := input.GetOutput("extract")
				if prev == nil {
					return nil, fmt.Errorf("extract output not found")
				}
				return &StepOutput{
					Content: "transformed:" + prev.Content,
					Success: true,
				}, nil
			}),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("func"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "transformed:raw-data" {
		t.Errorf("content = %q, want %q", output.Content, "transformed:raw-data")
	}
}

// ── OnError Modes ───────────────────────────────────────────────────

func TestStepsOnErrorFail(t *testing.T) {
	failingFn := func(ctx context.Context, input *StepInput) (*StepOutput, error) {
		return nil, fmt.Errorf("step exploded")
	}

	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfFunc("failing", failingFn),
			WfStep("never", testAgent("should not run")),
		),
	)

	_, err := wf.RunWorkflow(ctx(), NewSession("fail"), "go")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "step exploded") {
		t.Errorf("error = %q, want to contain 'step exploded'", err.Error())
	}
}

func TestStepsOnErrorSkip(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfFunc("failing", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				return nil, fmt.Errorf("boom")
			}).WithOnError(OnErrorSkip),
			WfStep("after", testAgent("continued")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("skip"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "continued" {
		t.Errorf("content = %q, want %q", output.Content, "continued")
	}
	// The failed step should appear in nested with Success=false
	if len(output.Nested) != 2 {
		t.Fatalf("nested = %d, want 2", len(output.Nested))
	}
	if output.Nested[0].Success {
		t.Error("first step should have Success=false")
	}
}

func TestStepsOnErrorPause(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfFunc("failing", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				return nil, fmt.Errorf("needs human")
			}).WithOnError(OnErrorPause),
		),
	)

	_, err := wf.RunWorkflow(ctx(), NewSession("pause"), "go")
	if err == nil {
		t.Fatal("expected ErrWorkflowPaused")
	}
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected ErrWorkflowPaused, got %T: %v", err, err)
	}
	if paused.Paused.PausedAt != "failing" {
		t.Errorf("paused at = %q, want %q", paused.Paused.PausedAt, "failing")
	}
}

// ── Retry ───────────────────────────────────────────────────────────

func TestStepsRetry(t *testing.T) {
	var attempts atomic.Int32

	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfFunc("flaky", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				n := attempts.Add(1)
				if n < 3 {
					return nil, fmt.Errorf("attempt %d failed", n)
				}
				return &StepOutput{Content: "success-on-3", Success: true}, nil
			}).WithRetries(2), // 3 total attempts
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("retry"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "success-on-3" {
		t.Errorf("content = %q, want %q", output.Content, "success-on-3")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

// ── HITL Confirmation ───────────────────────────────────────────────

func TestStepsRequiresConfirmation(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("safe", testAgent("safe-output")),
			WfStep("dangerous", testAgent("should-not-run")).WithConfirmation(),
		),
	)

	_, err := wf.RunWorkflow(ctx(), NewSession("confirm"), "go")
	if err == nil {
		t.Fatal("expected ErrWorkflowPaused")
	}
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected ErrWorkflowPaused, got %T", err)
	}
	if paused.Paused.PausedAt != "dangerous" {
		t.Errorf("paused at = %q, want %q", paused.Paused.PausedAt, "dangerous")
	}
	// Verify completed outputs are preserved
	if paused.Paused.CompletedOutputs == nil {
		t.Fatal("completed outputs should not be nil")
	}
	if _, ok := paused.Paused.CompletedOutputs["safe"]; !ok {
		t.Error("completed outputs should include 'safe' step")
	}
}

func TestStepsResumeAfterPause(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("step1", testAgent("first")),
			WfStep("step2", testAgent("second")).WithConfirmation(),
		),
	)

	session := NewSession("resume")

	// First run — should pause at step2
	_, err := wf.RunWorkflow(ctx(), session, "go")
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected pause, got %v", err)
	}

	// Resume with approval
	output, err := wf.ResumeWorkflow(ctx(), session, paused.Paused, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "second" {
		t.Errorf("content = %q, want %q", output.Content, "second")
	}
}

// ── Stop Flag ───────────────────────────────────────────────────────

func TestStepsStopFlag(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfFunc("stopper", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				return &StepOutput{Content: "stopped-here", Success: true, Stop: true}, nil
			}),
			WfStep("unreachable", testAgent("should not run")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("stop"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "stopped-here" {
		t.Errorf("content = %q, want %q", output.Content, "stopped-here")
	}
	if len(output.Nested) != 1 {
		t.Errorf("nested = %d, want 1 (second step should not have run)", len(output.Nested))
	}
}

// ── Parallel Steps ──────────────────────────────────────────────────

func TestParallelSteps(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfParallel("all",
			WfStep("weather", testAgent("sunny")),
			WfStep("news", testAgent("headlines")),
			WfStep("calendar", testAgent("meetings")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("parallel"), "briefing")
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Nested) != 3 {
		t.Errorf("nested = %d, want 3", len(output.Nested))
	}
	// Content should contain all three outputs
	if !strings.Contains(output.Content, "sunny") {
		t.Error("missing weather output")
	}
	if !strings.Contains(output.Content, "headlines") {
		t.Error("missing news output")
	}
	if !strings.Contains(output.Content, "meetings") {
		t.Error("missing calendar output")
	}
}

func TestParallelStepsConcurrency(t *testing.T) {
	// Each step sleeps 100ms. Sequential = 300ms. Parallel = ~100ms.
	makeSleepStep := func(name string) StepRunner {
		return WfFunc(name, func(ctx context.Context, input *StepInput) (*StepOutput, error) {
			time.Sleep(100 * time.Millisecond)
			return &StepOutput{Content: name + "-done", Success: true}, nil
		})
	}

	wf := NewWorkflowEngine("test",
		WfParallel("fast",
			makeSleepStep("a"),
			makeSleepStep("b"),
			makeSleepStep("c"),
		),
	)

	start := time.Now()
	_, err := wf.RunWorkflow(ctx(), NewSession("par-speed"), "go")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatal(err)
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("elapsed %v — steps may not be running concurrently", elapsed)
	}
}

// ── Loop Step ───────────────────────────────────────────────────────

func TestLoopStep(t *testing.T) {
	var iteration atomic.Int32

	body := WfFunc("refine", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
		n := iteration.Add(1)
		return &StepOutput{Content: fmt.Sprintf("iteration-%d", n), Success: true}, nil
	})

	wf := NewWorkflowEngine("test",
		WfLoop("loop", body, func(out *StepOutput, i int) bool {
			return i >= 2 // stop after 3 iterations (0, 1, 2)
		}).WithMaxIterations(10),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("loop"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "iteration-3" {
		t.Errorf("content = %q, want %q", output.Content, "iteration-3")
	}
	if len(output.Nested) != 3 {
		t.Errorf("nested = %d, want 3", len(output.Nested))
	}
}

func TestLoopStepMaxIterations(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfLoop("infinite", WfFunc("body", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "again", Success: true}, nil
		}), func(out *StepOutput, i int) bool {
			return false // never stop
		}).WithMaxIterations(3),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("max"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Nested) != 3 {
		t.Errorf("nested = %d, want 3 (max iterations)", len(output.Nested))
	}
}

// ── Condition Step ──────────────────────────────────────────────────

func TestConditionStepTrue(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfCondition("check",
			func(ctx context.Context, input *StepInput) bool {
				return strings.Contains(input.Input, "urgent")
			},
			WfStep("rush", testAgent("rush-processed")),
			WfStep("normal", testAgent("normal-processed")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("cond-true"), "urgent order")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "rush-processed" {
		t.Errorf("content = %q, want %q", output.Content, "rush-processed")
	}
}

func TestConditionStepFalse(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfCondition("check",
			func(ctx context.Context, input *StepInput) bool {
				return strings.Contains(input.Input, "urgent")
			},
			WfStep("rush", testAgent("rush-processed")),
			WfStep("normal", testAgent("normal-processed")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("cond-false"), "regular order")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "normal-processed" {
		t.Errorf("content = %q, want %q", output.Content, "normal-processed")
	}
}

func TestConditionStepNoFalseBranch(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfCondition("check",
			func(ctx context.Context, input *StepInput) bool { return false },
			WfStep("only-true", testAgent("should not run")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("no-false"), "hello")
	if err != nil {
		t.Fatal(err)
	}
	// Should pass through input unchanged
	if output.Content != "hello" {
		t.Errorf("content = %q, want %q (pass-through)", output.Content, "hello")
	}
}

// ── Router Step ─────────────────────────────────────────────────────

func TestRouterStep(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfRoute("dispatch",
			func(ctx context.Context, input *StepInput) string {
				if strings.Contains(input.Input, "refund") {
					return "refund"
				}
				return "support"
			},
			map[string]StepRunner{
				"refund":  WfStep("refund-agent", testAgent("refund-handled")),
				"support": WfStep("support-agent", testAgent("support-handled")),
			},
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("route"), "I want a refund")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "refund-handled" {
		t.Errorf("content = %q, want %q", output.Content, "refund-handled")
	}
}

func TestRouterStepFallback(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfRoute("dispatch",
			func(ctx context.Context, input *StepInput) string { return "unknown" },
			map[string]StepRunner{
				"a": WfStep("a", testAgent("a-result")),
				"b": WfStep("b", testAgent("b-result")),
			},
		).WithFallback("a"),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("fallback"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "a-result" {
		t.Errorf("content = %q, want %q", output.Content, "a-result")
	}
}

// ── Nested Composition ──────────────────────────────────────────────

func TestNestedComposition(t *testing.T) {
	// Sequential(Parallel(A, B), Condition(true→C, false→D))
	wf := NewWorkflowEngine("test",
		WfSequence("pipeline",
			WfParallel("gather",
				WfStep("research", testAgent("research-data")),
				WfStep("analyze", testAgent("analysis-data")),
			),
			WfCondition("decide",
				func(ctx context.Context, input *StepInput) bool {
					return strings.Contains(input.PrevContent, "research")
				},
				WfStep("approve", testAgent("approved")),
				WfStep("reject", testAgent("rejected")),
			),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("nested"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "approved" {
		t.Errorf("content = %q, want %q", output.Content, "approved")
	}
}

// ── WorkflowAdapter ─────────────────────────────────────────────────

func TestWorkflowAdapter(t *testing.T) {
	// Wrap an existing SequentialWorkflow inside the new engine
	oldWf := Sequential(
		Step("old_a", testAgent("old-A")),
		Step("old_b", testAgent("old-B")),
	)

	wf := NewWorkflowEngine("test",
		WfSequence("main",
			AdaptWorkflow("legacy", oldWf),
			WfStep("new_step", testAgent("new-output")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("adapter"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "new-output" {
		t.Errorf("content = %q, want %q", output.Content, "new-output")
	}
}

// ── WorkflowEngine implements Workflow ──────────────────────────────

func TestWorkflowEngineImplementsWorkflow(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfStep("simple", testAgent("hello")),
	)

	// Use via Workflow interface
	var w Workflow = wf
	resp, err := w.Run(ctx(), NewSession("compat"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello" {
		t.Errorf("text = %q, want %q", resp.Text, "hello")
	}
}

func TestWorkflowEngineHITLViaWorkflowInterface(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfStep("confirm", testAgent("result")).WithConfirmation(),
	)

	var w Workflow = wf
	resp, err := w.Run(ctx(), NewSession("hitl-compat"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.NeedsApproval {
		t.Error("expected NeedsApproval=true via Workflow interface")
	}
}

// ── StepInput.GetOutput recursive search ────────────────────────────

func TestStepInputGetOutputNested(t *testing.T) {
	input := &StepInput{
		PrevOutputs: map[string]*StepOutput{
			"parallel": {
				StepName: "parallel",
				Nested: []*StepOutput{
					{StepName: "research", Content: "found-it"},
					{StepName: "analyze", Content: "analyzed"},
				},
			},
		},
	}

	// Direct lookup
	if out := input.GetOutput("parallel"); out == nil {
		t.Error("direct lookup for 'parallel' should succeed")
	}

	// Nested lookup
	if out := input.GetOutput("research"); out == nil {
		t.Error("nested lookup for 'research' should succeed")
	} else if out.Content != "found-it" {
		t.Errorf("content = %q, want %q", out.Content, "found-it")
	}

	// Missing
	if out := input.GetOutput("nonexistent"); out != nil {
		t.Error("lookup for nonexistent should return nil")
	}
}

// ── Context cancellation ────────────────────────────────────────────

func TestStepsContextCancellation(t *testing.T) {
	c, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("a", testAgent("result")),
		),
	)

	_, err := wf.RunWorkflow(c, NewSession("cancel"), "go")
	if err == nil {
		t.Fatal("expected context error")
	}
}

// ── SkipIf ──────────────────────────────────────────────────────────

func TestAgentStepSkipIf(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("skippable", testAgent("should-skip")).WithSkipIf(func(ctx context.Context, input *StepInput) bool {
				return true // always skip
			}),
			WfStep("next", testAgent("reached")),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("skip-if"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "reached" {
		t.Errorf("content = %q, want %q", output.Content, "reached")
	}
}

// ── Parallel Data isolation ──────────────────────────────────────────

func TestParallelStepsDataIsolation(t *testing.T) {
	// Each parallel step writes to Data — must not race
	wf := NewWorkflowEngine("test",
		WfParallel("writers",
			WfFunc("writer_a", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				if input.Data != nil {
					input.Data["writer_a"] = "value_a"
				}
				time.Sleep(10 * time.Millisecond)
				return &StepOutput{Content: "a-done", Success: true}, nil
			}),
			WfFunc("writer_b", func(ctx context.Context, input *StepInput) (*StepOutput, error) {
				if input.Data != nil {
					input.Data["writer_b"] = "value_b"
				}
				time.Sleep(10 * time.Millisecond)
				return &StepOutput{Content: "b-done", Success: true}, nil
			}),
		),
	)

	output, err := wf.RunWorkflow(ctx(), NewSession("data-iso"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Nested) != 2 {
		t.Errorf("nested = %d, want 2", len(output.Nested))
	}
}

// ── Resume denied ───────────────────────────────────────────────────

func TestStepsResumeDenied(t *testing.T) {
	wf := NewWorkflowEngine("test",
		WfSequence("main",
			WfStep("step1", testAgent("first")),
			WfStep("step2", testAgent("should-not-run")).WithConfirmation(),
		),
	)

	session := NewSession("deny")
	_, err := wf.RunWorkflow(ctx(), session, "go")
	var paused *ErrWorkflowPaused
	if !errors.As(err, &paused) {
		t.Fatalf("expected pause, got %v", err)
	}

	// Resume with denial
	output, err := wf.ResumeWorkflow(ctx(), session, paused.Paused, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "Step was not approved." {
		t.Errorf("content = %q, want denial message", output.Content)
	}
}

// ── Helper ──────────────────────────────────────────────────────────

func ctx() context.Context { return context.Background() }
