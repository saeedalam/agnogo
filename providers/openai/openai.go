// Package openai provides an OpenAI model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/openai"
//	model := openai.New("sk-...", "gpt-4.1-mini")
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/saeedalam/agnogo"
)

// Provider implements agnogo.ModelProvider for OpenAI.
type Provider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     agnogo.ModelConfig
	client  *http.Client
}

// New creates an OpenAI provider.
//
//	model := openai.New("sk-...", "gpt-4.1-mini")
//	model := openai.New("sk-...", "gpt-4o", agnogo.ModelConfig{MaxTokens: 2000})
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 {
		c := cfgs[0]
		if c.MaxTokens > 0 { cfg.MaxTokens = c.MaxTokens }
		if c.Temperature > 0 { cfg.Temperature = c.Temperature }
		if c.Timeout > 0 { cfg.Timeout = c.Timeout }
	}
	return &Provider{
		apiKey: apiKey, model: model,
		baseURL: "https://api.openai.com/v1",
		cfg: cfg, client: &http.Client{Timeout: cfg.Timeout},
	}
}

// WithBaseURL sets a custom base URL (for proxies, Azure, etc.)
func (p *Provider) WithBaseURL(url string) *Provider {
	p.baseURL = url
	return p
}

func (p *Provider) ChatCompletion(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
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
		"model": p.model, "messages": oaiMsgs,
		"max_tokens": p.cfg.MaxTokens, "temperature": p.cfg.Temperature,
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, truncate(string(data), 300))
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
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	choice := result.Choices[0].Message
	mr := &agnogo.ModelResponse{}
	if choice.Content != nil {
		mr.Text = *choice.Content
	}
	for _, tc := range choice.ToolCalls {
		mr.ToolCalls = append(mr.ToolCalls, agnogo.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
		})
	}
	if result.Usage != nil {
		mr.Usage = &agnogo.Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
			TotalTokens:  result.Usage.TotalTokens,
		}
	}
	return mr, nil
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "…"
}
