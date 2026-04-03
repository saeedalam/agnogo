package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ── Eval Framework ──────────────────────────────────────────────────
//
// Eval runs test cases against an agent and reports pass/fail with details.
// Use it to prove agent quality, catch regressions, and benchmark reliability.
//
//	eval := agnogo.NewEval(agent)
//	eval.Add("greeting", "Say hello", agnogo.Contains("hello"))
//	eval.Add("tool-use", "What's 2+2?", agnogo.Contains("4"))
//	report := eval.Run(ctx)
//	report.Print()

// Assertion checks whether an agent's response meets expectations.
type Assertion func(response string) error

// Contains checks that the response contains the given substring (case-insensitive).
func Contains(substr string) Assertion {
	return func(response string) error {
		if !containsFold(response, substr) {
			return fmt.Errorf("expected response to contain %q", substr)
		}
		return nil
	}
}

// NotContains checks that the response does NOT contain the substring.
func NotContains(substr string) Assertion {
	return func(response string) error {
		if containsFold(response, substr) {
			return fmt.Errorf("expected response to NOT contain %q", substr)
		}
		return nil
	}
}

// Exact checks for an exact match (trimmed, case-insensitive).
func Exact(expected string) Assertion {
	return func(response string) error {
		if !strings.EqualFold(strings.TrimSpace(response), strings.TrimSpace(expected)) {
			return fmt.Errorf("expected %q, got %q", expected, response)
		}
		return nil
	}
}

// MatchesRegex checks that the response matches the given regex pattern.
func MatchesRegex(pattern string) Assertion {
	re := regexp.MustCompile(pattern)
	return func(response string) error {
		if !re.MatchString(response) {
			return fmt.Errorf("expected response to match /%s/", pattern)
		}
		return nil
	}
}

// LengthBetween checks that the response length is within bounds.
func LengthBetween(min, max int) Assertion {
	return func(response string) error {
		n := len(response)
		if n < min || n > max {
			return fmt.Errorf("expected length %d–%d, got %d", min, max, n)
		}
		return nil
	}
}

// UsedTool checks that a specific tool was called during the response.
// This is a marker assertion — it always passes when checked as text.
// Use AddWithTools() or set EvalCase.toolChecks to verify tool usage.
func UsedTool(toolName string) Assertion {
	return func(_ string) error {
		return nil
	}
}

// Custom allows any custom validation function.
func Custom(name string, fn func(response string) error) Assertion {
	return fn
}

// ── Eval Runner ─────────────────────────────────────────────────────

// EvalCase is a single test case for the eval framework.
type EvalCase struct {
	Name       string
	Input      string
	Assertions []Assertion
	toolChecks []string // tools that should have been used
}

// Eval runs test cases against an agent.
type Eval struct {
	agent       *Core
	cases       []EvalCase
	concurrency int
}

// NewEval creates an eval runner for the given agent.
func NewEval(agent *Core) *Eval {
	return &Eval{
		agent:       agent,
		concurrency: 1,
	}
}

// Add adds a test case with one or more assertions.
func (e *Eval) Add(name, input string, assertions ...Assertion) *Eval {
	e.cases = append(e.cases, EvalCase{
		Name:       name,
		Input:      input,
		Assertions: assertions,
	})
	return e
}

// AddWithTools adds a test case that also verifies specific tools were called.
func (e *Eval) AddWithTools(name, input string, expectedTools []string, assertions ...Assertion) *Eval {
	e.cases = append(e.cases, EvalCase{
		Name:       name,
		Input:      input,
		Assertions: assertions,
		toolChecks: expectedTools,
	})
	return e
}

// AddCase adds a pre-built EvalCase.
func (e *Eval) AddCase(c EvalCase) *Eval {
	e.cases = append(e.cases, c)
	return e
}

// WithConcurrency sets how many eval cases run in parallel.
// Default: 1 (sequential).
func (e *Eval) WithConcurrency(n int) *Eval {
	if n < 1 {
		n = 1
	}
	e.concurrency = n
	return e
}

// Run executes all test cases and returns a report.
func (e *Eval) Run(ctx context.Context) *EvalReport {
	report := &EvalReport{
		StartedAt: time.Now(),
		Results:   make([]EvalResult, len(e.cases)),
	}

	if e.concurrency <= 1 {
		for i, tc := range e.cases {
			report.Results[i] = e.runCase(ctx, tc)
		}
	} else {
		sem := make(chan struct{}, e.concurrency)
		var wg sync.WaitGroup
		for i, tc := range e.cases {
			wg.Add(1)
			go func(idx int, c EvalCase) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				report.Results[idx] = e.runCase(ctx, c)
			}(i, tc)
		}
		wg.Wait()
	}

	report.Duration = time.Since(report.StartedAt)
	for _, r := range report.Results {
		if r.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	return report
}

func (e *Eval) runCase(ctx context.Context, tc EvalCase) EvalResult {
	result := EvalResult{Name: tc.Name, Input: tc.Input}
	start := time.Now()

	session := NewSession("eval-" + tc.Name)
	resp, err := e.agent.Run(ctx, session, tc.Input)
	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		result.Passed = false
		return result
	}

	result.Response = resp.Text
	result.ToolsCalled = resp.ToolsCalled

	// Run assertions.
	result.Passed = true
	for _, assertion := range tc.Assertions {
		if err := assertion(resp.Text); err != nil {
			result.Passed = false
			result.Failures = append(result.Failures, err.Error())
		}
	}

	// Check tool usage.
	for _, expected := range tc.toolChecks {
		found := false
		for _, called := range resp.ToolsCalled {
			if called == expected {
				found = true
				break
			}
		}
		if !found {
			result.Passed = false
			result.Failures = append(result.Failures, fmt.Sprintf("expected tool %q to be called", expected))
		}
	}

	return result
}

// ── Results ─────────────────────────────────────────────────────────

// EvalResult is the outcome of a single test case.
type EvalResult struct {
	Name        string        `json:"name"`
	Input       string        `json:"input"`
	Response    string        `json:"response"`
	ToolsCalled []string      `json:"tools_called,omitempty"`
	Passed      bool          `json:"passed"`
	Failures    []string      `json:"failures,omitempty"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// EvalReport is the full evaluation report.
type EvalReport struct {
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Results   []EvalResult  `json:"results"`
}

// PassRate returns the percentage of tests that passed (0.0–1.0).
func (r *EvalReport) PassRate() float64 {
	total := r.Passed + r.Failed
	if total == 0 {
		return 0
	}
	return float64(r.Passed) / float64(total)
}

// Print prints a human-readable summary.
func (r *EvalReport) Print() {
	fmt.Printf("\n═══ Eval Report ═══════════════════════════════════\n")
	fmt.Printf("  Duration: %s\n", r.Duration.Round(time.Millisecond))
	fmt.Printf("  Pass rate: %d/%d (%.0f%%)\n\n", r.Passed, r.Passed+r.Failed, r.PassRate()*100)

	for _, res := range r.Results {
		icon := "✅"
		if !res.Passed {
			icon = "❌"
		}
		fmt.Printf("  %s %s (%s)\n", icon, res.Name, res.Duration.Round(time.Millisecond))
		if !res.Passed {
			for _, f := range res.Failures {
				fmt.Printf("     → %s\n", f)
			}
			if res.Error != "" {
				fmt.Printf("     → error: %s\n", res.Error)
			}
		}
	}
	fmt.Printf("\n═══════════════════════════════════════════════════\n\n")
}

// JSON returns the report as formatted JSON.
// Returns "{}" if marshaling fails.
func (r *EvalReport) JSON() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}
