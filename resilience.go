package agnogo

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// --- Fallback ---

// fallbackProvider tries a primary provider, falling back to a secondary on error.
type fallbackProvider struct {
	primary   ModelProvider
	secondary ModelProvider
}

// Fallback wraps two providers. Uses primary; falls back to secondary on error.
func Fallback(primary, secondary ModelProvider) ModelProvider {
	return &fallbackProvider{primary: primary, secondary: secondary}
}

func (f *fallbackProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	resp, err := f.primary.ChatCompletion(ctx, messages, tools)
	if err == nil {
		return resp, nil
	}
	slog.Warn("agnogo: primary provider failed, falling back", "error", err)
	return f.secondary.ChatCompletion(ctx, messages, tools)
}

// --- MultiProvider ---

// multiProvider tries providers in order until one succeeds.
type multiProvider struct {
	providers []ModelProvider
}

// MultiProvider tries providers in order until one succeeds.
// If all fail, the last error is returned.
func MultiProvider(providers ...ModelProvider) ModelProvider {
	return &multiProvider{providers: providers}
}

func (m *multiProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	var lastErr error
	for i, p := range m.providers {
		resp, err := p.ChatCompletion(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		slog.Warn("agnogo: provider failed", "index", i, "error", err)
	}
	if lastErr == nil {
		lastErr = errors.New("agnogo: no providers configured")
	}
	return nil, lastErr
}

// --- CircuitBreaker ---

// cbState represents the circuit breaker state.
type cbState int

const (
	cbClosed   cbState = iota // normal operation
	cbOpen                    // rejecting requests
	cbHalfOpen                // allowing one probe request
)

// circuitBreakerConfig holds circuit breaker settings.
type circuitBreakerConfig struct {
	failureThreshold int
	resetTimeout     time.Duration
}

// CBOption configures a CircuitBreaker.
type CBOption func(*circuitBreakerConfig)

// WithFailureThreshold sets the number of consecutive failures before the circuit opens.
// Default is 5.
func WithFailureThreshold(n int) CBOption {
	return func(c *circuitBreakerConfig) {
		c.failureThreshold = n
	}
}

// WithResetTimeout sets how long the circuit stays open before moving to half-open.
// Default is 30 seconds.
func WithResetTimeout(d time.Duration) CBOption {
	return func(c *circuitBreakerConfig) {
		c.resetTimeout = d
	}
}

// circuitBreakerProvider wraps a provider with the circuit breaker pattern.
type circuitBreakerProvider struct {
	provider     ModelProvider
	cfg          circuitBreakerConfig
	mu           sync.Mutex
	state        cbState
	failures     int
	lastFailTime time.Time
}

// CircuitBreaker wraps a provider with the circuit breaker pattern.
//
// States: closed (normal) -> open (after N failures, rejects immediately) ->
// half-open (after timeout, allows one probe request) -> closed (on success)
// or open (on failure).
func CircuitBreaker(provider ModelProvider, opts ...CBOption) ModelProvider {
	cfg := circuitBreakerConfig{
		failureThreshold: 5,
		resetTimeout:     30 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &circuitBreakerProvider{
		provider: provider,
		cfg:      cfg,
		state:    cbClosed,
	}
}

var errCircuitOpen = errors.New("agnogo: circuit breaker is open")

func (cb *circuitBreakerProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	cb.mu.Lock()
	switch cb.state {
	case cbOpen:
		if time.Since(cb.lastFailTime) >= cb.cfg.resetTimeout {
			// Transition to half-open: allow one probe request.
			cb.state = cbHalfOpen
			slog.Info("agnogo: circuit breaker half-open, allowing probe request")
		} else {
			cb.mu.Unlock()
			return nil, errCircuitOpen
		}
	case cbHalfOpen:
		// Already probing; reject additional requests while probe is in flight.
		cb.mu.Unlock()
		return nil, errCircuitOpen
	}
	cb.mu.Unlock()

	resp, err := cb.provider.ChatCompletion(ctx, messages, tools)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()
		if cb.state == cbHalfOpen || cb.failures >= cb.cfg.failureThreshold {
			cb.state = cbOpen
			slog.Warn("agnogo: circuit breaker opened", "failures", cb.failures)
		}
		return nil, err
	}

	// Success: reset to closed.
	cb.failures = 0
	cb.state = cbClosed
	return resp, nil
}

// --- RateLimiter ---

// rateLimiterProvider wraps a provider with token bucket rate limiting.
type rateLimiterProvider struct {
	provider  ModelProvider
	tokens    chan struct{}
	ticker    *time.Ticker
	done      chan struct{}
	closeOnce sync.Once
}

// RateLimiter wraps a provider with token bucket rate limiting.
// It allows up to requestsPerMinute requests per minute, using a token bucket
// that is replenished at a steady rate. Calls block until a token is available
// or the context is cancelled.
//
// The returned provider implements Closeable. Call CloseProvider (or Close
// directly) when done to stop the background replenishment goroutine.
func RateLimiter(provider ModelProvider, requestsPerMinute int) ModelProvider {
	tokens := make(chan struct{}, requestsPerMinute)
	// Fill the bucket initially.
	for range requestsPerMinute {
		tokens <- struct{}{}
	}

	// Replenish tokens at a steady rate.
	interval := time.Minute / time.Duration(requestsPerMinute)
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				select {
				case tokens <- struct{}{}:
				default:
					// Bucket is full; discard token.
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return &rateLimiterProvider{
		provider: provider,
		tokens:   tokens,
		ticker:   ticker,
		done:     done,
	}
}

// Close stops the background token replenishment goroutine.
func (r *rateLimiterProvider) Close() error {
	r.closeOnce.Do(func() { close(r.done) })
	return nil
}

func (r *rateLimiterProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	select {
	case <-r.tokens:
		// Token acquired.
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return r.provider.ChatCompletion(ctx, messages, tools)
}

// --- TimeoutProvider ---

// timeoutProvider wraps a provider with a per-request timeout.
type timeoutProvider struct {
	provider ModelProvider
	timeout  time.Duration
}

// TimeoutProvider wraps a provider with a per-request timeout.
// Each call to ChatCompletion is given a derived context that expires after
// the specified duration.
func TimeoutProvider(provider ModelProvider, timeout time.Duration) ModelProvider {
	return &timeoutProvider{provider: provider, timeout: timeout}
}

func (t *timeoutProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.provider.ChatCompletion(ctx, messages, tools)
}

// --- Closeable ---

// Closeable is implemented by providers that hold resources (goroutines, connections).
type Closeable interface {
	Close() error
}

// CloseProvider closes a provider if it implements Closeable.
// For wrapped providers (Fallback, MultiProvider), it closes all inner providers.
func CloseProvider(p ModelProvider) error {
	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	switch v := p.(type) {
	case *fallbackProvider:
		record(CloseProvider(v.primary))
		record(CloseProvider(v.secondary))
	case *multiProvider:
		for _, inner := range v.providers {
			record(CloseProvider(inner))
		}
	case *circuitBreakerProvider:
		record(CloseProvider(v.provider))
	case *rateLimiterProvider:
		record(v.Close())
		record(CloseProvider(v.provider))
	case *timeoutProvider:
		record(CloseProvider(v.provider))
	default:
		if c, ok := p.(Closeable); ok {
			record(c.Close())
		}
	}

	return firstErr
}
