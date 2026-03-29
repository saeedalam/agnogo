package agnogo

import (
	"context"
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
			delay := cfg.InitialDelay
			if cfg.ExponentialBackoff {
				delay = time.Duration(float64(cfg.InitialDelay) * math.Pow(2, float64(attempt-1)))
			}
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
			slog.Debug("agnogo: retrying model call", "attempt", attempt+1, "delay", delay)

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
		slog.Warn("agnogo: model call failed", "attempt", attempt+1, "error", err)
	}

	return nil, lastErr
}
