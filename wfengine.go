package agnogo

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ── Workflow Engine v2 ───────────────────────────────────────────────
//
// A structured workflow engine with rich data flow, error handling modes,
// HITL pause/resume, and composable step nesting. This is a superset of
// the existing Workflow interface — all existing workflow types continue
// to work unchanged.
//
// Quick example:
//
//	wf := agnogo.NewWorkflowEngine("pipeline",
//	    agnogo.WfSequence("main",
//	        agnogo.WfStep("extract", extractAgent),
//	        agnogo.WfFunc("validate", validateFn),
//	        agnogo.WfStep("process", processAgent),
//	    ),
//	)
//	output, err := wf.RunWorkflow(ctx, session, "Process order #123")

// ── Data flow types ──────────────────────────────────────────────────

// StepInput carries data into a step. Built fresh for each step by the
// parent composition (Steps, ParallelSteps, etc.).
type StepInput struct {
	Input       string                 // primary input text
	PrevContent string                 // output of the immediately previous step
	PrevOutputs map[string]*StepOutput // all previous step outputs, keyed by step name
	Data        map[string]any         // additional data flowing through the workflow
	Session     *Session               // shared workflow session
}

// GetOutput retrieves a previous step's output by name. Returns nil if not found.
// Searches nested outputs recursively for compound steps.
func (si *StepInput) GetOutput(stepName string) *StepOutput {
	if si.PrevOutputs == nil {
		return nil
	}
	if out, ok := si.PrevOutputs[stepName]; ok {
		return out
	}
	// Recursive search in nested outputs
	for _, out := range si.PrevOutputs {
		if found := searchNested(out, stepName); found != nil {
			return found
		}
	}
	return nil
}

func searchNested(out *StepOutput, name string) *StepOutput {
	if out == nil || len(out.Nested) == 0 {
		return nil
	}
	for _, nested := range out.Nested {
		if nested.StepName == name {
			return nested
		}
		if found := searchNested(nested, name); found != nil {
			return found
		}
	}
	return nil
}

// StepOutput carries results from a step execution.
type StepOutput struct {
	StepName string        // which step produced this
	Content  string        // primary output text
	Success  bool          // did the step succeed?
	Error    error         // non-nil if step failed
	Stop     bool          // should the workflow stop after this step?
	Nested   []*StepOutput // outputs from nested sub-steps (for compound steps)
	Response *Response     // underlying agent response (if agent step)
	Duration time.Duration // how long the step took
	Data     map[string]any // step-specific output data
}

// ── StepRunner interface ─────────────────────────────────────────────

// StepRunner is the core interface for all workflow steps.
// Every step type (AgentStep, FuncStep, Steps, ParallelSteps, LoopStep,
// ConditionStep, RouterStep) implements this.
type StepRunner interface {
	RunStep(ctx context.Context, input *StepInput) (*StepOutput, error)
	StepName() string
}

// ── Error handling ───────────────────────────────────────────────────

// OnError defines what happens when a step fails after all retries.
type OnError int

const (
	OnErrorFail  OnError = iota // propagate error, stop workflow (default)
	OnErrorSkip                 // skip the failed step, continue workflow
	OnErrorPause                // pause workflow for human intervention
)

// ── Step configuration ───────────────────────────────────────────────

// StepConfig holds common configuration for any step.
type StepConfig struct {
	Name                 string
	OnError              OnError
	MaxRetries           int
	RetryDelay           time.Duration
	RequiresConfirmation bool
	SkipIf               func(ctx context.Context, input *StepInput) bool
}

// ── HITL pause/resume ────────────────────────────────────────────────

// PausedWorkflow represents a workflow suspended for human input.
type PausedWorkflow struct {
	PausedAt         string                 // step name where paused
	PauseReason      string                 // why it paused
	CompletedOutputs map[string]*StepOutput // outputs completed before pause
	PendingInput     *StepInput             // input that was about to be processed
	StepIndex        int                    // index in parent Steps
}

// ErrWorkflowPaused is returned when a workflow pauses for HITL.
// Use errors.As to extract the PausedWorkflow.
type ErrWorkflowPaused struct {
	Paused *PausedWorkflow
}

func (e *ErrWorkflowPaused) Error() string {
	return fmt.Sprintf("agnogo: workflow paused at step %q: %s", e.Paused.PausedAt, e.Paused.PauseReason)
}

// ── Error handling helper ────────────────────────────────────────────

// executeWithErrorHandling wraps a step execution with retry and error policy.
func executeWithErrorHandling(ctx context.Context, config StepConfig, fn func() (*StepOutput, error)) (*StepOutput, error) {
	attempts := config.MaxRetries + 1
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 && config.RetryDelay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(config.RetryDelay):
			}
		}

		output, err := fn()
		if err == nil {
			return output, nil
		}
		lastErr = err
	}

	// All attempts failed — apply OnError policy
	switch config.OnError {
	case OnErrorSkip:
		return &StepOutput{
			StepName: config.Name,
			Content:  "",
			Success:  false,
			Error:    lastErr,
		}, nil // nil error = workflow continues
	case OnErrorPause:
		return nil, &ErrWorkflowPaused{
			Paused: &PausedWorkflow{
				PausedAt:    config.Name,
				PauseReason: fmt.Sprintf("step failed after %d attempt(s): %v", attempts, lastErr),
			},
		}
	default: // OnErrorFail
		return nil, fmt.Errorf("agnogo: step %q failed: %w", config.Name, lastErr)
	}
}

// ── State flow helper ────────────────────────────────────────────────

// buildChildInput creates a StepInput for a child step from accumulated state.
func buildChildInput(parent *StepInput, prevName, prevContent string, prevOutputs map[string]*StepOutput) *StepInput {
	input := prevContent
	if input == "" && parent != nil {
		input = parent.Input
	}
	var data map[string]any
	var session *Session
	if parent != nil {
		data = parent.Data
		session = parent.Session
	}
	return &StepInput{
		Input:       input,
		PrevContent: prevContent,
		PrevOutputs: prevOutputs,
		Data:        data,
		Session:     session,
	}
}

// ── WorkflowEngine ───────────────────────────────────────────────────

// WorkflowEngine orchestrates step execution with structured data flow.
// It implements the existing Workflow interface for backward compatibility.
type WorkflowEngine struct {
	name    string
	root    StepRunner
	storage Storage // optional: persist paused workflows
}

// NewWorkflowEngine creates a workflow engine with a root step.
func NewWorkflowEngine(name string, root StepRunner) *WorkflowEngine {
	return &WorkflowEngine{name: name, root: root}
}

// WithStorage sets a storage backend for persisting paused workflows.
func (we *WorkflowEngine) WithStorage(s Storage) *WorkflowEngine {
	we.storage = s
	return we
}

// RunWorkflow executes the workflow from the beginning.
// Returns ErrWorkflowPaused if a step requires human intervention.
func (we *WorkflowEngine) RunWorkflow(ctx context.Context, session *Session, input string) (*StepOutput, error) {
	if session == nil {
		return nil, fmt.Errorf("agnogo: session is nil")
	}
	si := &StepInput{
		Input:       input,
		PrevOutputs: make(map[string]*StepOutput),
		Data:        make(map[string]any),
		Session:     session,
	}
	return we.root.RunStep(ctx, si)
}

// ResumeWorkflow continues a paused workflow after human approval/input.
// If approved is false, the paused step is skipped and execution continues
// from the next step.
func (we *WorkflowEngine) ResumeWorkflow(ctx context.Context, session *Session, paused *PausedWorkflow, approved bool, humanInput string) (*StepOutput, error) {
	if paused == nil {
		return nil, fmt.Errorf("agnogo: no paused workflow to resume")
	}

	// Rebuild input from paused state
	prevOutputs := paused.CompletedOutputs
	if prevOutputs == nil {
		prevOutputs = make(map[string]*StepOutput)
	}

	// Determine input text
	input := humanInput
	if input == "" && paused.PendingInput != nil {
		input = paused.PendingInput.Input
	}

	var data map[string]any
	if paused.PendingInput != nil {
		data = paused.PendingInput.Data
	}
	if data == nil {
		data = make(map[string]any)
	}

	// Store approval decision in data for the step to read
	data["_approval_decision"] = approved
	data["_human_input"] = humanInput
	data["_resume_from"] = paused.PausedAt
	data["_resume_index"] = paused.StepIndex

	si := &StepInput{
		Input:       input,
		PrevOutputs: prevOutputs,
		Data:        data,
		Session:     session,
	}

	return we.root.RunStep(ctx, si)
}

// Run implements the existing Workflow interface for backward compatibility.
// Converts StepOutput → Response, ErrWorkflowPaused → Response{NeedsApproval}.
func (we *WorkflowEngine) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	output, err := we.RunWorkflow(ctx, session, input)
	if err != nil {
		var paused *ErrWorkflowPaused
		if errors.As(err, &paused) {
			return &Response{
				Text:          paused.Error(),
				NeedsApproval: true,
			}, nil
		}
		return nil, err
	}
	resp := &Response{
		Text:        output.Content,
		ToolsCalled: collectStepTools(output),
	}
	return resp, nil
}

// collectStepTools gathers all tool names from a StepOutput tree.
func collectStepTools(out *StepOutput) []string {
	if out == nil {
		return nil
	}
	var tools []string
	if out.Response != nil {
		tools = append(tools, out.Response.ToolsCalled...)
	}
	for _, nested := range out.Nested {
		tools = append(tools, collectStepTools(nested)...)
	}
	return tools
}

// ── WorkflowAdapter — bridge old Workflow to new StepRunner ──────────

// WorkflowAdapter wraps an existing Workflow as a StepRunner.
type WorkflowAdapter struct {
	name     string
	workflow Workflow
}

// AdaptWorkflow wraps an existing Workflow (Sequential, Parallel, etc.)
// so it can be used as a step inside the new WorkflowEngine.
func AdaptWorkflow(name string, wf Workflow) *WorkflowAdapter {
	return &WorkflowAdapter{name: name, workflow: wf}
}

func (a *WorkflowAdapter) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	start := time.Now()
	resp, err := a.workflow.Run(ctx, input.Session, input.Input)
	dur := time.Since(start)
	if err != nil {
		return &StepOutput{StepName: a.name, Error: err, Duration: dur}, err
	}
	return &StepOutput{
		StepName: a.name,
		Content:  resp.Text,
		Success:  true,
		Response: resp,
		Duration: dur,
	}, nil
}

func (a *WorkflowAdapter) StepName() string { return a.name }

// ── Convenience constructors ─────────────────────────────────────────

// WfStep creates an agent step.
func WfStep(name string, agent *Core) *AgentStep {
	return NewAgentStep(name, agent)
}

// WfFunc creates a function step.
func WfFunc(name string, fn func(ctx context.Context, input *StepInput) (*StepOutput, error)) *FuncStep {
	return NewFuncStep(name, fn)
}

// WfSequence creates a sequential step composition.
func WfSequence(name string, steps ...StepRunner) *Steps {
	return NewSteps(name, steps...)
}

// WfParallel creates a parallel step composition.
func WfParallel(name string, steps ...StepRunner) *ParallelSteps {
	return NewParallelSteps(name, steps...)
}

// WfLoop creates a loop step.
func WfLoop(name string, body StepRunner, stopWhen func(*StepOutput, int) bool) *LoopStep {
	return NewLoopStep(name, body, stopWhen)
}

// WfCondition creates a condition step.
func WfCondition(name string, eval func(context.Context, *StepInput) bool, trueBranch StepRunner, falseBranch ...StepRunner) *ConditionStep {
	return NewConditionStep(name, eval, trueBranch, falseBranch...)
}

// WfRoute creates a router step.
func WfRoute(name string, selector func(context.Context, *StepInput) string, routes map[string]StepRunner) *RouterStep {
	return NewRouterStep(name, selector, routes)
}
