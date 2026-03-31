package agnogo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── CostTracker Tests ───────────────────────────────────

func TestCostTrackerEstimate(t *testing.T) {
	ct := NewCostTracker()
	ct.SetPrice("test-model", 5.0, 15.0)

	usage := &Usage{InputTokens: 1_000_000, OutputTokens: 500_000}
	cost := ct.Estimate("test-model", usage)

	// 1M input * 5.0/1M + 0.5M output * 15.0/1M = 5.0 + 7.5 = 12.5
	if cost != 12.5 {
		t.Errorf("cost = %f, want 12.5", cost)
	}
}

func TestCostTrackerUnknownModel(t *testing.T) {
	ct := NewCostTracker()
	usage := &Usage{InputTokens: 1000, OutputTokens: 500}
	cost := ct.Estimate("nonexistent-model", usage)
	if cost != 0 {
		t.Errorf("cost = %f, want 0 for unknown model", cost)
	}
}

// ── MetricsCollector Tests ──────────────────────────────

func TestMetricsCollectorTrace(t *testing.T) {
	mc := NewMetricsCollector()
	tr := mc.Trace()

	// Simulate a model call
	resp := &ModelResponse{
		Text:  "hello",
		Usage: &Usage{InputTokens: 100, OutputTokens: 50},
	}
	tr.OnModelCall(nil, resp, 200*time.Millisecond)

	// Simulate a tool call
	tr.OnToolCall("search", nil, "result", 50*time.Millisecond, nil)
	tr.OnToolCall("search", nil, "result2", 30*time.Millisecond, nil)
	tr.OnToolCall("fetch", nil, "data", 10*time.Millisecond, nil)

	snap := mc.Snapshot()

	if snap.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d, want 1", snap.TotalRuns)
	}
	if snap.TotalTokensIn != 100 {
		t.Errorf("TotalTokensIn = %d, want 100", snap.TotalTokensIn)
	}
	if snap.TotalTokensOut != 50 {
		t.Errorf("TotalTokensOut = %d, want 50", snap.TotalTokensOut)
	}
	if snap.TotalToolCalls != 3 {
		t.Errorf("TotalToolCalls = %d, want 3", snap.TotalToolCalls)
	}
	if snap.ToolCallCounts["search"] != 2 {
		t.Errorf("ToolCallCounts[search] = %d, want 2", snap.ToolCallCounts["search"])
	}
	if snap.ToolCallCounts["fetch"] != 1 {
		t.Errorf("ToolCallCounts[fetch] = %d, want 1", snap.ToolCallCounts["fetch"])
	}
}

func TestMetricsCollectorHandler(t *testing.T) {
	mc := NewMetricsCollector()
	tr := mc.Trace()

	// Record some data
	tr.OnModelCall(nil, &ModelResponse{Usage: &Usage{InputTokens: 10, OutputTokens: 5}}, 100*time.Millisecond)
	tr.OnToolCall("ping", nil, "pong", 5*time.Millisecond, nil)

	handler := mc.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var snap MetricsSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if snap.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d, want 1", snap.TotalRuns)
	}
	if snap.TotalToolCalls != 1 {
		t.Errorf("TotalToolCalls = %d, want 1", snap.TotalToolCalls)
	}
}

// ── Validate Tests ──────────────────────────────────────

func TestValidateNilModel(t *testing.T) {
	// Build agent manually to bypass New's nil-model panic
	a := &Core{}
	errs := Validate(a)

	var found bool
	for _, e := range errs {
		if e.Field == "model" && e.Level == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error-level validation for nil model")
	}
}

func TestValidateWarnings(t *testing.T) {
	// Agent with nil model but we only check warnings here
	a := &Core{}
	errs := Validate(a)

	fields := map[string]bool{}
	for _, e := range errs {
		if e.Level == "warning" {
			fields[e.Field] = true
		}
	}

	for _, want := range []string{"tools", "retry", "history", "instructions"} {
		if !fields[want] {
			t.Errorf("missing warning for field %q", want)
		}
	}
}

func TestValidateCleanAgent(t *testing.T) {
	retry := DefaultRetryConfig()
	a := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "ok"}}},
		Instructions: "You are helpful.",
		Retry:        &retry,
		History:      &HistoryConfig{MaxMessages: 50, MaxToolMessages: 10, SummaryThreshold: 30},
	})
	a.Tool("noop", "does nothing", nil, nil)

	errs := Validate(a)
	for _, e := range errs {
		if e.Level == "error" {
			t.Errorf("unexpected error: %s: %s", e.Field, e.Message)
		}
	}
}

// ── Explain Tests ───────────────────────────────────────

func TestExplainDoesNotPanic(t *testing.T) {
	configs := []*Core{
		// Minimal agent
		{model: &mockModel{}, instructions: "hi"},
		// Agent with tools
		func() *Core {
			a := New(Config{Model: &mockModel{}, Instructions: "test"})
			a.Tool("x", "x", nil, nil)
			return a
		}(),
		// Agent with no instructions
		{model: &mockModel{}},
	}

	for i, a := range configs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Explain panicked on config %d: %v", i, r)
				}
			}()
			Explain(a)
		}()
	}
}
