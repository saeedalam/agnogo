package agnogo

import (
	"context"
	"fmt"
	"testing"
)

// ── Assertion Tests ─────────────────────────────────────

func TestContainsAssertion(t *testing.T) {
	a := Contains("hello")
	if err := a("Hello World"); err != nil {
		t.Errorf("should match case-insensitive: %v", err)
	}
	if err := a("goodbye"); err == nil {
		t.Error("should fail when missing")
	}
}

func TestNotContainsAssertion(t *testing.T) {
	a := NotContains("error")
	if err := a("all good"); err != nil {
		t.Errorf("should pass: %v", err)
	}
	if err := a("got an Error here"); err == nil {
		t.Error("should fail when present")
	}
}

func TestExactAssertion(t *testing.T) {
	a := Exact("hello world")
	if err := a("  Hello World  "); err != nil {
		t.Errorf("should match trimmed case-insensitive: %v", err)
	}
	if err := a("hello"); err == nil {
		t.Error("should fail on partial match")
	}
}

func TestMatchesRegexAssertion(t *testing.T) {
	a := MatchesRegex(`\d{3}-\d{4}`)
	if err := a("Call us at 555-1234"); err != nil {
		t.Errorf("should match: %v", err)
	}
	if err := a("no numbers here"); err == nil {
		t.Error("should fail when no match")
	}
}

func TestLengthBetweenAssertion(t *testing.T) {
	a := LengthBetween(5, 20)
	if err := a("hello world"); err != nil {
		t.Errorf("should pass: %v", err)
	}
	if err := a("hi"); err == nil {
		t.Error("should fail for too short")
	}
	if err := a("this is a very long response that exceeds the maximum allowed length by quite a bit"); err == nil {
		t.Error("should fail for too long")
	}
}

func TestCustomAssertion(t *testing.T) {
	a := Custom("is-json", func(resp string) error {
		if resp[0] != '{' {
			return errorf("expected JSON")
		}
		return nil
	})
	if err := a(`{"key": "value"}`); err != nil {
		t.Errorf("should pass: %v", err)
	}
	if err := a("not json"); err == nil {
		t.Error("should fail")
	}
}

// ── Eval Runner Tests ───────────────────────────────────

// mockEvalProvider returns controllable responses for testing.
type mockEvalProvider struct {
	responses map[string]string
}

func (p *mockEvalProvider) ChatCompletion(_ context.Context, messages []Message, _ []map[string]any) (*ModelResponse, error) {
	// Return based on last user message.
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			if resp, ok := p.responses[messages[i].Content]; ok {
				return &ModelResponse{Text: resp}, nil
			}
		}
	}
	return &ModelResponse{Text: "I don't know"}, nil
}

func TestEvalRunBasic(t *testing.T) {
	provider := &mockEvalProvider{
		responses: map[string]string{
			"Say hello":  "Hello! How can I help?",
			"What is 4?": "The answer is 4.",
		},
	}
	agent := Agent("You are helpful.", WithModel(provider), UnsafeMode)

	eval := NewEval(agent)
	eval.Add("greeting", "Say hello", Contains("hello"))
	eval.Add("math", "What is 4?", Contains("4"))

	report := eval.Run(context.Background())

	if report.Passed != 2 {
		t.Errorf("Passed = %d, want 2", report.Passed)
	}
	if report.Failed != 0 {
		t.Errorf("Failed = %d, want 0", report.Failed)
	}
	if report.PassRate() != 1.0 {
		t.Errorf("PassRate = %f, want 1.0", report.PassRate())
	}
}

func TestEvalRunWithFailures(t *testing.T) {
	provider := &mockEvalProvider{
		responses: map[string]string{
			"Say hello": "Goodbye!",
		},
	}
	agent := Agent("test", WithModel(provider), UnsafeMode)

	eval := NewEval(agent)
	eval.Add("bad-greeting", "Say hello", Contains("hello"), NotContains("goodbye"))

	report := eval.Run(context.Background())

	if report.Passed != 0 {
		t.Errorf("Passed = %d, want 0", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
	if len(report.Results[0].Failures) != 2 {
		t.Errorf("Failures = %d, want 2", len(report.Results[0].Failures))
	}
}

func TestEvalConcurrency(t *testing.T) {
	provider := &mockEvalProvider{
		responses: map[string]string{
			"test1": "response1",
			"test2": "response2",
			"test3": "response3",
		},
	}
	agent := Agent("test", WithModel(provider), UnsafeMode)

	eval := NewEval(agent)
	eval.WithConcurrency(3)
	eval.Add("t1", "test1", Contains("response1"))
	eval.Add("t2", "test2", Contains("response2"))
	eval.Add("t3", "test3", Contains("response3"))

	report := eval.Run(context.Background())

	if report.Passed != 3 {
		t.Errorf("Passed = %d, want 3", report.Passed)
	}
}

func TestEvalReportJSON(t *testing.T) {
	provider := &mockEvalProvider{
		responses: map[string]string{"hi": "hello"},
	}
	agent := Agent("test", WithModel(provider), UnsafeMode)

	eval := NewEval(agent)
	eval.Add("json-test", "hi", Contains("hello"))
	report := eval.Run(context.Background())

	jsonStr := report.JSON()
	if jsonStr == "" {
		t.Error("JSON() returned empty string")
	}
	if !containsFold(jsonStr, "json-test") {
		t.Error("JSON should contain test name")
	}
}

func TestEvalPassRateZero(t *testing.T) {
	report := &EvalReport{}
	if report.PassRate() != 0 {
		t.Errorf("PassRate for empty report = %f, want 0", report.PassRate())
	}
}

func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
