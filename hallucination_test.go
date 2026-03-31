package agnogo

import (
	"context"
	"strings"
	"testing"
)

// helper: build a detector with tools registered so the guard actually fires.
func testDetector() *hallucinationDetector {
	reg := NewToolRegistry()
	reg.Add("get_time", "Get current date and time", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "2026-03-31T12:00:00Z", nil
	})
	reg.Add("get_weather", "Get weather forecast for a location", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "sunny 22°C", nil
	})
	reg.Add("get_stock_price", "Get stock price and financial data", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "$142.50", nil
	})
	return &hallucinationDetector{
		tools:    reg,
		patterns: getDefaultPatterns(),
	}
}

// ── Pattern matching tests ───────────────────────────────

func TestHallucinationDetectsDate(t *testing.T) {
	d := testDetector()
	matches := d.findAllMatches("The date is March 29, 2026 and the meeting is at 14:30.")
	found := false
	for _, m := range matches {
		if m.Category == "date" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to detect a date pattern")
	}
}

func TestHallucinationDetectsRelativeTime(t *testing.T) {
	d := testDetector()

	cases := []string{
		"I'll have that ready for you tomorrow.",
		"As of now, the server is running fine.",
		"Let's schedule it for next monday.",
		"The report from last week shows growth.",
		"Currently the system is operational.",
	}
	for _, msg := range cases {
		matches := d.findAllMatches(msg)
		found := false
		for _, m := range matches {
			if m.Category == "relative_time" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected relative_time match in %q", msg)
		}
	}
}

func TestHallucinationDetectsWeather(t *testing.T) {
	d := testDetector()

	cases := []string{
		"It will be sunny all day.",
		"Expect overcast skies with drizzle.",
		"Conditions are foggy this morning.",
		"There is a thunderstorm warning.",
	}
	for _, msg := range cases {
		matches := d.findAllMatches(msg)
		found := false
		for _, m := range matches {
			if m.Category == "weather" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected weather match in %q", msg)
		}
	}
}

func TestHallucinationDetectsFinancial(t *testing.T) {
	d := testDetector()

	cases := []struct {
		msg      string
		category string
	}{
		{"The stock price 142.50 has been rising.", "financial"},
		{"Check $AAPL for the latest data.", "financial"},
		{"Market cap 2.5 trillion.", "financial"},
		{"Trading at 98.7% of peak.", "financial"},
	}
	for _, tc := range cases {
		matches := d.findAllMatches(tc.msg)
		found := false
		for _, m := range matches {
			if m.Category == tc.category {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s match in %q", tc.category, tc.msg)
		}
	}
}

func TestHallucinationDetectsCurrency(t *testing.T) {
	d := testDetector()

	cases := []string{
		"That will cost $100.",
		"The price is 350 SEK.",
		"Your total is €50.",
	}
	for _, msg := range cases {
		matches := d.findAllMatches(msg)
		found := false
		for _, m := range matches {
			if m.Category == "currency" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected currency match in %q", msg)
		}
	}
}

func TestHallucinationNoFalsePositiveOnNormalText(t *testing.T) {
	d := testDetector()

	innocuous := []string{
		"Hello, how can I help you?",
		"Please provide more details about your request.",
		"I would be happy to assist with that.",
		"The function returns a boolean value.",
		"Here is a summary of the configuration options.",
	}
	for _, msg := range innocuous {
		matches := d.findAllMatches(msg)
		if len(matches) > 0 {
			t.Errorf("false positive on %q: got %v", msg, matches)
		}
	}
}

func TestHallucinationWordBoundaries(t *testing.T) {
	d := testDetector()

	// "item14:30cost" should NOT match the time pattern because 14:30 is not
	// surrounded by word boundaries in this context.
	matches := d.findAllMatches("item14:30cost")
	for _, m := range matches {
		if m.Category == "time" && m.Match == "14:30" {
			t.Errorf("word boundary violation: %q matched time pattern in 'item14:30cost'", m.Match)
		}
	}

	// But "at 14:30 today" SHOULD match.
	matches = d.findAllMatches("The meeting is at 14:30 today.")
	found := false
	for _, m := range matches {
		if m.Category == "time" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected time match in 'at 14:30 today'")
	}
}

func TestHallucinationSeverityLikely(t *testing.T) {
	// Multiple matches should yield "likely".
	matches := []HallucinationMatch{
		{Category: "date", Match: "March 29, 2026"},
		{Category: "time", Match: "14:30"},
	}
	sev := determineSeverity(matches)
	if sev != SeverityLikely {
		t.Errorf("expected severity %q, got %q", SeverityLikely, sev)
	}
}

func TestHallucinationSeverityPossible(t *testing.T) {
	// Single ambiguous match (not relative_time) should yield "possible".
	matches := []HallucinationMatch{
		{Category: "currency", Match: "$100"},
	}
	sev := determineSeverity(matches)
	if sev != SeverityPossible {
		t.Errorf("expected severity %q, got %q", SeverityPossible, sev)
	}

	// A single relative_time match should be "likely" (strong signal).
	matches = []HallucinationMatch{
		{Category: "relative_time", Match: "tomorrow"},
	}
	sev = determineSeverity(matches)
	if sev != SeverityLikely {
		t.Errorf("expected severity %q for relative_time, got %q", SeverityLikely, sev)
	}
}

func TestHallucinationGuardMethodOnCore(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "It is March 29, 2026 at 14:30 and it is sunny today."}}}
	a := New(Config{Model: model, Instructions: "test"})
	a.Tool("get_time", "Get current date and time", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "now", nil
	})

	// HallucinationGuard() should add an output guardrail.
	a.HallucinationGuard()
	if len(a.outputGuards) == 0 {
		t.Fatal("expected at least one output guardrail after HallucinationGuard()")
	}

	// The guardrail should fire on hallucinated content.
	sess := NewSession("test")
	sess.History = append(sess.History, Message{Role: "user", Content: "What time is it today?"})
	guard := a.outputGuards[len(a.outputGuards)-1]
	err := guard.Check(context.Background(), sess, "It is March 29, 2026 at 14:30 and today is sunny.")
	if err == nil {
		t.Error("expected hallucination guard to return an error for hallucinated content")
	} else if !strings.Contains(err.Error(), "hallucination-guard") {
		t.Errorf("unexpected error: %v", err)
	}

	// HallucinationGuardWithPatterns() should also work.
	a2 := New(Config{Model: model, Instructions: "test"})
	a2.Tool("get_time", "Get current date and time", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "now", nil
	})
	a2.HallucinationGuardWithPatterns([]string{`\bFOOBAR\b`})
	if len(a2.outputGuards) == 0 {
		t.Fatal("expected output guardrail after HallucinationGuardWithPatterns()")
	}
}
