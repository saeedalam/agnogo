package agnogo

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── Basic Span Collection ───────────────────────────────────────────

func TestSpanCollectorModelCall(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Simulate a model call
	trace.OnModelCall(nil, &ModelResponse{
		Text:  "hello",
		Usage: &Usage{InputTokens: 100, OutputTokens: 50, TotalTokens: 150},
	}, 500*time.Millisecond)

	rt := sc.Collect(nil)
	if len(rt.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(rt.Spans))
	}
	if rt.Spans[0].Kind != SpanModel {
		t.Errorf("kind = %v, want SpanModel", rt.Spans[0].Kind)
	}
	if rt.Spans[0].InputTokens != 100 {
		t.Errorf("input tokens = %d", rt.Spans[0].InputTokens)
	}
	if rt.ModelCalls != 1 {
		t.Errorf("model calls = %d", rt.ModelCalls)
	}
	if rt.TotalTokens != 150 {
		t.Errorf("total tokens = %d", rt.TotalTokens)
	}
}

func TestSpanCollectorToolCall(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnToolCall("search", map[string]string{"q": "test"}, "found 3 results", 200*time.Millisecond, nil)

	rt := sc.Collect(nil)
	if rt.ToolCalls != 1 {
		t.Errorf("tool calls = %d", rt.ToolCalls)
	}
	if rt.Spans[0].ToolResult != "found 3 results" {
		t.Errorf("tool result = %q", rt.Spans[0].ToolResult)
	}
}

func TestSpanCollectorGuardrail(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnGuardrail("pii-check", "input", false) // passed
	trace.OnGuardrail("hallucination", "output", true) // blocked

	rt := sc.Collect(nil)
	if len(rt.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(rt.Spans))
	}
	if rt.Spans[0].Blocked {
		t.Error("first guardrail should not be blocked")
	}
	if !rt.Spans[1].Blocked {
		t.Error("second guardrail should be blocked")
	}
	if rt.Spans[1].Status != SpanBlocked {
		t.Errorf("blocked status = %v", rt.Spans[1].Status)
	}
}

func TestSpanCollectorReasoning(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnReasoning(ReasoningStep{Title: "Analyze", Confidence: 0.8}, 0)
	trace.OnReasoning(ReasoningStep{Title: "Validate", Confidence: 0.9}, 1)

	rt := sc.Collect(nil)
	// Should be 1 parent reasoning span with 2 children
	if len(rt.Spans) != 1 {
		t.Fatalf("expected 1 parent span, got %d", len(rt.Spans))
	}
	if rt.Spans[0].Kind != SpanReasoning {
		t.Errorf("kind = %v", rt.Spans[0].Kind)
	}
	if len(rt.Spans[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(rt.Spans[0].Children))
	}
	if rt.Spans[0].Children[0].Confidence != 0.8 {
		t.Errorf("first step confidence = %f", rt.Spans[0].Children[0].Confidence)
	}
}

// ── Concurrent Safety ───────────────────────────────────────────────

func TestSpanCollectorConcurrentTools(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Simulate 10 concurrent tool calls (like parallel tool execution)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			time.Sleep(time.Duration(n) * time.Millisecond)
			trace.OnToolCall("tool", nil, "result", 10*time.Millisecond, nil)
		}(i)
	}
	wg.Wait()

	rt := sc.Collect(nil)
	if rt.ToolCalls != 10 {
		t.Errorf("tool calls = %d, want 10", rt.ToolCalls)
	}
	if len(rt.Spans) != 10 {
		t.Errorf("spans = %d, want 10", len(rt.Spans))
	}
}

// ── Full Agent Integration ──────────────────────────────────────────

func TestSpanCollectorWithAgent(t *testing.T) {
	// Mock model with a tool call then text response
	model := &mockModel{responses: []ModelResponse{
		{
			ToolCalls: []ToolCall{{ID: "c1", Name: "search", Arguments: `{"q":"test"}`}},
		},
		{Text: "Here are the results."},
	}}

	sc := NewSpanCollector()
	agent := New(Config{Model: model})
	agent.Tool("search", "search tool", nil, func(_ context.Context, _ map[string]string) (string, error) {
		return "3 results found", nil
	})

	// Attach span collector via trace
	agent.trace = sc.Trace()

	session := NewSession("span-test")
	resp, err := agent.Run(context.Background(), session, "find stuff")
	if err != nil {
		t.Fatal(err)
	}

	trace := sc.Collect(resp)

	// Should have: model call → tool call → model call
	if trace.ModelCalls < 2 {
		t.Errorf("model calls = %d, want >= 2", trace.ModelCalls)
	}
	if trace.ToolCalls < 1 {
		t.Errorf("tool calls = %d, want >= 1", trace.ToolCalls)
	}
	if trace.RunID == "" {
		t.Error("RunID should be set from Response.Metrics")
	}
}

// ── Print Output ────────────────────────────────────────────────────

func TestRunTracePrint(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnGuardrail("pii-input", "input", false)
	trace.OnModelCall(nil, &ModelResponse{
		Usage: &Usage{InputTokens: 200, OutputTokens: 100},
	}, 1200*time.Millisecond)
	trace.OnToolCall("check_availability", nil, "3 slots found", 400*time.Millisecond, nil)
	trace.OnGuardrail("hallucination", "output", false)

	rt := sc.Collect(nil)

	// Just verify Print doesn't panic
	rt.Print()
}

// ── JSON Output ─────────────────────────────────────────────────────

func TestRunTraceJSON(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnModelCall(nil, &ModelResponse{
		Usage: &Usage{InputTokens: 100, OutputTokens: 50},
	}, 500*time.Millisecond)

	rt := sc.Collect(nil)
	jsonStr := rt.JSON()

	if jsonStr == "" || jsonStr == "{}" {
		t.Error("JSON should not be empty")
	}

	// Verify it's valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify span kind is serialized as string
	if !strings.Contains(jsonStr, `"model"`) {
		t.Error("span kind should be serialized as 'model' string")
	}
}

// ── Reset ───────────────────────────────────────────────────────────

func TestSpanCollectorReset(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnModelCall(nil, &ModelResponse{}, 100*time.Millisecond)
	trace.OnToolCall("x", nil, "y", 50*time.Millisecond, nil)

	rt1 := sc.Collect(nil)
	if len(rt1.Spans) != 2 {
		t.Fatalf("before reset: spans = %d", len(rt1.Spans))
	}

	sc.Reset()

	rt2 := sc.Collect(nil)
	if len(rt2.Spans) != 0 {
		t.Errorf("after reset: spans = %d, want 0", len(rt2.Spans))
	}
}

// ── Collect/Reset Independence ───────────────────────────────────────

func TestCollectIndependentFromReset(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	trace.OnModelCall(nil, &ModelResponse{Usage: &Usage{InputTokens: 10}}, 100*time.Millisecond)

	rt := sc.Collect(nil)
	if len(rt.Spans) != 1 {
		t.Fatalf("before reset: spans = %d", len(rt.Spans))
	}

	// Reset should NOT affect the already-collected trace
	sc.Reset()

	if len(rt.Spans) != 1 {
		t.Errorf("after reset: collected trace spans = %d, want 1 (should be independent)", len(rt.Spans))
	}
}

// ── JSON Duration Milliseconds ──────────────────────────────────────

func TestRunTraceJSONDurationIsMilliseconds(t *testing.T) {
	sc := NewSpanCollector()
	// Wait a bit so duration is measurable
	time.Sleep(10 * time.Millisecond)
	rt := sc.Collect(nil)

	jsonStr := rt.JSON()

	var parsed map[string]any
	json.Unmarshal([]byte(jsonStr), &parsed)

	durMS, ok := parsed["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms not found or wrong type in JSON")
	}

	// Should be ~10ms, not ~10,000,000 (nanoseconds)
	if durMS > 5000 {
		t.Errorf("duration_ms = %.0f — looks like nanoseconds, not milliseconds", durMS)
	}
	if durMS < 5 {
		t.Errorf("duration_ms = %.0f — too small, might not be measuring correctly", durMS)
	}
}

// ── SpanKind String ─────────────────────────────────────────────────

func TestSpanKindString(t *testing.T) {
	cases := map[SpanKind]string{
		SpanModel:     "model",
		SpanTool:      "tool",
		SpanGuardrail: "guard",
		SpanKnowledge: "knowledge",
		SpanReasoning: "reasoning",
		SpanApproval:  "approval",
		SpanSession:   "session",
	}
	for kind, want := range cases {
		if got := kind.String(); got != want {
			t.Errorf("SpanKind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}
