package agnogo

import (
	"context"
	"errors"
	"testing"
	"time"
)

// errModel returns an error on every call.
type errModel struct{ err error }

func (m *errModel) ChatCompletion(_ context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	return nil, m.err
}

// ModelProviderFunc adapts a plain function to the ModelProvider interface.
type ModelProviderFunc func(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error)

func (f ModelProviderFunc) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	return f(ctx, messages, tools)
}

// --- Fallback ---

func TestFallbackPrimarySucceeds(t *testing.T) {
	primary := &mockModel{responses: []ModelResponse{{Text: "primary"}}}
	secondary := &mockModel{responses: []ModelResponse{{Text: "secondary"}}}

	fb := Fallback(primary, secondary)
	resp, err := fb.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "primary" {
		t.Errorf("text = %q, want 'primary'", resp.Text)
	}
	if secondary.callCount != 0 {
		t.Error("secondary should not have been called")
	}
}

func TestFallbackSecondaryUsed(t *testing.T) {
	primary := &errModel{err: errors.New("primary down")}
	secondary := &mockModel{responses: []ModelResponse{{Text: "secondary"}}}

	fb := Fallback(primary, secondary)
	resp, err := fb.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "secondary" {
		t.Errorf("text = %q, want 'secondary'", resp.Text)
	}
}

// --- MultiProvider ---

func TestMultiProviderFirstSucceeds(t *testing.T) {
	p1 := &mockModel{responses: []ModelResponse{{Text: "first"}}}
	p2 := &mockModel{responses: []ModelResponse{{Text: "second"}}}

	mp := MultiProvider(p1, p2)
	resp, err := mp.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "first" {
		t.Errorf("text = %q, want 'first'", resp.Text)
	}
	if p2.callCount != 0 {
		t.Error("second provider should not have been called")
	}
}

func TestMultiProviderFallsThrough(t *testing.T) {
	p1 := &errModel{err: errors.New("p1 down")}
	p2 := &mockModel{responses: []ModelResponse{{Text: "second"}}}

	mp := MultiProvider(p1, p2)
	resp, err := mp.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "second" {
		t.Errorf("text = %q, want 'second'", resp.Text)
	}
}

func TestMultiProviderAllFail(t *testing.T) {
	p1 := &errModel{err: errors.New("p1 down")}
	p2 := &errModel{err: errors.New("p2 down")}

	mp := MultiProvider(p1, p2)
	_, err := mp.ChatCompletion(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
	if !errors.Is(err, p2.err) {
		t.Errorf("expected last error, got: %v", err)
	}
}

// --- CircuitBreaker ---

func TestCircuitBreakerClosedState(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	cb := CircuitBreaker(inner, WithFailureThreshold(2), WithResetTimeout(50*time.Millisecond))

	resp, err := cb.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q, want 'ok'", resp.Text)
	}
}

func TestCircuitBreakerOpens(t *testing.T) {
	inner := &errModel{err: errors.New("fail")}
	cb := CircuitBreaker(inner, WithFailureThreshold(2), WithResetTimeout(50*time.Millisecond))

	// Two failures should open the circuit.
	cb.ChatCompletion(context.Background(), nil, nil)
	cb.ChatCompletion(context.Background(), nil, nil)

	// Third call should get errCircuitOpen.
	_, err := cb.ChatCompletion(context.Background(), nil, nil)
	if !errors.Is(err, errCircuitOpen) {
		t.Errorf("expected errCircuitOpen, got: %v", err)
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	inner := &errModel{err: errors.New("fail")}
	cb := CircuitBreaker(inner, WithFailureThreshold(2), WithResetTimeout(50*time.Millisecond))

	// Open the circuit.
	cb.ChatCompletion(context.Background(), nil, nil)
	cb.ChatCompletion(context.Background(), nil, nil)

	// Wait for reset timeout.
	time.Sleep(60 * time.Millisecond)

	// Next call should be allowed (half-open probe), but it will fail
	// because inner still errors.
	_, err := cb.ChatCompletion(context.Background(), nil, nil)
	// The probe request goes through to the inner provider, which errors.
	if err == nil {
		t.Fatal("expected error from probe request")
	}
	// It should be the inner error, not errCircuitOpen.
	if errors.Is(err, errCircuitOpen) {
		t.Error("half-open should allow probe, not return errCircuitOpen")
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	// Use a model that fails twice then succeeds.
	callCount := 0
	inner := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	failThenSucceed := ModelProviderFunc(func(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
		callCount++
		if callCount <= 2 {
			return nil, errors.New("fail")
		}
		return inner.ChatCompletion(ctx, messages, tools)
	})

	cb := CircuitBreaker(failThenSucceed, WithFailureThreshold(2), WithResetTimeout(50*time.Millisecond))

	// Two failures open the circuit.
	cb.ChatCompletion(context.Background(), nil, nil)
	cb.ChatCompletion(context.Background(), nil, nil)

	// Wait for reset timeout so it goes to half-open.
	time.Sleep(60 * time.Millisecond)

	// Probe succeeds, should reset to closed.
	resp, err := cb.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("probe should succeed: %v", err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q, want 'ok'", resp.Text)
	}

	// Subsequent call should also work (closed state).
	resp, err = cb.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("post-reset call failed: %v", err)
	}
}

// --- RateLimiter ---

func TestRateLimiterAllowsRequests(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{
		{Text: "r1"}, {Text: "r2"}, {Text: "r3"},
	}}
	rl := RateLimiter(inner, 60) // 60 RPM = plenty of tokens

	for i := 0; i < 3; i++ {
		resp, err := rl.ChatCompletion(context.Background(), nil, nil)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if resp.Text == "" {
			t.Errorf("call %d: empty text", i)
		}
	}
}

func TestRateLimiterContextCancel(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	// 1 RPM with bucket size 1, consume the single token first.
	rl := RateLimiter(inner, 1)
	rl.ChatCompletion(context.Background(), nil, nil) // consume the token

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := rl.ChatCompletion(ctx, nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// --- TimeoutProvider ---

func TestTimeoutProviderSuccess(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{{Text: "fast"}}}
	tp := TimeoutProvider(inner, 1*time.Second)

	resp, err := tp.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "fast" {
		t.Errorf("text = %q, want 'fast'", resp.Text)
	}
}

func TestTimeoutProviderExpires(t *testing.T) {
	inner := ModelProviderFunc(func(ctx context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
		select {
		case <-time.After(5 * time.Second):
			return &ModelResponse{Text: "slow"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	tp := TimeoutProvider(inner, 50*time.Millisecond)

	_, err := tp.ChatCompletion(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// --- RateLimiter Close ---

func TestRateLimiterClose(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	rl := RateLimiter(inner, 60)

	// Use it once to verify it works.
	resp, err := rl.ChatCompletion(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok" {
		t.Errorf("text = %q, want 'ok'", resp.Text)
	}

	// Close should not panic or block.
	c, ok := rl.(Closeable)
	if !ok {
		t.Fatal("RateLimiter should implement Closeable")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestCloseProviderNested(t *testing.T) {
	inner1 := &mockModel{responses: []ModelResponse{{Text: "a"}}}
	inner2 := &mockModel{responses: []ModelResponse{{Text: "b"}}}
	rl := RateLimiter(inner1, 60)
	fb := Fallback(rl, inner2)

	if err := CloseProvider(fb); err != nil {
		t.Fatalf("CloseProvider error: %v", err)
	}

	// Verify the rate limiter's done channel is closed (goroutine stopped).
	rlp := rl.(*rateLimiterProvider)
	select {
	case <-rlp.done:
		// OK, channel is closed.
	default:
		t.Error("expected done channel to be closed after CloseProvider")
	}
}

func TestCloseProviderNoop(t *testing.T) {
	inner := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	// CloseProvider on a plain mockModel should do nothing and return nil.
	if err := CloseProvider(inner); err != nil {
		t.Fatalf("CloseProvider on plain provider returned error: %v", err)
	}
}
