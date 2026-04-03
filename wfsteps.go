package agnogo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ── AgentStep — wraps a *Core agent ──────────────────────────────────

// AgentStep executes a single LLM agent as a workflow step.
type AgentStep struct {
	config StepConfig
	agent  *Core
}

// NewAgentStep creates a step that runs an LLM agent.
func NewAgentStep(name string, agent *Core) *AgentStep {
	return &AgentStep{
		config: StepConfig{Name: name},
		agent:  agent,
	}
}

func (s *AgentStep) WithOnError(mode OnError) *AgentStep {
	s.config.OnError = mode
	return s
}

func (s *AgentStep) WithRetries(n int) *AgentStep {
	s.config.MaxRetries = n
	return s
}

func (s *AgentStep) WithRetryDelay(d time.Duration) *AgentStep {
	s.config.RetryDelay = d
	return s
}

func (s *AgentStep) WithConfirmation() *AgentStep {
	s.config.RequiresConfirmation = true
	return s
}

func (s *AgentStep) WithSkipIf(fn func(ctx context.Context, input *StepInput) bool) *AgentStep {
	s.config.SkipIf = fn
	return s
}

func (s *AgentStep) StepName() string { return s.config.Name }

func (s *AgentStep) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	// Skip check
	if s.config.SkipIf != nil && s.config.SkipIf(ctx, input) {
		return &StepOutput{StepName: s.config.Name, Content: input.Input, Success: true}, nil
	}

	// HITL confirmation check
	if s.config.RequiresConfirmation {
		// Check if this is a resume with approval
		if input.Data != nil {
			if resumeFrom, ok := input.Data["_resume_from"].(string); ok && resumeFrom == s.config.Name {
				approved, _ := input.Data["_approval_decision"].(bool)
				if !approved {
					return &StepOutput{StepName: s.config.Name, Content: "Step was not approved.", Success: true}, nil
				}
				// Approved — fall through to execute
			} else {
				// Not a resume — pause for confirmation
				return nil, &ErrWorkflowPaused{
					Paused: &PausedWorkflow{
						PausedAt:    s.config.Name,
						PauseReason: fmt.Sprintf("step %q requires confirmation before execution", s.config.Name),
					},
				}
			}
		} else {
			return nil, &ErrWorkflowPaused{
				Paused: &PausedWorkflow{
					PausedAt:    s.config.Name,
					PauseReason: fmt.Sprintf("step %q requires confirmation before execution", s.config.Name),
				},
			}
		}
	}

	return executeWithErrorHandling(ctx, s.config, func() (*StepOutput, error) {
		start := time.Now()
		session := input.Session
		if session == nil {
			session = NewSession("wf-" + s.config.Name)
		}
		resp, err := s.agent.Run(ctx, session, input.Input)
		dur := time.Since(start)
		if err != nil {
			return nil, fmt.Errorf("agent %q: %w", s.config.Name, err)
		}
		return &StepOutput{
			StepName: s.config.Name,
			Content:  resp.Text,
			Success:  true,
			Response: resp,
			Duration: dur,
		}, nil
	})
}

// ── FuncStep — wraps a Go function ───────────────────────────────────

// FuncStep executes a plain Go function as a workflow step.
// No LLM call — for validation, transformation, API calls, etc.
type FuncStep struct {
	config StepConfig
	fn     func(ctx context.Context, input *StepInput) (*StepOutput, error)
}

// NewFuncStep creates a step that runs a Go function.
func NewFuncStep(name string, fn func(ctx context.Context, input *StepInput) (*StepOutput, error)) *FuncStep {
	return &FuncStep{
		config: StepConfig{Name: name},
		fn:     fn,
	}
}

func (s *FuncStep) WithOnError(mode OnError) *FuncStep {
	s.config.OnError = mode
	return s
}

func (s *FuncStep) WithRetries(n int) *FuncStep {
	s.config.MaxRetries = n
	return s
}

func (s *FuncStep) StepName() string { return s.config.Name }

func (s *FuncStep) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	return executeWithErrorHandling(ctx, s.config, func() (*StepOutput, error) {
		start := time.Now()
		output, err := s.fn(ctx, input)
		if err != nil {
			return nil, err
		}
		if output.StepName == "" {
			output.StepName = s.config.Name
		}
		if output.Duration == 0 {
			output.Duration = time.Since(start)
		}
		return output, nil
	})
}

// ── Steps — sequential composition ───────────────────────────────────

// Steps executes steps in order. Each step's output flows to the next
// via StepInput.PrevOutputs.
type Steps struct {
	config StepConfig
	steps  []StepRunner
}

// NewSteps creates a sequential step composition.
func NewSteps(name string, steps ...StepRunner) *Steps {
	return &Steps{
		config: StepConfig{Name: name},
		steps:  steps,
	}
}

func (s *Steps) StepName() string { return s.config.Name }

func (s *Steps) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	prevOutputs := make(map[string]*StepOutput)
	// Copy any existing previous outputs (for resume scenarios)
	for k, v := range input.PrevOutputs {
		prevOutputs[k] = v
	}

	var nested []*StepOutput
	prevContent := input.Input
	startIndex := 0

	// Check if we're resuming from a paused step
	if input.Data != nil {
		if resumeIdx, ok := input.Data["_resume_index"].(int); ok {
			startIndex = resumeIdx
		}
	}

	for i := startIndex; i < len(s.steps); i++ {
		step := s.steps[i]

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		childInput := buildChildInput(input, "", prevContent, prevOutputs)

		output, err := step.RunStep(ctx, childInput)
		if err != nil {
			// Check for HITL pause — enrich with context and propagate
			var paused *ErrWorkflowPaused
			if errors.As(err, &paused) {
				paused.Paused.CompletedOutputs = prevOutputs
				paused.Paused.PendingInput = childInput
				paused.Paused.StepIndex = i
				return nil, err
			}
			return nil, err
		}

		prevOutputs[step.StepName()] = output
		nested = append(nested, output)
		prevContent = output.Content

		if output.Stop {
			break
		}
	}

	// Final output is the last step's content
	finalContent := prevContent
	return &StepOutput{
		StepName: s.config.Name,
		Content:  finalContent,
		Success:  true,
		Nested:   nested,
	}, nil
}

// ── ParallelSteps — concurrent composition ───────────────────────────

// ParallelSteps executes steps concurrently and merges results.
type ParallelSteps struct {
	config    StepConfig
	steps     []StepRunner
	mergeFunc func(outputs map[string]*StepOutput) *StepOutput
}

// NewParallelSteps creates a parallel step composition.
func NewParallelSteps(name string, steps ...StepRunner) *ParallelSteps {
	return &ParallelSteps{
		config: StepConfig{Name: name},
		steps:  steps,
	}
}

// WithMerge sets a custom merge function for combining parallel results.
func (p *ParallelSteps) WithMerge(fn func(outputs map[string]*StepOutput) *StepOutput) *ParallelSteps {
	p.mergeFunc = fn
	return p
}

func (p *ParallelSteps) StepName() string { return p.config.Name }

func (p *ParallelSteps) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	start := time.Now()

	type indexedResult struct {
		index  int
		name   string
		output *StepOutput
		err    error
	}

	results := make([]indexedResult, len(p.steps))
	var wg sync.WaitGroup

	for i, step := range p.steps {
		wg.Add(1)
		go func(idx int, s StepRunner) {
			defer wg.Done()

			// Clone session for parallel isolation
			var parallelSession *Session
			if input.Session != nil {
				parallelSession = cloneSession(input.Session, input.Session.ID+"_par_"+s.StepName())
			}

			// Copy Data map to prevent concurrent map writes
			var dataCopy map[string]any
			if input.Data != nil {
				dataCopy = make(map[string]any, len(input.Data))
				for k, v := range input.Data {
					dataCopy[k] = v
				}
			}

			parallelInput := &StepInput{
				Input:       input.Input,
				PrevContent: input.PrevContent,
				PrevOutputs: input.PrevOutputs,
				Data:        dataCopy,
				Session:     parallelSession,
			}

			output, err := s.RunStep(ctx, parallelInput)
			results[idx] = indexedResult{index: idx, name: s.StepName(), output: output, err: err}
		}(i, step)
	}

	wg.Wait()

	// Check for errors and collect outputs
	outputMap := make(map[string]*StepOutput, len(p.steps))
	var nested []*StepOutput
	for _, r := range results {
		if r.err != nil {
			// Propagate first error (including ErrWorkflowPaused)
			return nil, r.err
		}
		outputMap[r.name] = r.output
		nested = append(nested, r.output)
	}

	// Merge results
	if p.mergeFunc != nil {
		merged := p.mergeFunc(outputMap)
		merged.StepName = p.config.Name
		merged.Nested = nested
		merged.Duration = time.Since(start)
		return merged, nil
	}

	// Default merge: concatenate content
	var parts []string
	for _, step := range p.steps {
		if out, ok := outputMap[step.StepName()]; ok && out.Content != "" {
			parts = append(parts, out.Content)
		}
	}

	return &StepOutput{
		StepName: p.config.Name,
		Content:  strings.Join(parts, "\n\n"),
		Success:  true,
		Nested:   nested,
		Duration: time.Since(start),
	}, nil
}

// ── LoopStep — iteration ─────────────────────────────────────────────

// LoopStep repeats a step until a stop condition is met or max iterations reached.
type LoopStep struct {
	config   StepConfig
	body     StepRunner
	stopWhen func(output *StepOutput, iteration int) bool
	maxIter  int
}

// NewLoopStep creates a loop step.
func NewLoopStep(name string, body StepRunner, stopWhen func(*StepOutput, int) bool) *LoopStep {
	return &LoopStep{
		config:   StepConfig{Name: name},
		body:     body,
		stopWhen: stopWhen,
		maxIter:  10,
	}
}

// WithMaxIterations sets the maximum loop iterations (default 10).
func (l *LoopStep) WithMaxIterations(n int) *LoopStep {
	l.maxIter = n
	return l
}

func (l *LoopStep) StepName() string { return l.config.Name }

func (l *LoopStep) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	start := time.Now()
	var nested []*StepOutput
	currentInput := input

	for i := 0; i < l.maxIter; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		output, err := l.body.RunStep(ctx, currentInput)
		if err != nil {
			return nil, err
		}

		nested = append(nested, output)

		if l.stopWhen(output, i) {
			break
		}

		if output.Stop {
			break
		}

		// Feed output back as input for next iteration
		currentInput = &StepInput{
			Input:       output.Content,
			PrevContent: output.Content,
			PrevOutputs: input.PrevOutputs,
			Data:        input.Data,
			Session:     input.Session,
		}
	}

	// Final content is the last iteration's output
	finalContent := ""
	if len(nested) > 0 {
		finalContent = nested[len(nested)-1].Content
	}

	return &StepOutput{
		StepName: l.config.Name,
		Content:  finalContent,
		Success:  true,
		Nested:   nested,
		Duration: time.Since(start),
	}, nil
}

// ── ConditionStep — branching ────────────────────────────────────────

// ConditionStep evaluates a condition and executes the true or false branch.
type ConditionStep struct {
	config      StepConfig
	evaluator   func(ctx context.Context, input *StepInput) bool
	trueBranch  StepRunner
	falseBranch StepRunner // optional — nil means pass through
}

// NewConditionStep creates a condition step.
func NewConditionStep(name string, eval func(ctx context.Context, input *StepInput) bool, trueBranch StepRunner, falseBranch ...StepRunner) *ConditionStep {
	var fb StepRunner
	if len(falseBranch) > 0 {
		fb = falseBranch[0]
	}
	return &ConditionStep{
		config:      StepConfig{Name: name},
		evaluator:   eval,
		trueBranch:  trueBranch,
		falseBranch: fb,
	}
}

func (c *ConditionStep) StepName() string { return c.config.Name }

func (c *ConditionStep) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	start := time.Now()

	if c.evaluator(ctx, input) {
		output, err := c.trueBranch.RunStep(ctx, input)
		if err != nil {
			return nil, err
		}
		return &StepOutput{
			StepName: c.config.Name,
			Content:  output.Content,
			Success:  output.Success,
			Nested:   []*StepOutput{output},
			Duration: time.Since(start),
		}, nil
	}

	if c.falseBranch != nil {
		output, err := c.falseBranch.RunStep(ctx, input)
		if err != nil {
			return nil, err
		}
		return &StepOutput{
			StepName: c.config.Name,
			Content:  output.Content,
			Success:  output.Success,
			Nested:   []*StepOutput{output},
			Duration: time.Since(start),
		}, nil
	}

	// No false branch — pass through input unchanged
	return &StepOutput{
		StepName: c.config.Name,
		Content:  input.Input,
		Success:  true,
		Duration: time.Since(start),
	}, nil
}

// ── RouterStep — dynamic selection ───────────────────────────────────

// RouterStep selects one step from a map based on a selector function.
type RouterStep struct {
	config   StepConfig
	selector func(ctx context.Context, input *StepInput) string
	routes   map[string]StepRunner
	fallback string
}

// NewRouterStep creates a router step.
func NewRouterStep(name string, selector func(ctx context.Context, input *StepInput) string, routes map[string]StepRunner) *RouterStep {
	var fallback string
	for k := range routes {
		fallback = k
		break
	}
	return &RouterStep{
		config:   StepConfig{Name: name},
		selector: selector,
		routes:   routes,
		fallback: fallback,
	}
}

// WithFallback sets the default route if selector returns an unknown name.
func (r *RouterStep) WithFallback(name string) *RouterStep {
	r.fallback = name
	return r
}

func (r *RouterStep) StepName() string { return r.config.Name }

func (r *RouterStep) RunStep(ctx context.Context, input *StepInput) (*StepOutput, error) {
	start := time.Now()
	selected := r.selector(ctx, input)

	// Record routing decision in session
	if input.Session != nil {
		input.Session.Set("_routed_to", selected)
	}

	step, ok := r.routes[selected]
	if !ok {
		step = r.routes[r.fallback]
		if step == nil {
			return &StepOutput{
				StepName: r.config.Name,
				Content:  fmt.Sprintf("no route found for: %s", selected),
				Success:  false,
				Duration: time.Since(start),
			}, nil
		}
	}

	output, err := step.RunStep(ctx, input)
	if err != nil {
		return nil, err
	}

	return &StepOutput{
		StepName: r.config.Name,
		Content:  output.Content,
		Success:  output.Success,
		Nested:   []*StepOutput{output},
		Duration: time.Since(start),
	}, nil
}
