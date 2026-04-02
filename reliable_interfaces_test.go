package agnogo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ── Reliable() backward compatibility ───────────────────

func TestReliableDefaults(t *testing.T) {
	a := Agent("reliable agent", WithModel(&mockProvider{}), Reliable())

	// Cost budget should be set
	if a.costBudget == nil {
		t.Error("costBudget should be set")
	}
	if a.costBudget.MaxPerRun != 1.00 {
		t.Errorf("MaxPerRun = %f, want 1.00", a.costBudget.MaxPerRun)
	}
	if a.costBudget.MaxPerSession != 10.00 {
		t.Errorf("MaxPerSession = %f, want 10.00", a.costBudget.MaxPerSession)
	}

	// Tool validator should be set
	if a.toolValidator == nil {
		t.Error("toolValidator should be set")
	}
	if !a.toolValidator.RequireNonEmpty {
		t.Error("RequireNonEmpty should be true")
	}
	if !a.toolValidator.JSONValidate {
		t.Error("JSONValidate should be true")
	}

	// Confidence threshold
	if a.confidenceThreshold != 0.5 {
		t.Errorf("confidenceThreshold = %f, want 0.5", a.confidenceThreshold)
	}

	// Hallucination guard should be registered as output guardrail
	found := false
	for _, g := range a.outputGuards {
		if g.Name == "hallucination-guard" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hallucination-guard in output guards")
	}

	// PII guards should be registered
	hasInputPII := false
	for _, g := range a.inputGuards {
		if g.Name == "pii-input-guard" {
			hasInputPII = true
			break
		}
	}
	if !hasInputPII {
		t.Error("expected pii-input-guard in input guards")
	}
}

// ── Custom hallucination checker ────────────────────────

type testHallucinationChecker struct {
	called bool
	block  bool
}

func (h *testHallucinationChecker) Check(_ context.Context, _ *Session, response string) error {
	h.called = true
	if h.block {
		return errors.New("custom hallucination detected")
	}
	return nil
}

func TestReliableCustomHallucination(t *testing.T) {
	checker := &testHallucinationChecker{}
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomHallucination(checker),
	))

	// Should have the custom checker wired as output guardrail
	found := false
	for _, g := range a.outputGuards {
		if g.Name == "hallucination-guard" {
			found = true
			// Invoke it to verify our custom checker is called
			_ = g.Check(context.Background(), NewSession("test"), "test response")
			break
		}
	}
	if !found {
		t.Error("expected hallucination-guard in output guards")
	}
	if !checker.called {
		t.Error("custom hallucination checker should have been called")
	}
}

func TestReliableCustomHallucinationBlocks(t *testing.T) {
	checker := &testHallucinationChecker{block: true}
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomHallucination(checker),
	))

	for _, g := range a.outputGuards {
		if g.Name == "hallucination-guard" {
			err := g.Check(context.Background(), NewSession("test"), "bad response")
			if err == nil {
				t.Error("expected error from custom checker")
			}
			if !strings.Contains(err.Error(), "custom hallucination") {
				t.Errorf("unexpected error: %v", err)
			}
			return
		}
	}
	t.Error("hallucination-guard not found")
}

// ── Custom PII scanner ──────────────────────────────────

type testPIIScanner struct {
	detectCalled bool
	redactCalled bool
}

func (s *testPIIScanner) Detect(text string) []PIIMatch {
	s.detectCalled = true
	if strings.Contains(text, "secret@evil.com") {
		return []PIIMatch{{Type: PIIEmail, Match: "secret@evil.com", Redacted: "[CUSTOM REDACTED]"}}
	}
	return nil
}

func (s *testPIIScanner) Redact(text string) string {
	s.redactCalled = true
	return strings.ReplaceAll(text, "secret@evil.com", "[CUSTOM REDACTED]")
}

func TestReliableCustomPII(t *testing.T) {
	scanner := &testPIIScanner{}
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomPII(scanner),
	))

	// Output guard should use custom scanner
	for _, g := range a.outputGuards {
		if g.Name == "pii-output-guard" {
			err := g.Check(context.Background(), NewSession("test"), "my email is secret@evil.com")
			if err == nil {
				t.Error("expected PII block error")
			}
			if !scanner.detectCalled {
				t.Error("custom scanner Detect() should have been called")
			}
			return
		}
	}
	t.Error("pii-output-guard not found")
}

func TestReliableCustomPIIAllowsClean(t *testing.T) {
	scanner := &testPIIScanner{}
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomPII(scanner),
	))

	for _, g := range a.outputGuards {
		if g.Name == "pii-output-guard" {
			err := g.Check(context.Background(), NewSession("test"), "no sensitive data here")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			return
		}
	}
}

// ── Custom budget ───────────────────────────────────────

func TestReliableCustomBudget(t *testing.T) {
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithReliableBudget(0.25, 2.50),
	))

	if a.costBudget.MaxPerRun != 0.25 {
		t.Errorf("MaxPerRun = %f, want 0.25", a.costBudget.MaxPerRun)
	}
	if a.costBudget.MaxPerSession != 2.50 {
		t.Errorf("MaxPerSession = %f, want 2.50", a.costBudget.MaxPerSession)
	}
}

// ── Custom confidence threshold ─────────────────────────

func TestReliableCustomConfidence(t *testing.T) {
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithReliableConfidenceThreshold(0.8),
	))

	if a.confidenceThreshold != 0.8 {
		t.Errorf("confidenceThreshold = %f, want 0.8", a.confidenceThreshold)
	}
}

// ── ReliableWith backward compat ────────────────────────

func TestReliableWithBackwardCompat(t *testing.T) {
	a := Agent("test", WithModel(&mockProvider{}), ReliableWith(0.50, 5.00))

	if a.costBudget.MaxPerRun != 0.50 {
		t.Errorf("MaxPerRun = %f, want 0.50", a.costBudget.MaxPerRun)
	}
	if a.costBudget.MaxPerSession != 5.00 {
		t.Errorf("MaxPerSession = %f, want 5.00", a.costBudget.MaxPerSession)
	}
}

// ── HallucinationCheckerFunc adapter ────────────────────

func TestHallucinationCheckerFunc(t *testing.T) {
	called := false
	fn := HallucinationCheckerFunc(func(_ context.Context, _ *Session, resp string) error {
		called = true
		if strings.Contains(resp, "BLOCK") {
			return errors.New("blocked")
		}
		return nil
	})

	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomHallucination(fn),
	))

	for _, g := range a.outputGuards {
		if g.Name == "hallucination-guard" {
			_ = g.Check(context.Background(), NewSession("test"), "ok")
			if !called {
				t.Error("func adapter should have been called")
			}
			return
		}
	}
	t.Error("hallucination-guard not found")
}

// ── ConfidenceScorerFunc adapter ────────────────────────

func TestConfidenceScorerFunc(t *testing.T) {
	scorer := ConfidenceScorerFunc(func(resp *Response, _ *Session, tc int) ConfidenceScore {
		return ConfidenceScore{Score: 0.99, Reasons: []string{"custom scorer"}}
	})

	result := scorer.Score(&Response{Text: "test"}, NewSession("test"), 0)
	if result.Score != 0.99 {
		t.Errorf("Score = %f, want 0.99", result.Score)
	}
}

// ── DefaultPIIScanner ───────────────────────────────────

func TestDefaultPIIScanner(t *testing.T) {
	scanner := &DefaultPIIScanner{}

	matches := scanner.Detect("email me at test@example.com")
	if len(matches) == 0 {
		t.Error("expected PII match for email")
	}

	redacted := scanner.Redact("email me at test@example.com")
	if strings.Contains(redacted, "test@example.com") {
		t.Error("expected email to be redacted")
	}
}

// ── Multiple custom overrides ───────────────────────────

func TestReliableMultipleOverrides(t *testing.T) {
	checker := &testHallucinationChecker{}
	scanner := &testPIIScanner{}
	a := Agent("test", WithModel(&mockProvider{}), Reliable(
		WithCustomHallucination(checker),
		WithCustomPII(scanner),
		WithReliableBudget(0.10, 1.00),
		WithReliableConfidenceThreshold(0.9),
	))

	// All should be set
	if a.costBudget.MaxPerRun != 0.10 {
		t.Errorf("MaxPerRun = %f, want 0.10", a.costBudget.MaxPerRun)
	}
	if a.confidenceThreshold != 0.9 {
		t.Errorf("confidenceThreshold = %f, want 0.9", a.confidenceThreshold)
	}

	// Verify custom hallucination checker
	for _, g := range a.outputGuards {
		if g.Name == "hallucination-guard" {
			_ = g.Check(context.Background(), NewSession("test"), "x")
			if !checker.called {
				t.Error("custom hallucination checker should be used")
			}
			break
		}
	}

	// Verify custom PII scanner
	for _, g := range a.outputGuards {
		if g.Name == "pii-output-guard" {
			_ = g.Check(context.Background(), NewSession("test"), "secret@evil.com")
			if !scanner.detectCalled {
				t.Error("custom PII scanner should be used")
			}
			break
		}
	}
}
