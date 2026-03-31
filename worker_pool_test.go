package agnogo

import (
	"context"
	"fmt"
	"testing"
)

func TestWorkerPoolBasic(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 5; i++ {
		model.responses = append(model.responses, ModelResponse{Text: fmt.Sprintf("reply-%d", i)})
	}

	a := New(Config{Model: model, Instructions: "test"})
	pool := NewWorkerPool(a, 2)
	pool.Start(context.Background())

	for i := 0; i < 3; i++ {
		pool.Submit(WorkerTask{ID: fmt.Sprintf("t%d", i), Message: fmt.Sprintf("msg-%d", i)})
	}

	results := make([]WorkerResult, 0, 3)
	for i := 0; i < 3; i++ {
		results = append(results, <-pool.Results())
	}
	pool.Stop()

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("task %s error: %v", r.TaskID, r.Err)
		}
		if r.Response == nil {
			t.Errorf("task %s: nil response", r.TaskID)
		}
		if r.Duration == 0 {
			t.Errorf("task %s: zero duration", r.TaskID)
		}
	}
}

func TestBatch(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 10; i++ {
		model.responses = append(model.responses, ModelResponse{Text: "ok"})
	}

	a := New(Config{Model: model, Instructions: "test"})

	tasks := make([]WorkerTask, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = WorkerTask{ID: fmt.Sprintf("b%d", i), Message: fmt.Sprintf("msg-%d", i)}
	}

	results := Batch(context.Background(), a, tasks, 2)

	if len(results) != 5 {
		t.Fatalf("got %d results, want 5", len(results))
	}

	// Verify sorted by TaskID
	for i := 1; i < len(results); i++ {
		if results[i].TaskID < results[i-1].TaskID {
			t.Errorf("results not sorted: %s < %s", results[i].TaskID, results[i-1].TaskID)
		}
	}
}

func TestBatchEmpty(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	a := New(Config{Model: model, Instructions: "test"})

	results := Batch(context.Background(), a, nil, 2)
	if results != nil {
		t.Errorf("expected nil for empty task list, got %v", results)
	}
}

func TestWorkerPoolWithMockModel(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 4; i++ {
		model.responses = append(model.responses, ModelResponse{Text: "done"})
	}

	a := New(Config{Model: model, Instructions: "worker test"})

	tasks := []WorkerTask{
		{ID: "w1", Message: "hello"},
		{ID: "w2", Message: "world"},
		{ID: "w3", Message: "test"},
		{ID: "w4", Message: "four"},
	}

	results := Batch(context.Background(), a, tasks, 2)

	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("task %s: unexpected error: %v", r.TaskID, r.Err)
		}
		if r.Response == nil || r.Response.Text == "" {
			t.Errorf("task %s: empty response", r.TaskID)
		}
	}
}
