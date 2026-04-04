package agnogo

import (
	"context"
	"time"
)

// ModelProvider is the interface for LLM backends.
// Implement for: OpenAI, Anthropic, Gemini, Ollama, Grok, or any model.
//
// Built-in providers: providers/openai, providers/anthropic, providers/gemini, etc.
type ModelProvider interface {
	ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error)
}

// ModelResponse is the parsed LLM response.
type ModelResponse struct {
	Text      string
	ToolCalls []ToolCall
	Usage     *Usage  // token usage from the model (optional)
	Model     string  // which model produced this (for accurate cost estimation)
}

// Usage tracks token counts from a model call.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// RunMetrics tracks cumulative metrics for a single Run call.
type RunMetrics struct {
	RunID        string        `json:"run_id"`
	Duration     time.Duration `json:"duration"`
	ModelCalls   int           `json:"model_calls"`
	ToolCalls    int           `json:"tool_calls"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	TotalTokens  int           `json:"total_tokens"`
}

func (m *RunMetrics) addUsage(u *Usage) {
	if u == nil {
		return
	}
	m.InputTokens += u.InputTokens
	m.OutputTokens += u.OutputTokens
	m.TotalTokens += u.TotalTokens
}

// ModelConfig holds configurable model parameters.
type ModelConfig struct {
	MaxTokens   int
	Temperature float64
	Timeout     time.Duration
}

// DefaultModelConfig returns sensible defaults.
func DefaultModelConfig() ModelConfig {
	return ModelConfig{MaxTokens: 1000, Temperature: 0.3, Timeout: 60 * time.Second}
}

// StructuredOutput forces the model to return JSON matching a schema.
// Used with Config.ResponseFormat.
type StructuredOutput struct {
	Name   string         // e.g. "booking_result"
	Schema map[string]any // JSON Schema definition
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
