package agnogo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ── History Tests ────────────────────────────────────────

func TestTrimHistory(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are helpful."},
	}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, Message{Role: "user", Content: fmt.Sprintf("msg %d", i)})
		msgs = append(msgs, Message{Role: "assistant", Content: fmt.Sprintf("reply %d", i)})
	}
	// 1 system + 40 conversation = 41 total

	cfg := HistoryConfig{MaxMessages: 10}
	trimmed := trimHistory(msgs, cfg)
	if len(trimmed) > 12 { // system + marker + 10 recent
		t.Errorf("trimmed len = %d, want <= 12", len(trimmed))
	}
	if trimmed[0].Role != "system" {
		t.Error("first message should be system")
	}
	// Last message should be the most recent
	if !strings.Contains(trimmed[len(trimmed)-1].Content, "reply 19") {
		t.Errorf("last message = %q, want reply 19", trimmed[len(trimmed)-1].Content)
	}
}

func TestTrimHistoryNoOp(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
	}
	cfg := HistoryConfig{MaxMessages: 50}
	trimmed := trimHistory(msgs, cfg)
	if len(trimmed) != 2 {
		t.Errorf("should not trim, got %d", len(trimmed))
	}
}

func TestTrimToolMessages(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "tool", Content: "result1"},
		{Role: "tool", Content: "result2"},
		{Role: "tool", Content: "result3"},
		{Role: "tool", Content: "result4"},
		{Role: "tool", Content: "result5"},
		{Role: "assistant", Content: "done"},
	}
	trimmed := trimToolMessages(msgs, 2)
	toolCount := 0
	for _, m := range trimmed {
		if m.Role == "tool" {
			toolCount++
		}
	}
	if toolCount != 2 {
		t.Errorf("tool count = %d, want 2", toolCount)
	}
}

// ── Retry Tests ──────────────────────────────────────────

func TestRetrySuccess(t *testing.T) {
	attempts := 0
	cfg := RetryConfig{MaxRetries: 3, InitialDelay: 0}
	resp, err := retryModelCall(context.Background(), cfg, func() (*ModelResponse, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("temporary error")
		}
		return &ModelResponse{Text: "ok"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q", resp.Text)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetryExhausted(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, InitialDelay: 0}
	_, err := retryModelCall(context.Background(), cfg, func() (*ModelResponse, error) {
		return nil, errors.New("always fails")
	})
	if err == nil {
		t.Error("expected error after retries exhausted")
	}
}

// ── Debug Tests ──────────────────────────────────────────

func TestDebugPrint(t *testing.T) {
	var output []string
	dbg := DebugConfig{
		Enabled: true,
		Level:   2,
		Printer: func(s string) { output = append(output, s) },
	}

	dbg.printResponse("Hello World")
	if len(output) != 1 {
		t.Fatalf("output count = %d", len(output))
	}
	if !strings.Contains(output[0], "Hello World") {
		t.Errorf("output = %q", output[0])
	}
}

func TestDebugDisabled(t *testing.T) {
	var output []string
	dbg := DebugConfig{
		Enabled: false,
		Printer: func(s string) { output = append(output, s) },
	}

	dbg.printResponse("Should not appear")
	if len(output) != 0 {
		t.Error("debug should be disabled")
	}
}

// ── Workflow Tests ────────────────────────────────────────

func TestSequentialWorkflow(t *testing.T) {
	step1 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "step1 done"}}}})
	step2 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "step2 done"}}}})

	wf := Sequential(Step("first", step1), Step("second", step2))
	session := NewSession("wf-1")
	resp, err := wf.Run(context.Background(), session, "start")
	if err != nil {
		t.Fatal(err)
	}
	// Final output should be from step2
	if resp.Text != "step2 done" {
		t.Errorf("text = %q, want 'step2 done'", resp.Text)
	}
}

func TestParallelWorkflow(t *testing.T) {
	agent1 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "weather: sunny"}}}})
	agent2 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "news: market up"}}}})

	wf := Parallel(Step("weather", agent1), Step("news", agent2))
	session := NewSession("wf-2")
	resp, err := wf.Run(context.Background(), session, "briefing")
	if err != nil {
		t.Fatal(err)
	}
	// Both results should be in the merged output
	if !strings.Contains(resp.Text, "weather") || !strings.Contains(resp.Text, "news") {
		t.Errorf("parallel result = %q, should contain both", resp.Text)
	}
}

func TestLoopWorkflow(t *testing.T) {
	iteration := 0
	model := &mockModel{}
	for i := 0; i < 5; i++ {
		model.responses = append(model.responses, ModelResponse{Text: fmt.Sprintf("iteration %d", i)})
	}
	agent := New(Config{Model: model})

	wf := Loop(agent, func(resp *Response, i int) bool {
		iteration = i
		return i >= 2 // stop after 3 iterations
	}).WithMaxIterations(10)

	session := NewSession("wf-3")
	resp, _ := wf.Run(context.Background(), session, "start")
	if iteration != 2 {
		t.Errorf("stopped at iteration %d, want 2", iteration)
	}
	if !strings.Contains(resp.Text, "iteration 2") {
		t.Errorf("text = %q", resp.Text)
	}
}

// ── Structured Output Tests ──────────────────────────────

func TestRunStructured(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"name": "Erik", "age": 30}`},
	}}
	agent := New(Config{Model: model})
	session := NewSession("struct-1")

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var result Person
	err := RunStructured(context.Background(), agent, session, "Who is Erik?", &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "Erik" || result.Age != 30 {
		t.Errorf("result = %+v", result)
	}
}

func TestRunStructuredMarkdown(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "```json\n{\"city\": \"Stockholm\"}\n```"},
	}}
	agent := New(Config{Model: model})
	session := NewSession("struct-2")

	type Location struct {
		City string `json:"city"`
	}
	var result Location
	err := RunStructured(context.Background(), agent, session, "Where?", &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.City != "Stockholm" {
		t.Errorf("city = %q", result.City)
	}
}

// ── Knowledge Question Detection ─────────────────────────

func TestLooksLikeQuestionExtended(t *testing.T) {
	questions := []string{
		"What are your opening hours?",
		"How much does a haircut cost?",
		"Vad kostar klippning?",
		"Hur lång tid tar det?",
		"Can you help me?",
		"Do you have parking?",
		"Is there WiFi?",
		"Har ni parkering?",
		"Finns det lediga tider?",
	}
	nonQuestions := []string{
		"Book me a haircut",
		"Cancel my appointment",
		"OK thanks",
		"Yes please",
		"14:00 works",
	}
	for _, q := range questions {
		if !looksLikeQuestion(q) {
			t.Errorf("should be question: %q", q)
		}
	}
	for _, q := range nonQuestions {
		if looksLikeQuestion(q) {
			t.Errorf("should NOT be question: %q", q)
		}
	}
}
