package agnogo

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProviderErrorFormat(t *testing.T) {
	pe := &ProviderError{
		Provider:   "openai",
		StatusCode: 500,
		Message:    "internal server error",
	}
	got := pe.Error()
	want := "openai 500: internal server error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestProviderErrorRetryAfter(t *testing.T) {
	pe := &ProviderError{
		Provider:   "anthropic",
		StatusCode: 429,
		Message:    "rate limited",
		RetryAfter: 30 * time.Second,
	}
	got := pe.Error()
	if !strings.Contains(got, "retry after") {
		t.Errorf("Error() = %q, expected to contain 'retry after'", got)
	}
	if !strings.Contains(got, "30s") {
		t.Errorf("Error() = %q, expected to contain '30s'", got)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"429 is retryable", &ProviderError{StatusCode: 429, Retryable: true}, true},
		{"500 is retryable", &ProviderError{StatusCode: 500, Retryable: true}, true},
		{"401 not retryable", &ProviderError{StatusCode: 401, Retryable: false}, false},
		{"403 not retryable", &ProviderError{StatusCode: 403, Retryable: false}, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"429 is rate limited", &ProviderError{StatusCode: 429}, true},
		{"500 is not rate limited", &ProviderError{StatusCode: 500}, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRateLimited(tt.err); got != tt.want {
				t.Errorf("IsRateLimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryAfterDuration(t *testing.T) {
	pe := &ProviderError{RetryAfter: 10 * time.Second}
	if got := RetryAfter(pe); got != 10*time.Second {
		t.Errorf("RetryAfter() = %v, want 10s", got)
	}
	if got := RetryAfter(errors.New("plain")); got != 0 {
		t.Errorf("RetryAfter(plain) = %v, want 0", got)
	}
}

func TestParseProviderErrorOpenAI(t *testing.T) {
	body := []byte(`{"error":{"message":"Rate limit exceeded","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
	headers := http.Header{}
	headers.Set("Retry-After", "5")

	pe := ParseProviderError("openai", 429, body, headers)
	if pe.Provider != "openai" {
		t.Errorf("Provider = %q, want 'openai'", pe.Provider)
	}
	if pe.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", pe.StatusCode)
	}
	if pe.Message != "Rate limit exceeded" {
		t.Errorf("Message = %q, want 'Rate limit exceeded'", pe.Message)
	}
	if pe.Code != "rate_limit_exceeded" {
		t.Errorf("Code = %q, want 'rate_limit_exceeded'", pe.Code)
	}
	if !pe.Retryable {
		t.Error("expected Retryable=true for 429")
	}
	if pe.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", pe.RetryAfter)
	}
}

func TestParseProviderErrorAnthropic(t *testing.T) {
	body := []byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	headers := http.Header{}

	pe := ParseProviderError("anthropic", 529, body, headers)
	if pe.Message != "Overloaded" {
		t.Errorf("Message = %q, want 'Overloaded'", pe.Message)
	}
	if pe.Code != "overloaded_error" {
		t.Errorf("Code = %q, want 'overloaded_error'", pe.Code)
	}
	if !pe.Retryable {
		t.Error("expected Retryable=true for 529")
	}
}

func TestParseProviderErrorFallback(t *testing.T) {
	body := []byte(`this is not json`)
	headers := http.Header{}

	pe := ParseProviderError("gemini", 400, body, headers)
	if pe.Message != "this is not json" {
		t.Errorf("Message = %q, want raw body", pe.Message)
	}
	if pe.Retryable {
		t.Error("expected Retryable=false for 400")
	}
}

func TestToolErrorFormat(t *testing.T) {
	te := &ToolError{Tool: "web_search", Message: "timeout"}
	got := te.Error()
	want := "tool web_search: timeout"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestRetrySkipsPermanentErrors(t *testing.T) {
	callCount := 0
	fn := func() (*ModelResponse, error) {
		callCount++
		return nil, &ProviderError{
			Provider:   "openai",
			StatusCode: 401,
			Message:    "invalid api key",
			Retryable:  false,
		}
	}

	cfg := RetryConfig{
		MaxRetries:         3,
		InitialDelay:       time.Millisecond,
		ExponentialBackoff: false,
		MaxDelay:           time.Millisecond,
	}

	_, err := retryModelCall(context.Background(), cfg, fn)
	if err == nil {
		t.Fatal("expected error")
	}
	// Should have called fn only once (initial attempt), then stopped on first retry check.
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not retry non-retryable errors)", callCount)
	}

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatal("expected ProviderError")
	}
	if pe.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", pe.StatusCode)
	}
}
