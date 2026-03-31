package agnogo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ── Ask Tests ───────────────────────────────────────────
// Note: errModel is defined in resilience_test.go (same package).

func TestAsk(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "Paris is the capital of France."},
	}}
	a := New(Config{Model: model, Instructions: "Answer questions."})

	answer, err := a.Ask(context.Background(), "What is the capital of France?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "Paris is the capital of France." {
		t.Errorf("answer = %q", answer)
	}
}

func TestAskError(t *testing.T) {
	model := &errModel{err: errors.New("API rate limit exceeded")}
	a := New(Config{Model: model})

	_, err := a.Ask(context.Background(), "Hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API rate limit exceeded") {
		t.Errorf("error = %q, want it to contain 'API rate limit exceeded'", err.Error())
	}
}

func TestAskStream(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "Hello World"},
	}}
	a := New(Config{Model: model})

	ch := a.AskStream(context.Background(), "Hi")

	var text string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		text += chunk.Text
	}

	// RunStream splits on words and joins with spaces.
	if text != "Hello World" {
		t.Errorf("streamed text = %q, want %q", text, "Hello World")
	}
}

func TestAskStructured(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	model := &mockModel{responses: []ModelResponse{
		{Text: `{"name":"Erik","age":30}`},
	}}
	a := New(Config{Model: model})

	var result Person
	err := AskStructured(context.Background(), a, "Who is Erik?", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Erik" {
		t.Errorf("name = %q", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("age = %d", result.Age)
	}
}
