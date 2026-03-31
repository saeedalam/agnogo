package agnogo

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"time"
)

// RetryConfig controls retry behavior for model calls.
// Follows Agno's pattern: retries with exponential backoff.
type RetryConfig struct {
	MaxRetries         int           // max retry attempts (default 0 = no retry)
	InitialDelay       time.Duration // delay before first retry (default 1s)
	ExponentialBackoff bool          // double delay each retry (default true)
	MaxDelay           time.Duration // cap on delay (default 30s)
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:         3,
		InitialDelay:       time.Second,
		ExponentialBackoff: true,
		MaxDelay:           30 * time.Second,
	}
}

// retryModelCall wraps a model call with retry logic.
func retryModelCall(ctx context.Context, cfg RetryConfig, fn func() (*ModelResponse, error)) (*ModelResponse, error) {
	var lastErr error
	attempts := cfg.MaxRetries + 1 // first attempt + retries

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			// If the error is a ProviderError and not retryable, stop immediately.
			if !IsRetryable(lastErr) {
				var pe *ProviderError
				if errors.As(lastErr, &pe) {
					slog.Debug("agnogo: not retrying non-retryable error", "status", pe.StatusCode)
					return nil, lastErr
				}
			}

			// Use Retry-After if available, otherwise exponential backoff.
			delay := RetryAfter(lastErr)
			if delay == 0 {
				delay = cfg.InitialDelay
				if cfg.ExponentialBackoff {
					delay = time.Duration(float64(cfg.InitialDelay) * math.Pow(2, float64(attempt-1)))
				}
				if delay > cfg.MaxDelay {
					delay = cfg.MaxDelay
				}
			}
			slog.Info("agnogo: retrying", "attempt", attempt+1, "of", cfg.MaxRetries+1, "in", delay.Round(time.Millisecond))

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := fn()
		if err == nil {
			return resp, nil
		}
		lastErr = err
		msg := err.Error()
		if len(msg) > 120 {
			msg = msg[:120] + "..."
		}
		slog.Warn("agnogo: attempt failed", "attempt", attempt+1, "error", msg)
	}

	return nil, lastErr
}
