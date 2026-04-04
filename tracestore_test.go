package agnogo

import (
	"context"
	"testing"
	"time"
)

// ── Layer 1: Trace Store ────────────────────────────────────────────

func TestMemoryTraceStoreSaveLoad(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	trace := &RunTrace{RunID: "run_001", TotalCost: 0.05, TotalTokens: 500}
	if err := store.SaveTrace(ctx, trace); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTrace(ctx, "run_001")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TotalCost != 0.05 {
		t.Errorf("cost = %f", loaded.TotalCost)
	}
}

func TestMemoryTraceStoreNotFound(t *testing.T) {
	store := NewMemoryTraceStore()
	_, err := store.LoadTrace(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemoryTraceStoreDelete(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "run_del"})
	store.DeleteTrace(ctx, "run_del")

	if store.Count() != 0 {
		t.Errorf("count = %d after delete", store.Count())
	}
}

func TestMemoryTraceStoreQueryMinCost(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "cheap", TotalCost: 0.001})
	store.SaveTrace(ctx, &RunTrace{RunID: "expensive", TotalCost: 0.10})
	store.SaveTrace(ctx, &RunTrace{RunID: "mid", TotalCost: 0.02})

	results, err := store.QueryTraces(ctx, TraceQuery{MinCost: 0.01})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results (mid + expensive), got %d", len(results))
	}
}

func TestMemoryTraceStoreQueryHasErrors(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "ok", HasErrors: false})
	store.SaveTrace(ctx, &RunTrace{RunID: "bad", HasErrors: true})

	hasErrors := true
	results, _ := store.QueryTraces(ctx, TraceQuery{HasErrors: &hasErrors})
	if len(results) != 1 || results[0].RunID != "bad" {
		t.Errorf("expected 1 error trace, got %d", len(results))
	}
}

func TestMemoryTraceStoreQuerySessionID(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "r1", SessionID: "s1"})
	store.SaveTrace(ctx, &RunTrace{RunID: "r2", SessionID: "s2"})
	store.SaveTrace(ctx, &RunTrace{RunID: "r3", SessionID: "s1"})

	results, _ := store.QueryTraces(ctx, TraceQuery{SessionID: "s1"})
	if len(results) != 2 {
		t.Errorf("expected 2 traces for session s1, got %d", len(results))
	}
}

func TestMemoryTraceStoreQueryLimit(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		store.SaveTrace(ctx, &RunTrace{RunID: runID(i)})
	}

	results, _ := store.QueryTraces(ctx, TraceQuery{Limit: 3})
	if len(results) != 3 {
		t.Errorf("expected 3, got %d", len(results))
	}
}

func TestMemoryTraceStoreQueryTimeWindow(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()

	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-10 * time.Minute)

	store.SaveTrace(ctx, &RunTrace{RunID: "old", StartTime: old})
	store.SaveTrace(ctx, &RunTrace{RunID: "recent", StartTime: recent})

	results, _ := store.QueryTraces(ctx, TraceQuery{Since: time.Now().Add(-1 * time.Hour)})
	if len(results) != 1 || results[0].RunID != "recent" {
		t.Errorf("expected 1 recent trace, got %d", len(results))
	}
}

func TestAutoSaveOnCollect(t *testing.T) {
	store := NewMemoryTraceStore()
	sc := NewSpanCollector().WithTraceStore(store)
	trace := sc.Trace()

	trace.OnModelCall(nil, &ModelResponse{
		Text: "hello", Usage: &Usage{InputTokens: 100},
	}, 500*time.Millisecond)

	// Collect with a Response that has a RunID
	sc.Collect(&Response{Metrics: &RunMetrics{RunID: "run_auto"}})

	// Should be auto-saved
	if store.Count() != 1 {
		t.Errorf("expected 1 auto-saved trace, got %d", store.Count())
	}

	loaded, err := store.LoadTrace(context.Background(), "run_auto")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TotalTokens != 100 {
		t.Errorf("tokens = %d", loaded.TotalTokens)
	}
}

func TestUserMessageCapture(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Simulate model call with messages (like agent.go does)
	msgs := []Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Book Thursday 2pm"},
	}
	trace.OnModelCall(msgs, &ModelResponse{Usage: &Usage{}}, 100*time.Millisecond)

	rt := sc.Collect(nil)
	if rt.UserMessage != "Book Thursday 2pm" {
		t.Errorf("user message = %q", rt.UserMessage)
	}
}

// ── Layer 2: Trace Intelligence ─────────────────────────────────────

func TestCostSummary(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	store.SaveTrace(ctx, &RunTrace{RunID: "r1", StartTime: now, TotalCost: 0.01, TotalTokens: 100, Duration: time.Second})
	store.SaveTrace(ctx, &RunTrace{RunID: "r2", StartTime: now, TotalCost: 0.03, TotalTokens: 300, Duration: 2 * time.Second})
	store.SaveTrace(ctx, &RunTrace{RunID: "r3", StartTime: now, TotalCost: 0.02, TotalTokens: 200, Duration: time.Second})

	analyzer := NewTraceAnalyzer(store)
	summary, err := analyzer.CostSummary(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.RunCount != 3 {
		t.Errorf("run count = %d", summary.RunCount)
	}
	if summary.TotalCost < 0.059 || summary.TotalCost > 0.061 {
		t.Errorf("total cost = %f", summary.TotalCost)
	}
	if summary.MaxCost != 0.03 {
		t.Errorf("max cost = %f", summary.MaxCost)
	}
	if summary.TokenCount != 600 {
		t.Errorf("tokens = %d", summary.TokenCount)
	}
}

func TestCostSummaryEmpty(t *testing.T) {
	analyzer := NewTraceAnalyzer(NewMemoryTraceStore())
	summary, err := analyzer.CostSummary(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.RunCount != 0 {
		t.Errorf("expected 0 runs")
	}
}

func TestDetectAnomaliesHighCost(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	// 9 normal runs at ~$0.01
	for i := 0; i < 9; i++ {
		store.SaveTrace(ctx, &RunTrace{
			RunID: runID(i), StartTime: now,
			TotalCost: 0.01, Duration: time.Second, ModelCalls: 2,
		})
	}
	// 1 expensive outlier
	store.SaveTrace(ctx, &RunTrace{
		RunID: "run_outlier", StartTime: now,
		TotalCost: 0.50, Duration: time.Second, ModelCalls: 2,
	})

	analyzer := NewTraceAnalyzer(store)
	anomalies, err := analyzer.DetectAnomalies(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, a := range anomalies {
		if a.RunID == "run_outlier" && a.Type == "high_cost" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected high_cost anomaly for run_outlier")
	}
}

func TestDetectAnomaliesErrors(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	store.SaveTrace(ctx, &RunTrace{RunID: "ok1", StartTime: now, TotalCost: 0.01, Duration: time.Second})
	store.SaveTrace(ctx, &RunTrace{RunID: "ok2", StartTime: now, TotalCost: 0.01, Duration: time.Second})
	store.SaveTrace(ctx, &RunTrace{RunID: "bad", StartTime: now, TotalCost: 0.01, Duration: time.Second, HasErrors: true})

	analyzer := NewTraceAnalyzer(store)
	anomalies, _ := analyzer.DetectAnomalies(ctx, now.Add(-time.Hour))

	found := false
	for _, a := range anomalies {
		if a.RunID == "bad" && a.Type == "error" {
			found = true
		}
	}
	if !found {
		t.Error("expected error anomaly for 'bad' run")
	}
}

func TestDetectAnomaliesNotEnoughData(t *testing.T) {
	store := NewMemoryTraceStore()
	store.SaveTrace(context.Background(), &RunTrace{RunID: "only_one", StartTime: time.Now()})

	analyzer := NewTraceAnalyzer(store)
	anomalies, _ := analyzer.DetectAnomalies(context.Background(), time.Now().Add(-time.Hour))
	if anomalies != nil {
		t.Error("expected nil anomalies with < 3 traces")
	}
}

func TestToolStats(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	store.SaveTrace(ctx, &RunTrace{
		RunID: "r1", StartTime: now,
		Spans: []*Span{
			{Name: "search", Kind: SpanTool, Duration: 100 * time.Millisecond, Status: SpanOK},
			{Name: "search", Kind: SpanTool, Duration: 200 * time.Millisecond, Status: SpanOK},
			{Name: "book", Kind: SpanTool, Duration: 50 * time.Millisecond, Status: SpanError},
		},
	})

	analyzer := NewTraceAnalyzer(store)
	stats, err := analyzer.ToolStats(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	search := stats["search"]
	if search == nil {
		t.Fatal("missing search stats")
	}
	if search.CallCount != 2 {
		t.Errorf("search calls = %d", search.CallCount)
	}
	if search.ErrorRate != 0.0 {
		t.Errorf("search error rate = %f", search.ErrorRate)
	}

	book := stats["book"]
	if book == nil {
		t.Fatal("missing book stats")
	}
	if book.ErrorRate != 1.0 {
		t.Errorf("book error rate = %f", book.ErrorRate)
	}
}

func TestErrorReport(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	store.SaveTrace(ctx, &RunTrace{RunID: "ok", StartTime: now})
	store.SaveTrace(ctx, &RunTrace{RunID: "err1", StartTime: now, HasErrors: true})
	store.SaveTrace(ctx, &RunTrace{RunID: "err2", StartTime: now, HasErrors: true})

	analyzer := NewTraceAnalyzer(store)
	errors, _ := analyzer.ErrorReport(ctx, now.Add(-time.Hour))
	if len(errors) != 2 {
		t.Errorf("expected 2 error traces, got %d", len(errors))
	}
}

// ── Layer 3: Trace Replay ───────────────────────────────────────────

func TestReplay(t *testing.T) {
	original := &RunTrace{
		RunID:       "run_original",
		UserMessage: "What time is it?",
		TotalCost:   0.005,
		TotalTokens: 200,
		Duration:    2 * time.Second,
		ModelCalls:  2,
		ToolCalls:   1,
	}

	agent := New(Config{
		Model: &mockModel{responses: []ModelResponse{{Text: "It's 10:30 AM"}}},
	})

	result, err := Replay(context.Background(), original, agent)
	if err != nil {
		t.Fatal(err)
	}
	if result.Original.RunID != "run_original" {
		t.Errorf("original RunID = %q", result.Original.RunID)
	}
	if result.Replayed == nil {
		t.Fatal("replayed trace is nil")
	}
	if result.Diff == nil {
		t.Fatal("diff is nil")
	}
	if result.Diff.ReplayedResponse != "It's 10:30 AM" {
		t.Errorf("replayed response = %q", result.Diff.ReplayedResponse)
	}
}

func TestReplayNoUserMessage(t *testing.T) {
	original := &RunTrace{RunID: "run_empty"}
	agent := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "hi"}}}})

	_, err := Replay(context.Background(), original, agent)
	if err == nil {
		t.Fatal("expected error for missing UserMessage")
	}
}

func TestReplayPrint(t *testing.T) {
	result := &ReplayResult{
		Original: &RunTrace{
			RunID: "original", UserMessage: "test",
			TotalCost: 0.005, TotalTokens: 200, Duration: 2 * time.Second,
			ModelCalls: 2, ToolCalls: 1,
		},
		Replayed: &RunTrace{
			RunID: "replayed",
			TotalCost: 0.003, TotalTokens: 150, Duration: time.Second,
			ModelCalls: 1, ToolCalls: 1,
		},
		Diff: &TraceDiff{
			CostDelta: -0.002, TokenDelta: -50,
			DurationDelta: -time.Second, ModelCallDelta: -1,
		},
	}
	// Just verify Print doesn't panic
	result.Print()
}

// ── Fix: ResponseText in RunTrace ────────────────────────────────────

func TestResponseTextCapture(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()
	trace.OnModelCall(nil, &ModelResponse{Usage: &Usage{}}, 100*time.Millisecond)

	rt := sc.Collect(&Response{Text: "The answer is 42"})
	if rt.ResponseText != "The answer is 42" {
		t.Errorf("response text = %q", rt.ResponseText)
	}
}

// ── Fix: SessionID via OnRunStart ────────────────────────────────────

func TestSessionIDFromRunStart(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Simulate what agent.go does
	session := NewSession("booking-123")
	trace.OnRunStart("run_abc", session)
	trace.OnModelCall(nil, &ModelResponse{Usage: &Usage{}}, 100*time.Millisecond)

	rt := sc.Collect(nil)
	if rt.SessionID != "booking-123" {
		t.Errorf("sessionID = %q, want %q", rt.SessionID, "booking-123")
	}
}

// ── Fix: Model Name in Cost Estimation ──────────────────────────────

func TestModelNameInCostEstimation(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Model that identifies itself
	trace.OnModelCall(nil, &ModelResponse{
		Model: "gpt-4o",
		Usage: &Usage{InputTokens: 1000, OutputTokens: 500},
	}, 100*time.Millisecond)

	rt := sc.Collect(nil)
	if rt.Spans[0].Cost == 0 {
		t.Error("cost should be > 0")
	}
	// gpt-4o costs more than gpt-4.1-mini — verify different pricing
	// gpt-4o: $2.50/M input, $10.00/M output → 1000*2.50/1M + 500*10/1M = 0.0025 + 0.005 = 0.0075
	if rt.Spans[0].Cost < 0.005 {
		t.Errorf("cost = %f, expected gpt-4o pricing (>$0.005)", rt.Spans[0].Cost)
	}
}

func TestModelNameFallback(t *testing.T) {
	sc := NewSpanCollector()
	trace := sc.Trace()

	// Model that doesn't identify itself — should fallback to gpt-4.1-mini
	trace.OnModelCall(nil, &ModelResponse{
		Usage: &Usage{InputTokens: 1000, OutputTokens: 500},
	}, 100*time.Millisecond)

	rt := sc.Collect(nil)
	// gpt-4.1-mini: $0.40/M input, $1.60/M output → much cheaper
	if rt.Spans[0].Cost > 0.005 {
		t.Errorf("cost = %f, expected gpt-4.1-mini pricing (<$0.005)", rt.Spans[0].Cost)
	}
}

// ── Fix: FileTraceStore ─────────────────────────────────────────────

func TestFileTraceStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileTraceStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	trace := &RunTrace{RunID: "run_file1", TotalCost: 0.05, UserMessage: "hello"}
	if err := store.SaveTrace(context.Background(), trace); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTrace(context.Background(), "run_file1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TotalCost != 0.05 {
		t.Errorf("cost = %f", loaded.TotalCost)
	}
	if loaded.UserMessage != "hello" {
		t.Errorf("user message = %q", loaded.UserMessage)
	}
}

func TestFileTraceStoreQuery(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileTraceStore(dir)
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "cheap", TotalCost: 0.001, StartTime: time.Now()})
	store.SaveTrace(ctx, &RunTrace{RunID: "expensive", TotalCost: 0.10, StartTime: time.Now()})

	results, err := store.QueryTraces(ctx, TraceQuery{MinCost: 0.01})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 expensive trace, got %d", len(results))
	}
}

func TestFileTraceStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewFileTraceStore(dir)
	ctx := context.Background()

	store.SaveTrace(ctx, &RunTrace{RunID: "to_delete"})
	store.DeleteTrace(ctx, "to_delete")

	_, err := store.LoadTrace(ctx, "to_delete")
	if err == nil {
		t.Error("expected error after delete")
	}
}

// ── Fix: CostTrend ──────────────────────────────────────────────────

func TestCostTrend(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	// "Previous" window: 2-1 hours ago, avg $0.01
	store.SaveTrace(ctx, &RunTrace{RunID: "old1", StartTime: now.Add(-90 * time.Minute), TotalCost: 0.01})
	store.SaveTrace(ctx, &RunTrace{RunID: "old2", StartTime: now.Add(-80 * time.Minute), TotalCost: 0.01})

	// "Current" window: last hour, avg $0.05
	store.SaveTrace(ctx, &RunTrace{RunID: "new1", StartTime: now.Add(-30 * time.Minute), TotalCost: 0.05})
	store.SaveTrace(ctx, &RunTrace{RunID: "new2", StartTime: now.Add(-20 * time.Minute), TotalCost: 0.05})

	analyzer := NewTraceAnalyzer(store)
	trend, err := analyzer.CostTrend(ctx, time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if trend.Direction != "increasing" {
		t.Errorf("direction = %q, want increasing", trend.Direction)
	}
	if trend.ChangePercent < 100 {
		t.Errorf("change = %.0f%%, expected > 100%% increase", trend.ChangePercent)
	}
}

func TestCostTrendStable(t *testing.T) {
	store := NewMemoryTraceStore()
	ctx := context.Background()
	now := time.Now()

	store.SaveTrace(ctx, &RunTrace{RunID: "old1", StartTime: now.Add(-90 * time.Minute), TotalCost: 0.01})
	store.SaveTrace(ctx, &RunTrace{RunID: "new1", StartTime: now.Add(-30 * time.Minute), TotalCost: 0.01})

	analyzer := NewTraceAnalyzer(store)
	trend, _ := analyzer.CostTrend(ctx, time.Hour, time.Hour)

	if trend.Direction != "stable" {
		t.Errorf("direction = %q, want stable", trend.Direction)
	}
}

// ── Fix: TraceDiff ResponseChanged ──────────────────────────────────

func TestReplayResponseChanged(t *testing.T) {
	original := &RunTrace{
		RunID:        "run_orig",
		UserMessage:  "What time is it?",
		ResponseText: "It's 10:00 AM",
	}

	agent := New(Config{
		Model: &mockModel{responses: []ModelResponse{{Text: "It's 3:00 PM"}}},
	})

	result, err := Replay(context.Background(), original, agent)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Diff.ResponseChanged {
		t.Error("response should have changed (different text)")
	}
	if result.Diff.OriginalResponse != "It's 10:00 AM" {
		t.Errorf("original response = %q", result.Diff.OriginalResponse)
	}
	if result.Diff.ReplayedResponse != "It's 3:00 PM" {
		t.Errorf("replayed response = %q", result.Diff.ReplayedResponse)
	}
}

func TestReplayResponseUnchanged(t *testing.T) {
	original := &RunTrace{
		RunID:        "run_same",
		UserMessage:  "Hello",
		ResponseText: "Hi there!",
	}

	agent := New(Config{
		Model: &mockModel{responses: []ModelResponse{{Text: "Hi there!"}}},
	})

	result, _ := Replay(context.Background(), original, agent)
	if result.Diff.ResponseChanged {
		t.Error("response should NOT have changed (same text)")
	}
}

// ── Helpers ──────────────────────────────────────────────────────────

func runID(i int) string {
	return "run_" + string(rune('a'+i))
}
