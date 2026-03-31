package agnogo

import (
	"context"
	"strings"
	"testing"
	"time"
)

// slowModel sleeps before returning, respects context cancellation.
type slowModel struct {
	delay time.Duration
	text  string
}

func (m *slowModel) ChatCompletion(ctx context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	select {
	case <-time.After(m.delay):
		return &ModelResponse{Text: m.text}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestPipelineThen(t *testing.T) {
	a1 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "step1"}}},
		Instructions: "Agent 1",
	})
	a2 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "step2"}}},
		Instructions: "Agent 2",
	})
	a3 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "step3"}}},
		Instructions: "Agent 3",
	})

	session := NewSession("pipeline-test")
	resp, err := a1.Then(a2).Then(a3).Run(context.Background(), session, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "step3" {
		t.Errorf("expected final output 'step3', got %q", resp.Text)
	}
}

func TestFanOutAll(t *testing.T) {
	a1 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "output-A"}}},
		Instructions: "Agent A",
	})
	a2 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "output-B"}}},
		Instructions: "Agent B",
	})
	a3 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "output-C"}}},
		Instructions: "Agent C",
	})

	session := NewSession("fanout-test")
	resp, err := All(a1, a2, a3).Run(context.Background(), session, "input")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"output-A", "output-B", "output-C"} {
		if !strings.Contains(resp.Text, want) {
			t.Errorf("merged result missing %q, got %q", want, resp.Text)
		}
	}
}

func TestFanOutCustomMerge(t *testing.T) {
	a1 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "alpha"}}},
		Instructions: "A1",
	})
	a2 := New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: "beta"}}},
		Instructions: "A2",
	})

	merge := func(outputs []string) string {
		return "CUSTOM:" + strings.Join(outputs, ",")
	}

	session := NewSession("fanout-merge-test")
	resp, err := All(a1, a2).WithMerge(merge).Run(context.Background(), session, "go")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "CUSTOM:alpha,beta" {
		t.Errorf("expected custom merge, got %q", resp.Text)
	}
}

func TestRaceFirstWins(t *testing.T) {
	fast := New(Config{
		Model:        &slowModel{delay: 10 * time.Millisecond, text: "fast-wins"},
		Instructions: "Fast",
	})
	slow := New(Config{
		Model:        &slowModel{delay: 5 * time.Second, text: "slow-done"},
		Instructions: "Slow",
	})

	session := NewSession("race-test")
	resp, err := Race(fast, slow).Run(context.Background(), session, "go")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "fast-wins" {
		t.Errorf("expected fast agent to win, got %q", resp.Text)
	}
}

func TestRaceAllFail(t *testing.T) {
	errAgent1 := New(Config{
		Model:        &errModel{err: context.DeadlineExceeded},
		Instructions: "Err1",
	})
	errAgent2 := New(Config{
		Model:        &errModel{err: context.DeadlineExceeded},
		Instructions: "Err2",
	})

	session := NewSession("race-fail-test")
	_, err := Race(errAgent1, errAgent2).Run(context.Background(), session, "go")
	if err == nil {
		t.Fatal("expected error when all race agents fail")
	}
	if !strings.Contains(err.Error(), "all race agents failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMapConcurrent(t *testing.T) {
	// Each call returns the input text uppercased via the mock.
	// We use 5 separate agents with mockModels that return predictable text.
	// Map uses a single agent, so we need a model that handles multiple calls.
	model := &mockModel{responses: []ModelResponse{
		{Text: "result-0"},
		{Text: "result-1"},
		{Text: "result-2"},
		{Text: "result-3"},
		{Text: "result-4"},
	}}

	a := New(Config{
		Model:        model,
		Instructions: "Map agent",
	})

	inputs := []string{"in-0", "in-1", "in-2", "in-3", "in-4"}
	results := Map(context.Background(), a, inputs, 2)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d] error: %v", i, r.Err)
		}
		if r.Input != inputs[i] {
			t.Errorf("result[%d].Input = %q, want %q", i, r.Input, inputs[i])
		}
		if r.Response == nil {
			t.Errorf("result[%d].Response is nil", i)
		}
	}
}
