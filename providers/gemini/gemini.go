// Package gemini provides a Google Gemini model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/gemini"
//	model := gemini.New("AIza...", "gemini-2.5-flash")
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/saeedalam/agnogo"
)

// Provider implements agnogo.ModelProvider for Google Gemini.
type Provider struct {
	apiKey string
	model  string
	cfg    agnogo.ModelConfig
	client *http.Client
}

// New creates a Gemini provider.
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
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (p *Provider) ChatCompletion(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
	// Convert to Gemini format
	var contents []map[string]any
	var systemInstruction string

	for _, m := range messages {
		if m.Role == "system" {
			systemInstruction += m.Content + "\n"
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "tool" {
			// Gemini tool results
			var parsed any
			json.Unmarshal([]byte(m.Content), &parsed)
			if parsed == nil {
				parsed = map[string]any{"result": m.Content}
			}
			contents = append(contents, map[string]any{
				"role": "function",
				"parts": []map[string]any{{
					"functionResponse": map[string]any{
						"name":     m.Name,
						"response": parsed,
					},
				}},
			})
			continue
		}
		parts := []map[string]any{{"text": m.Content}}
		if len(m.ToolCalls) > 0 {
			parts = nil
			for _, tc := range m.ToolCalls {
				var args any
				json.Unmarshal([]byte(tc.Arguments), &args)
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{"name": tc.Name, "args": args},
				})
			}
		}
		contents = append(contents, map[string]any{"role": role, "parts": parts})
	}

	body := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": p.cfg.MaxTokens,
			"temperature":     p.cfg.Temperature,
		},
	}
	if systemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}
	if len(tools) > 0 {
		// Convert OpenAI tools to Gemini function declarations
		var funcDecls []map[string]any
		for _, t := range tools {
			fn, ok := t["function"].(map[string]any)
			if !ok {
				continue
			}
			decl := map[string]any{"name": fn["name"], "description": fn["description"]}
			if params, ok := fn["parameters"]; ok {
				decl["parameters"] = params
			}
			funcDecls = append(funcDecls, decl)
		}
		body["tools"] = []map[string]any{{"functionDeclarations": funcDecls}}
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.model, p.apiKey)
	bodyJSON, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini read body: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(data)[:min(len(data), 300)])
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	mr := &agnogo.ModelResponse{}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			mr.Text += part.Text
		}
		if part.FunctionCall != nil {
			mr.ToolCalls = append(mr.ToolCalls, agnogo.ToolCall{
				ID: fmt.Sprintf("call_%s", part.FunctionCall.Name),
				Name: part.FunctionCall.Name,
				Arguments: string(part.FunctionCall.Args),
			})
		}
	}
	if result.UsageMetadata != nil {
		mr.Usage = &agnogo.Usage{
			InputTokens:  result.UsageMetadata.PromptTokenCount,
			OutputTokens: result.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  result.UsageMetadata.TotalTokenCount,
		}
	}
	return mr, nil
}
