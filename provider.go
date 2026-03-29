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
