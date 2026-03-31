package agnogo

import (
	"context"
	"testing"
)

func TestHookExecutionOrder(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "hello"}}}

	var order []string

	hookA := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		order = append(order, "A-before")
		resp, err := next(ctx, a, s, msg)
		order = append(order, "A-after")
		return resp, err
	}
	hookB := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		order = append(order, "B-before")
		resp, err := next(ctx, a, s, msg)
		order = append(order, "B-after")
		return resp, err
	}

	a := New(Config{Model: model})
	a.hooks = []Hook{hookA, hookB}

	session := NewSession("hook-order")
	_, err := a.Run(context.Background(), session, "hi")
	if err != nil {
		t.Fatal(err)
	}

	// First hook is outermost: A wraps B wraps inner
	expected := []string{"A-before", "B-before", "B-after", "A-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestHookShortCircuit(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "should not reach"}}}

	authHook := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		// Short circuit: return without calling next
		return &Response{Text: "Unauthorized"}, nil
	}

	a := New(Config{Model: model})
	a.hooks = []Hook{authHook}

	session := NewSession("hook-short")
	resp, err := a.Run(context.Background(), session, "do something")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Unauthorized" {
		t.Errorf("text = %q, want %q", resp.Text, "Unauthorized")
	}
	if model.callCount != 0 {
		t.Errorf("model called %d times, want 0", model.callCount)
	}
}

func TestHookModifyResponse(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "original"}}}

	modifyHook := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		resp, err := next(ctx, a, s, msg)
		if err != nil {
			return resp, err
		}
		resp.Text = resp.Text + " [modified]"
		return resp, nil
	}

	a := New(Config{Model: model})
	a.hooks = []Hook{modifyHook}

	session := NewSession("hook-modify")
	resp, err := a.Run(context.Background(), session, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "original [modified]" {
		t.Errorf("text = %q, want %q", resp.Text, "original [modified]")
	}
}

func TestWithHooksOption(t *testing.T) {
	// Verify WithHooks option sets hooks on the smartConfig
	var order []string

	hook1 := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		order = append(order, "hook1")
		return next(ctx, a, s, msg)
	}

	// Use WithHooks to build the option, then apply it to a smartConfig
	opt := WithHooks(hook1)
	sc := &smartConfig{}
	opt.applyOption(sc)

	if len(sc.hooks) != 1 {
		t.Errorf("hooks count = %d, want 1", len(sc.hooks))
	}
}
