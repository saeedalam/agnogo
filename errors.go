package agnogo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrToolNotFound    = errors.New("tool not found")
	ErrMaxLoops        = errors.New("max tool call loops reached")
	ErrApprovalNeeded  = errors.New("human approval required")
	ErrModelFailed     = errors.New("model call failed")
	ErrGuardrailBlock  = errors.New("blocked by guardrail")
)

// ProviderError is a structured error from an LLM provider.
type ProviderError struct {
	Provider   string        // "openai", "anthropic", "gemini", etc.
	StatusCode int           // HTTP status code
	Message    string        // human-readable error message
	Code       string        // provider-specific error code (e.g. "rate_limit_exceeded")
	Retryable  bool          // whether this error is worth retrying
	RetryAfter time.Duration // suggested delay before retry (from Retry-After header)
	Err        error         // original wrapped error
}

func (e *ProviderError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s %d: %s (retry after %s)", e.Provider, e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("%s %d: %s", e.Provider, e.StatusCode, e.Message)
}

func (e *ProviderError) Unwrap() error { return e.Err }

// ToolError is a structured error from a tool execution.
type ToolError struct {
	Tool    string
	Message string
	Err     error
}

func (e *ToolError) Error() string { return fmt.Sprintf("tool %s: %s", e.Tool, e.Message) }
func (e *ToolError) Unwrap() error { return e.Err }

// IsRetryable returns true if the error is a retryable provider error.
func IsRetryable(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.Retryable
	}
	return false
}

// IsRateLimited returns true if the error is a 429 rate limit error.
func IsRateLimited(err error) bool {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.StatusCode == 429
	}
	return false
}

// RetryAfter returns the suggested retry delay, or 0 if none.
func RetryAfter(err error) time.Duration {
	var pe *ProviderError
	if errors.As(err, &pe) {
		return pe.RetryAfter
	}
	return 0
}

// classifyHTTPStatus returns whether a status code represents a retryable error.
func classifyHTTPStatus(code int) bool {
	// Retryable: 429 (rate limit), 500, 502, 503, 504 (server errors)
	// Not retryable: 400, 401, 403, 404 (client errors)
	return code == 429 || code >= 500
}

// ParseProviderError creates a ProviderError from an HTTP response.
// It parses common error JSON formats from OpenAI, Anthropic, and Gemini.
func ParseProviderError(provider string, statusCode int, body []byte, headers http.Header) *ProviderError {
	pe := &ProviderError{
		Provider:   provider,
		StatusCode: statusCode,
		Retryable:  classifyHTTPStatus(statusCode),
	}

	// Parse Retry-After header
	if ra := headers.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			pe.RetryAfter = time.Duration(secs) * time.Second
		}
	}

	// Try to parse JSON error body
	// OpenAI format: {"error": {"message": "...", "type": "...", "code": "..."}}
	// Anthropic format: {"type": "error", "error": {"type": "...", "message": "..."}}
	// Gemini format: {"error": {"message": "...", "status": "...", "code": N}}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"` // string or int depending on provider
		} `json:"error"`
		Type string `json:"type"` // Anthropic uses top-level "type": "error"
	}
	if json.Unmarshal(body, &parsed) == nil && parsed.Error.Message != "" {
		pe.Message = parsed.Error.Message
		pe.Code = parsed.Error.Type
		if codeStr, ok := parsed.Error.Code.(string); ok {
			pe.Code = codeStr
		}
	} else {
		// Fallback: use raw body (truncated)
		msg := string(body)
		if len(msg) > 500 {
			msg = msg[:500] + "..."
		}
		pe.Message = msg
	}

	return pe
}
