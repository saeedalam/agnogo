package agnogo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ModelProvider is the interface for LLM backends.
// Implement this for any model: OpenAI, Anthropic, Gemini, Ollama, etc.
type ModelProvider interface {
	ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error)
}

// ModelResponse is the parsed response from the LLM.
type ModelResponse struct {
	Text      string
	ToolCalls []ToolCall
}

// ModelConfig holds configurable model parameters.
type ModelConfig struct {
	MaxTokens   int     // default 1000
	Temperature float64 // default 0.3
	Timeout     time.Duration // default 60s
}

// ── OpenAI ─────────────────────────────────────────────

// OpenAIProvider implements ModelProvider for OpenAI.
type OpenAIProvider struct {
	apiKey string
	model  string
	cfg    ModelConfig
	client *http.Client
}

// OpenAI creates an OpenAI model provider.
//
//	model := agnogo.OpenAI("sk-...", "gpt-4.1-mini")
//	model := agnogo.OpenAI("sk-...", "gpt-4.1-mini", agnogo.ModelConfig{MaxTokens: 2000})
func OpenAI(apiKey, model string, cfgs ...ModelConfig) *OpenAIProvider {
	cfg := ModelConfig{MaxTokens: 1000, Temperature: 0.3, Timeout: 60 * time.Second}
	if len(cfgs) > 0 {
		c := cfgs[0]
		if c.MaxTokens > 0 {
			cfg.MaxTokens = c.MaxTokens
		}
		if c.Temperature > 0 {
			cfg.Temperature = c.Temperature
		}
		if c.Timeout > 0 {
			cfg.Timeout = c.Timeout
		}
	}
	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (o *OpenAIProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	oaiMsgs := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if m.Name != "" && m.Role == "tool" {
			msg["tool_call_id"] = m.Name
		}
		if len(m.ToolCalls) > 0 {
			calls := make([]map[string]any, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				calls[i] = map[string]any{
					"id": tc.ID, "type": "function",
					"function": map[string]string{"name": tc.Name, "arguments": tc.Arguments},
				}
			}
			msg["tool_calls"] = calls
			delete(msg, "content")
		}
		oaiMsgs = append(oaiMsgs, msg)
	}

	body := map[string]any{
		"model": o.model, "messages": oaiMsgs,
		"max_tokens": o.cfg.MaxTokens, "temperature": o.cfg.Temperature,
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	choice := result.Choices[0].Message
	mr := &ModelResponse{}
	if choice.Content != nil {
		mr.Text = *choice.Content
	}
	for _, tc := range choice.ToolCalls {
		mr.ToolCalls = append(mr.ToolCalls, ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
		})
	}
	return mr, nil
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
