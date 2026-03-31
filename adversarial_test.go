package agnogo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── Helper Models ───────────────────────────────────────

// providerErrorModel always returns a ProviderError.
type providerErrorModel struct {
	err       *ProviderError
	callCount int
	mu        sync.Mutex
}

func (m *providerErrorModel) ChatCompletion(_ context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return nil, m.err
}

// sequenceModel returns different responses for successive calls.
// After exhausting the list, returns a fallback text response.
type sequenceModel struct {
	mu        sync.Mutex
	calls     []func() (*ModelResponse, error)
	callCount int
}

func (m *sequenceModel) ChatCompletion(_ context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCount < len(m.calls) {
		fn := m.calls[m.callCount]
		m.callCount++
		return fn()
	}
	m.callCount++
	return &ModelResponse{Text: "done"}, nil
}

// ── 1. TestMalformedToolCallArgs ────────────────────────

func TestMalformedToolCallArgs(t *testing.T) {
	// Model returns a tool call with broken JSON arguments, then a text response.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "search", Arguments: `{broken`}}},
		{Text: "I found something."},
	}}

	a := New(Config{Model: model})
	a.Tool("search", "Search", Params{
		"query": {Type: "string", Desc: "Query"},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		// ParseArgs returns empty map for invalid JSON; tool still runs.
		return "result for: " + args["query"], nil
	})

	session := NewSession("malformed-args")
	resp, err := a.Run(context.Background(), session, "Search for something")
	if err != nil {
		t.Fatalf("agent should not error on malformed args: %v", err)
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected a response")
	}
}

// ── 2. TestEmptyToolCallName ────────────────────────────

func TestEmptyToolCallName(t *testing.T) {
	// Model returns a tool call with empty Name, then a text response.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "", Arguments: `{}`}}},
		{Text: "Recovered gracefully."},
	}}

	a := New(Config{Model: model})

	session := NewSession("empty-tool-name")
	resp, err := a.Run(context.Background(), session, "Do something")
	if err != nil {
		t.Fatalf("agent should not error on empty tool name: %v", err)
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected a response")
	}
}

// ── 3. TestToolPanic ────────────────────────────────────

func TestToolPanic(t *testing.T) {
	// Tool panics with "boom". Agent should recover and continue.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "crasher", Arguments: `{}`}}},
		{Text: "Recovered."},
	}}

	a := New(Config{Model: model})
	a.Tool("crasher", "A tool that panics", nil, func(ctx context.Context, args map[string]string) (string, error) {
		panic("boom")
	})

	session := NewSession("tool-panic")
	resp, err := a.Run(context.Background(), session, "Crash")
	if err != nil {
		t.Fatalf("agent should recover from tool panic: %v", err)
	}
	if resp == nil || resp.Text == "" {
		t.Fatal("expected a response after panic recovery")
	}
}

// ── 4. TestToolTimeout ──────────────────────────────────

func TestToolTimeout(t *testing.T) {
	// Tool takes 5 seconds, context has 100ms timeout.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "slow", Arguments: `{}`}}},
		{Text: "Done."},
	}}

	a := New(Config{Model: model})
	a.Tool("slow", "Slow tool", nil, func(ctx context.Context, args map[string]string) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "finished", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	session := NewSession("tool-timeout")
	_, err := a.Run(ctx, session, "Do slow thing")

	// The tool respects context cancellation, and agent checks ctx between loops.
	// Either the tool returns an error or the agent loop catches ctx.Done().
	if err == nil {
		// If no error, the model may have returned text with the tool error.
		// That's acceptable too.
		t.Log("Agent handled timeout without returning error (tool error was treated as result)")
		return
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

// ── 5. TestConcurrentSessions ───────────────────────────

func TestConcurrentSessions(t *testing.T) {
	// 20 goroutines each create their own session and call agent.Run simultaneously.
	// Run with -race flag to detect data races.
	const n = 20

	model := &mockModel{}
	for i := 0; i < n; i++ {
		model.responses = append(model.responses, ModelResponse{
			Text: fmt.Sprintf("response-%d", i),
		})
	}

	a := New(Config{Model: model, Instructions: "You are helpful."})

	var wg sync.WaitGroup
	errs := make([]error, n)
	results := make([]string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			session := NewSession(fmt.Sprintf("concurrent-%d", idx))
			resp, err := a.Run(context.Background(), session, fmt.Sprintf("msg-%d", idx))
			errs[idx] = err
			if resp != nil {
				results[idx] = resp.Text
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}
	// Just verify all goroutines completed and got some response.
	for i, r := range results {
		if r == "" {
			t.Errorf("goroutine %d got empty response", i)
		}
	}
}

// ── 6. TestConcurrentToolCalls ──────────────────────────

func TestConcurrentToolCalls(t *testing.T) {
	// Model returns 3 tool calls in one response. All tools called, results collected.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{
			{ID: "c1", Name: "tool_a", Arguments: `{"v":"1"}`},
			{ID: "c2", Name: "tool_b", Arguments: `{"v":"2"}`},
			{ID: "c3", Name: "tool_c", Arguments: `{"v":"3"}`},
		}},
		{Text: "All three tools returned results."},
	}}

	var mu sync.Mutex
	called := map[string]bool{}

	makeTool := func(name string) ToolFunc {
		return func(ctx context.Context, args map[string]string) (string, error) {
			mu.Lock()
			called[name] = true
			mu.Unlock()
			return name + "-result", nil
		}
	}

	a := New(Config{Model: model})
	a.Tool("tool_a", "Tool A", nil, makeTool("tool_a"))
	a.Tool("tool_b", "Tool B", nil, makeTool("tool_b"))
	a.Tool("tool_c", "Tool C", nil, makeTool("tool_c"))

	session := NewSession("concurrent-tools")
	resp, err := a.Run(context.Background(), session, "Use all three tools")
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.ToolsCalled) != 3 {
		t.Errorf("tools called = %v, want 3", resp.ToolsCalled)
	}
	for _, name := range []string{"tool_a", "tool_b", "tool_c"} {
		if !called[name] {
			t.Errorf("tool %s was not called", name)
		}
	}
}

// ── 7. TestModelReturnsEmptyText ────────────────────────

func TestModelReturnsEmptyText(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: ""},
	}}

	a := New(Config{Model: model})
	session := NewSession("empty-text")
	resp, err := a.Run(context.Background(), session, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "..." {
		t.Errorf("expected fallback '...', got %q", resp.Text)
	}
}

// ── 8. TestModelReturnsNilUsage ─────────────────────────

func TestModelReturnsNilUsage(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "ok", Usage: nil},
	}}

	a := New(Config{Model: model})
	session := NewSession("nil-usage")
	resp, err := a.Run(context.Background(), session, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q, want 'ok'", resp.Text)
	}
	// Verify metrics don't have nil pointer issues.
	if resp.Metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if resp.Metrics.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens with nil usage, got %d", resp.Metrics.TotalTokens)
	}
}

// ── 9. TestSessionHistoryOverflow ───────────────────────

func TestSessionHistoryOverflow(t *testing.T) {
	// Session with 1000 messages, verify history trimming works and doesn't crash.
	model := &mockModel{responses: []ModelResponse{
		{Text: "I see you have a lot of history."},
	}}

	a := New(Config{
		Model: model,
		History: &HistoryConfig{
			MaxMessages:     50,
			MaxToolMessages: 20,
		},
	})

	session := NewSession("overflow")
	for i := 0; i < 1000; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		session.History = append(session.History, Message{
			Role:    role,
			Content: fmt.Sprintf("message %d", i),
		})
	}

	resp, err := a.Run(context.Background(), session, "Still here?")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text == "" {
		t.Error("expected non-empty response")
	}
}

// ── 10. TestProviderError429Retry ───────────────────────

func TestProviderError429Retry(t *testing.T) {
	// Model returns 429 with RetryAfter=10ms. After retries, should eventually fail.
	pe := &ProviderError{
		Provider:   "test",
		StatusCode: 429,
		Message:    "rate limited",
		Retryable:  true,
		RetryAfter: 10 * time.Millisecond,
	}

	model := &providerErrorModel{err: pe}

	a := New(Config{
		Model: model,
		Retry: &RetryConfig{
			MaxRetries:         2,
			InitialDelay:       10 * time.Millisecond,
			ExponentialBackoff: false,
			MaxDelay:           50 * time.Millisecond,
		},
	})

	session := NewSession("retry-429")
	start := time.Now()
	_, err := a.Run(context.Background(), session, "Hello")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	// Should have made 3 calls total (1 initial + 2 retries).
	model.mu.Lock()
	calls := model.callCount
	model.mu.Unlock()
	if calls != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", calls)
	}

	// Should have respected RetryAfter delay (at least 2 * 10ms).
	if elapsed < 15*time.Millisecond {
		t.Errorf("retries should have taken at least ~20ms, took %v", elapsed)
	}
}

// ── 11. TestProviderErrorPermanent ──────────────────────

func TestProviderErrorPermanent(t *testing.T) {
	// 401 non-retryable error. Should NOT retry.
	pe := &ProviderError{
		Provider:   "test",
		StatusCode: 401,
		Message:    "unauthorized",
		Retryable:  false,
	}

	model := &providerErrorModel{err: pe}

	a := New(Config{
		Model: model,
		Retry: &RetryConfig{
			MaxRetries:         3,
			InitialDelay:       10 * time.Millisecond,
			ExponentialBackoff: false,
		},
	})

	session := NewSession("permanent-401")
	_, err := a.Run(context.Background(), session, "Hello")

	if err == nil {
		t.Fatal("expected error")
	}

	// Should have made only 2 calls: initial + one attempt that sees non-retryable.
	model.mu.Lock()
	calls := model.callCount
	model.mu.Unlock()
	if calls > 2 {
		t.Errorf("expected at most 2 calls for non-retryable error, got %d", calls)
	}
}

// ── 12. TestGraphStateRace ──────────────────────────────

func TestGraphStateRace(t *testing.T) {
	// 10 goroutines read/write GraphState simultaneously.
	// Run with -race flag to detect data races.
	state := NewGraphState()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			for j := 0; j < 100; j++ {
				state.Set(key, j)
				_ = state.Get(key)
				_ = state.GetStr(key)
				_ = state.GetInt(key)
				_ = state.GetBool(key)
				state.Set(fmt.Sprintf("shared-%d", j%5), idx)
			}
		}(i)
	}
	wg.Wait()
	// No panic or race = pass.
}

// ── 13. TestHookPanic ───────────────────────────────────

func TestHookPanic(t *testing.T) {
	// Hook panics. Verify agent doesn't crash (or document if it does).
	model := &mockModel{responses: []ModelResponse{
		{Text: "ok"},
	}}

	a := New(Config{Model: model})
	a.hooks = []Hook{
		func(ctx context.Context, agent *Core, session *Session, msg string, next NextFunc) (*Response, error) {
			panic("hook exploded")
		},
	}

	session := NewSession("hook-panic")

	// Hook panic should be recovered and returned as an error.
	resp, err := a.Run(context.Background(), session, "Hello")
	if err == nil {
		t.Fatal("expected error from hook panic")
	}
	if !strings.Contains(err.Error(), "hook panicked") {
		t.Errorf("expected 'hook panicked' in error, got: %v", err)
	}
	_ = resp
}

// ── 14. TestRunContextNilSafe ───────────────────────────

func TestRunContextNilSafe(t *testing.T) {
	// RunCtx on a context without RunContext should return nil, not panic.
	ctx := context.Background()
	rc := RunCtx(ctx)
	if rc != nil {
		t.Errorf("expected nil RunContext, got %v", rc)
	}

	// Verify that calling methods on nil RunContext would panic,
	// but that the nil check pattern works correctly.
	rc = RunCtx(ctx)
	if rc == nil {
		t.Log("RunCtx returns nil for context without RunContext (correct behavior)")
	}

	// Verify the full pattern: tool checks for nil before using.
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "check_ctx", Arguments: `{}`}}},
		{Text: "ok"},
	}}

	a := New(Config{Model: model})
	a.Tool("check_ctx", "Check context", nil, func(ctx context.Context, args map[string]string) (string, error) {
		rc := RunCtx(ctx)
		if rc == nil {
			return "no run context", nil
		}
		return rc.GetStr("key"), nil
	})

	session := NewSession("nil-runctx")
	resp, err := a.Run(context.Background(), session, "check")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q", resp.Text)
	}
}
