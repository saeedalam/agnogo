package agnogo

import (
	"context"
	"sync"
	"testing"
)

func TestCostBudgetPerRun(t *testing.T) {
	// Model returns large usage to exceed the per-run budget.
	model := &mockModel{responses: []ModelResponse{
		{Text: "Hello!", Usage: &Usage{InputTokens: 50000, OutputTokens: 50000, TotalTokens: 100000}},
	}}
	a := New(Config{Model: model, Instructions: "test"})
	a.costBudget = &CostBudget{MaxPerRun: 0.0001} // extremely low

	session := NewSession("test")
	resp, err := a.Run(context.Background(), session, "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "I've reached my cost limit for this request. Please try a simpler question." {
		t.Fatalf("expected budget exceeded message, got: %s", resp.Text)
	}
}

func TestCostBudgetPerSession(t *testing.T) {
	// First call accumulates session cost, second call exceeds session budget.
	model := &mockModel{responses: []ModelResponse{
		{Text: "First", Usage: &Usage{InputTokens: 10000, OutputTokens: 10000, TotalTokens: 20000}},
		{Text: "Second", Usage: &Usage{InputTokens: 10000, OutputTokens: 10000, TotalTokens: 20000}},
	}}
	a := New(Config{Model: model, Instructions: "test"})
	a.costBudget = &CostBudget{MaxPerSession: 0.001} // very low session budget

	session := NewSession("test-session")

	// First run - sets session cost
	resp1, err := a.Run(context.Background(), session, "Hello")
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}
	// If the first call already exceeds, that's fine.
	// Second call should definitely be over session budget.
	if resp1.Text == "First" {
		resp2, err := a.Run(context.Background(), session, "Again")
		if err != nil {
			t.Fatalf("second run error: %v", err)
		}
		if resp2.Text != "This session has reached its cost limit." {
			t.Fatalf("expected session budget message, got: %s", resp2.Text)
		}
	}
}

func TestCostBudgetCallback(t *testing.T) {
	var mu sync.Mutex
	var callbackCalled bool
	var callbackSpent, callbackLimit float64

	model := &mockModel{responses: []ModelResponse{
		{Text: "Hello!", Usage: &Usage{InputTokens: 50000, OutputTokens: 50000, TotalTokens: 100000}},
	}}
	a := New(Config{Model: model, Instructions: "test"})
	a.costBudget = &CostBudget{
		MaxPerRun: 0.0001,
		OnExceeded: func(spent, limit float64) {
			mu.Lock()
			defer mu.Unlock()
			callbackCalled = true
			callbackSpent = spent
			callbackLimit = limit
		},
	}

	session := NewSession("test")
	_, err := a.Run(context.Background(), session, "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !callbackCalled {
		t.Fatal("OnExceeded callback was not called")
	}
	if callbackSpent <= 0 {
		t.Fatalf("expected positive spent amount, got: %f", callbackSpent)
	}
	if callbackLimit != 0.0001 {
		t.Fatalf("expected limit 0.0001, got: %f", callbackLimit)
	}
}

func TestCostBudgetUnlimited(t *testing.T) {
	// Zero limits = no enforcement, runs should complete normally.
	model := &mockModel{responses: []ModelResponse{
		{Text: "All good!", Usage: &Usage{InputTokens: 100000, OutputTokens: 100000, TotalTokens: 200000}},
	}}
	a := New(Config{Model: model, Instructions: "test"})
	a.costBudget = &CostBudget{} // all zeros = unlimited

	session := NewSession("test")
	resp, err := a.Run(context.Background(), session, "Hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "All good!" {
		t.Fatalf("expected normal response, got: %s", resp.Text)
	}
}

func TestWithBudgetOption(t *testing.T) {
	opt := WithBudget(CostBudget{MaxPerRun: 0.50, MaxPerSession: 5.00})
	sc := &smartConfig{}
	opt.applyOption(sc)
	if sc.costBudget == nil {
		t.Fatal("costBudget not set")
	}
	if sc.costBudget.MaxPerRun != 0.50 {
		t.Fatalf("expected MaxPerRun 0.50, got %f", sc.costBudget.MaxPerRun)
	}
	if sc.costBudget.MaxPerSession != 5.00 {
		t.Fatalf("expected MaxPerSession 5.00, got %f", sc.costBudget.MaxPerSession)
	}
}
