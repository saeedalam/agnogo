package agnogo

import (
	"context"
	"fmt"
	"log/slog"
)

// ── Workflow Agents ──────────────────────────────────────
// Inspired by Google ADK: Sequential, Parallel, and Loop agents.
// Compose agents into complex workflows.

// SequentialWorkflow runs agents in order. Each agent sees the previous output.
//
//	wf := agnogo.Sequential(
//	    agnogo.Step("extract", extractAgent),
//	    agnogo.Step("validate", validateAgent),
//	    agnogo.Step("book", bookAgent),
//	)
//	resp, _ := wf.Run(ctx, session, "Book a haircut tomorrow")
type SequentialWorkflow struct {
	steps []WorkflowStep
}

// WorkflowStep is a named agent in a workflow.
type WorkflowStep struct {
	Name  string
	Agent *Agent
}

// Step creates a named workflow step.
func Step(name string, agent *Agent) WorkflowStep {
	return WorkflowStep{Name: name, Agent: agent}
}

// Sequential creates a workflow that runs agents in order.
func Sequential(steps ...WorkflowStep) *SequentialWorkflow {
	return &SequentialWorkflow{steps: steps}
}

func (w *SequentialWorkflow) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	currentInput := input
	var allTools []string

	for _, step := range w.steps {
		slog.Debug("agnogo: workflow step", "step", step.Name, "input_len", len(currentInput))
		resp, err := step.Agent.Run(ctx, session, currentInput)
		if err != nil {
			return nil, fmt.Errorf("step %s failed: %w", step.Name, err)
		}
		allTools = append(allTools, resp.ToolsCalled...)
		currentInput = resp.Text // output of one step becomes input of next
		session.Set("_workflow_step", step.Name)

		// If a step needs approval, pause the workflow
		if resp.NeedsApproval {
			session.Set("_workflow_paused_at", step.Name)
			return resp, nil
		}
	}

	return &Response{Text: currentInput, ToolsCalled: allTools}, nil
}

// ParallelWorkflow runs agents concurrently and merges results.
//
//	wf := agnogo.Parallel(
//	    agnogo.Step("weather", weatherAgent),
//	    agnogo.Step("news", newsAgent),
//	    agnogo.Step("calendar", calendarAgent),
//	)
//	resp, _ := wf.Run(ctx, session, "Give me my morning briefing")
type ParallelWorkflow struct {
	steps      []WorkflowStep
	mergeFunc  func(results map[string]string) string
}

// Parallel creates a workflow that runs agents concurrently.
func Parallel(steps ...WorkflowStep) *ParallelWorkflow {
	return &ParallelWorkflow{
		steps: steps,
		mergeFunc: func(results map[string]string) string {
			var out string
			for name, result := range results {
				out += fmt.Sprintf("## %s\n%s\n\n", name, result)
			}
			return out
		},
	}
}

// WithMerge sets a custom function to merge parallel results.
func (w *ParallelWorkflow) WithMerge(fn func(results map[string]string) string) *ParallelWorkflow {
	w.mergeFunc = fn
	return w
}

func (w *ParallelWorkflow) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	type stepResult struct {
		name string
		resp *Response
		err  error
	}

	ch := make(chan stepResult, len(w.steps))
	for _, step := range w.steps {
		go func(s WorkflowStep) {
			// Each parallel agent gets its own session copy to avoid races
			parallelSession := NewSession(session.ID + "_" + s.Name)
			parallelSession.Memory = session.Memory
			parallelSession.Metadata = session.Metadata
			resp, err := s.Agent.Run(ctx, parallelSession, input)
			ch <- stepResult{name: s.Name, resp: resp, err: err}
		}(step)
	}

	results := make(map[string]string)
	var allTools []string
	for range w.steps {
		r := <-ch
		if r.err != nil {
			results[r.name] = fmt.Sprintf("Error: %s", r.err)
		} else {
			results[r.name] = r.resp.Text
			allTools = append(allTools, r.resp.ToolsCalled...)
		}
	}

	merged := w.mergeFunc(results)
	return &Response{Text: merged, ToolsCalled: allTools}, nil
}

// LoopWorkflow runs an agent repeatedly until a condition is met.
//
//	wf := agnogo.Loop(refinementAgent, func(resp *agnogo.Response, iteration int) bool {
//	    return strings.Contains(resp.Text, "DONE") || iteration >= 5
//	})
type LoopWorkflow struct {
	agent     *Agent
	condition func(resp *Response, iteration int) bool
	maxIter   int
}

// Loop creates a workflow that repeats until condition returns true.
func Loop(agent *Agent, stopWhen func(resp *Response, iteration int) bool) *LoopWorkflow {
	return &LoopWorkflow{agent: agent, condition: stopWhen, maxIter: 10}
}

// WithMaxIterations sets the maximum loop count (default 10).
func (w *LoopWorkflow) WithMaxIterations(n int) *LoopWorkflow {
	w.maxIter = n
	return w
}

func (w *LoopWorkflow) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	currentInput := input
	var lastResp *Response

	for i := 0; i < w.maxIter; i++ {
		slog.Debug("agnogo: loop iteration", "i", i, "input_len", len(currentInput))
		resp, err := w.agent.Run(ctx, session, currentInput)
		if err != nil {
			return nil, fmt.Errorf("loop iteration %d: %w", i, err)
		}
		lastResp = resp

		if w.condition(resp, i) {
			break
		}
		currentInput = resp.Text
	}

	return lastResp, nil
}

// ── Condition Workflow ───────────────────────────────────
// Matches Agno's Condition: evaluate → true branch or false branch.
//
//	wf := agnogo.Condition(
//	    func(ctx context.Context, input string) bool { return strings.Contains(input, "urgent") },
//	    agnogo.Sequential(urgentStep1, urgentStep2), // true branch
//	    agnogo.Sequential(normalStep1),              // false branch (optional)
//	)

// Workflow is the interface all workflow types implement.
type Workflow interface {
	Run(ctx context.Context, session *Session, input string) (*Response, error)
}

// ConditionWorkflow executes different branches based on a condition.
type ConditionWorkflow struct {
	evaluator func(ctx context.Context, input string) bool
	trueBranch  Workflow
	falseBranch Workflow // optional
}

// Condition creates a conditional workflow.
func Condition(eval func(ctx context.Context, input string) bool, trueBranch Workflow, falseBranch ...Workflow) *ConditionWorkflow {
	var fb Workflow
	if len(falseBranch) > 0 {
		fb = falseBranch[0]
	}
	return &ConditionWorkflow{evaluator: eval, trueBranch: trueBranch, falseBranch: fb}
}

func (w *ConditionWorkflow) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	if w.evaluator(ctx, input) {
		slog.Debug("agnogo: condition=true, executing true branch")
		return w.trueBranch.Run(ctx, session, input)
	}
	if w.falseBranch != nil {
		slog.Debug("agnogo: condition=false, executing false branch")
		return w.falseBranch.Run(ctx, session, input)
	}
	slog.Debug("agnogo: condition=false, no false branch, skipping")
	return &Response{Text: input}, nil // pass through
}

// ── Router Workflow ──────────────────────────────────────
// Matches Agno's Router: select a path from multiple choices.
//
//	wf := agnogo.Route(
//	    func(ctx context.Context, input string) string {
//	        if strings.Contains(input, "book") { return "booking" }
//	        return "support"
//	    },
//	    map[string]Workflow{
//	        "booking": bookingWorkflow,
//	        "support": supportWorkflow,
//	    },
//	)

// RouterWorkflow selects one workflow from a map based on a selector function.
type RouterWorkflow struct {
	selector func(ctx context.Context, input string) string
	routes   map[string]Workflow
	fallback string
}

// Route creates a router workflow.
func Route(selector func(ctx context.Context, input string) string, routes map[string]Workflow) *RouterWorkflow {
	var fallback string
	for name := range routes {
		fallback = name
		break
	}
	return &RouterWorkflow{selector: selector, routes: routes, fallback: fallback}
}

// WithFallback sets the default route name if selector returns unknown.
func (w *RouterWorkflow) WithFallback(name string) *RouterWorkflow {
	w.fallback = name
	return w
}

func (w *RouterWorkflow) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	selected := w.selector(ctx, input)
	slog.Debug("agnogo: router selected", "route", selected)

	wf, ok := w.routes[selected]
	if !ok {
		wf = w.routes[w.fallback]
		if wf == nil {
			return &Response{Text: fmt.Sprintf("No route found for: %s", selected)}, nil
		}
	}
	session.Set("_routed_to", selected)
	return wf.Run(ctx, session, input)
}
