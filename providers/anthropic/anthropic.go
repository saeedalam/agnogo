// Package anthropic provides a Claude model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/anthropic"
//	model := anthropic.New("sk-ant-...", "claude-sonnet-4-5-20250514")
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saeedalam/agnogo"
)

// sharedAnthropicTransport pools connections across Anthropic provider instances.
var sharedAnthropicTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// Provider implements agnogo.ModelProvider for Anthropic Claude.
type Provider struct {
	apiKey string
	model  string
	cfg    agnogo.ModelConfig
	client *http.Client
}

// New creates a Claude provider.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 {
		c := cfgs[0]
		if c.MaxTokens > 0 { cfg.MaxTokens = c.MaxTokens }
		if c.Temperature > 0 { cfg.Temperature = c.Temperature }
		if c.Timeout > 0 { cfg.Timeout = c.Timeout }
	}
	return &Provider{
		apiKey: apiKey, model: model, cfg: cfg,
		client: &http.Client{Timeout: cfg.Timeout, Transport: sharedAnthropicTransport},
	}
}

func (p *Provider) ChatCompletion(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
	// Separate system prompt from messages (Anthropic API requires it at top level)
	var systemPrompt string
	var apiMsgs []map[string]any
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt += m.Content + "\n"
			continue
		}
		msg := map[string]any{"role": m.Role, "content": m.Content}
		// Map tool results to Anthropic format
		if m.Role == "tool" {
			msg = map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":         "tool_result",
					"tool_use_id":  m.Name,
					"content":      m.Content,
				}},
			}
		}
		if len(m.ToolCalls) > 0 {
			content := make([]map[string]any, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				var args any
				json.Unmarshal([]byte(tc.Arguments), &args)
				content[i] = map[string]any{
					"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": args,
				}
			}
			msg = map[string]any{"role": "assistant", "content": content}
		}
		apiMsgs = append(apiMsgs, msg)
	}

	body := map[string]any{
		"model":      p.model,
		"max_tokens": p.cfg.MaxTokens,
		"messages":   apiMsgs,
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if len(tools) > 0 {
		// Convert OpenAI tool format to Anthropic format
		anthropicTools := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			fn, ok := t["function"].(map[string]any)
			if !ok {
				continue
			}
			at := map[string]any{"name": fn["name"], "description": fn["description"]}
			if params, ok := fn["parameters"]; ok {
				at["input_schema"] = params
			} else {
				at["input_schema"] = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			anthropicTools = append(anthropicTools, at)
		}
		body["tools"] = anthropicTools
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, agnogo.ParseProviderError("anthropic", resp.StatusCode, data, resp.Header)
	}

	var result struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("anthropic: parse response JSON: %w", err)
	}

	mr := &agnogo.ModelResponse{}
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			mr.Text += c.Text
		case "tool_use":
			mr.ToolCalls = append(mr.ToolCalls, agnogo.ToolCall{
				ID: c.ID, Name: c.Name, Arguments: string(c.Input),
			})
		}
	}
	if result.Usage != nil {
		mr.Usage = &agnogo.Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
		}
	}
	return mr, nil
}
