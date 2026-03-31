package agnogo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// sharedTransport is reused across all inline providers to pool connections.
var sharedTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// providerDef describes a provider that can be auto-detected from env vars.
type providerDef struct {
	envVar       string
	defaultModel string
	baseURL      string
	format       string // "openai", "anthropic", "gemini"
}

// builtinProviders is scanned in order; the first matching env var wins.
var builtinProviders = []providerDef{
	{"OPENAI_API_KEY", "gpt-4.1-mini", "https://api.openai.com/v1", "openai"},
	{"ANTHROPIC_API_KEY", "claude-sonnet-4-5-20250514", "https://api.anthropic.com/v1/messages", "anthropic"},
	{"GEMINI_API_KEY", "gemini-2.0-flash", "https://generativelanguage.googleapis.com/v1beta", "gemini"},
	{"GROQ_API_KEY", "llama-3.3-70b-versatile", "https://api.groq.com/openai/v1", "openai"},
	{"DEEPSEEK_API_KEY", "deepseek-chat", "https://api.deepseek.com/v1", "openai"},
	{"MISTRAL_API_KEY", "mistral-large-latest", "https://api.mistral.ai/v1", "openai"},
	{"TOGETHER_API_KEY", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "https://api.together.xyz/v1", "openai"},
	{"PERPLEXITY_API_KEY", "sonar", "https://api.perplexity.ai", "openai"},
	{"GROK_API_KEY", "grok-3-mini-fast", "https://api.x.ai/v1", "openai"},
}

// autoProvider scans environment variables and returns the first available
// provider. It checks registered providers first (backward compat), then
// the builtin table, then Ollama (no key needed).
func autoProvider() (ModelProvider, error) {
	// 1. Check registered providers first (backward compat with autodetect import).
	for _, rp := range providerRegistry {
		if key := os.Getenv(rp.envVar); key != "" {
			return rp.factory(key), nil
		}
	}

	// 2. Scan builtin provider table.
	for _, pd := range builtinProviders {
		key := os.Getenv(pd.envVar)
		if key == "" {
			continue
		}
		switch pd.format {
		case "openai":
			return newOpenAICompat(key, pd.defaultModel, pd.baseURL), nil
		case "anthropic":
			return newAnthropicInline(key, pd.defaultModel, pd.baseURL), nil
		case "gemini":
			return newGeminiInline(key, pd.defaultModel, pd.baseURL), nil
		}
	}

	// 3. Check Ollama last (no API key needed).
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost != "" {
		return newOllamaInline("llama3.1", ollamaHost), nil
	}
	// Try default Ollama at localhost — only if nothing else matched.
	// We probe with a quick HEAD to avoid false positives.
	if ollamaAvailable("http://localhost:11434") {
		return newOllamaInline("llama3.1", "http://localhost:11434"), nil
	}

	var tried []string
	for _, pd := range builtinProviders {
		tried = append(tried, pd.envVar)
	}
	return nil, fmt.Errorf("agnogo: no API key found; set one of: %v (or run Ollama locally)", tried)
}

// ollamaAvailable does a quick check to see if Ollama is running.
func ollamaAvailable(host string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(host + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ─── OpenAI-compatible inline provider ──────────────────────

type openaiCompatProvider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     ModelConfig
	client  *http.Client
}

func newOpenAICompat(apiKey, model, baseURL string) *openaiCompatProvider {
	cfg := DefaultModelConfig()
	return &openaiCompatProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		cfg:     cfg,
		client:  &http.Client{Timeout: cfg.Timeout, Transport: sharedTransport},
	}
}

func (p *openaiCompatProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
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
			if m.Content == "" {
				delete(msg, "content")
			}
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

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
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
		return nil, ParseProviderError("openai-compat", resp.StatusCode, data, resp.Header)
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
	mr := &ModelResponse{}
	if choice.Content != nil {
		mr.Text = *choice.Content
	}
	for _, tc := range choice.ToolCalls {
		mr.ToolCalls = append(mr.ToolCalls, ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
		})
	}
	if result.Usage != nil {
		mr.Usage = &Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
			TotalTokens:  result.Usage.TotalTokens,
		}
	}
	return mr, nil
}

// ─── Anthropic inline provider ──────────────────────────────

type anthropicInlineProvider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     ModelConfig
	client  *http.Client
}

func newAnthropicInline(apiKey, model, baseURL string) *anthropicInlineProvider {
	cfg := DefaultModelConfig()
	return &anthropicInlineProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		cfg:     cfg,
		client:  &http.Client{Timeout: cfg.Timeout, Transport: sharedTransport},
	}
}

func (p *anthropicInlineProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	// Separate system prompt from messages (Anthropic API requires it at top level).
	var systemPrompt string
	var apiMsgs []map[string]any
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt += m.Content + "\n"
			continue
		}
		msg := map[string]any{"role": m.Role, "content": m.Content}
		// Map tool results to Anthropic format.
		if m.Role == "tool" {
			msg = map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": m.Name,
					"content":     m.Content,
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
		// Convert OpenAI tool format to Anthropic format.
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
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(bodyJSON))
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
		return nil, ParseProviderError("anthropic", resp.StatusCode, data, resp.Header)
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
		return nil, fmt.Errorf("parse: %w", err)
	}

	mr := &ModelResponse{}
	for _, c := range result.Content {
		switch c.Type {
		case "text":
			mr.Text += c.Text
		case "tool_use":
			mr.ToolCalls = append(mr.ToolCalls, ToolCall{
				ID: c.ID, Name: c.Name, Arguments: string(c.Input),
			})
		}
	}
	if result.Usage != nil {
		mr.Usage = &Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
			TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
		}
	}
	return mr, nil
}

// ─── Gemini inline provider ─────────────────────────────────

type geminiInlineProvider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     ModelConfig
	client  *http.Client
}

func newGeminiInline(apiKey, model, baseURL string) *geminiInlineProvider {
	cfg := DefaultModelConfig()
	return &geminiInlineProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		cfg:     cfg,
		client:  &http.Client{Timeout: cfg.Timeout, Transport: sharedTransport},
	}
}

func (p *geminiInlineProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
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
			// Gemini tool results.
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
		// Convert OpenAI tools to Gemini function declarations.
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

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, ParseProviderError("gemini", resp.StatusCode, data, resp.Header)
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

	mr := &ModelResponse{}
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			mr.Text += part.Text
		}
		if part.FunctionCall != nil {
			mr.ToolCalls = append(mr.ToolCalls, ToolCall{
				ID:        fmt.Sprintf("call_%s", part.FunctionCall.Name),
				Name:      part.FunctionCall.Name,
				Arguments: string(part.FunctionCall.Args),
			})
		}
	}
	if result.UsageMetadata != nil {
		mr.Usage = &Usage{
			InputTokens:  result.UsageMetadata.PromptTokenCount,
			OutputTokens: result.UsageMetadata.CandidatesTokenCount,
			TotalTokens:  result.UsageMetadata.TotalTokenCount,
		}
	}
	return mr, nil
}

// ─── Ollama inline provider (OpenAI-compatible, no API key) ─

func newOllamaInline(model, host string) *openaiCompatProvider {
	cfg := ModelConfig{MaxTokens: 2000, Temperature: 0.3, Timeout: 60 * time.Second}
	return &openaiCompatProvider{
		apiKey:  "ollama",
		model:   model,
		baseURL: host + "/v1",
		cfg:     cfg,
		client:  &http.Client{Timeout: cfg.Timeout, Transport: sharedTransport},
	}
}
