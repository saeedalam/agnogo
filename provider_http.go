// provider_http.go contains the shared HTTP logic for LLM providers.
// Both auto_providers.go (inline constructors) and providers/*/ (subpackages) use these.
package agnogo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SharedHTTPTransport is reused across all providers to pool connections.
var SharedHTTPTransport = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
}

// newHTTPClient creates an http.Client using the shared transport.
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: SharedHTTPTransport}
}

// ─── OpenAI-compatible chat completion ─────────────────────

// formatOpenAIMessages converts agnogo Messages to the OpenAI API format.
func formatOpenAIMessages(messages []Message) []map[string]any {
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
	return oaiMsgs
}

// parseOpenAIResponse parses the OpenAI-format JSON response into a ModelResponse.
func parseOpenAIResponse(data []byte) (*ModelResponse, error) {
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

// OpenAIChatCompletion sends a chat completion request to an OpenAI-compatible API.
// Used by auto_providers.go and providers/openai.
func OpenAIChatCompletion(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	oaiMsgs := formatOpenAIMessages(messages)

	body := map[string]any{
		"model": model, "messages": oaiMsgs,
		"max_tokens": cfg.MaxTokens, "temperature": cfg.Temperature,
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: HTTP request to %s failed: %w", baseURL, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, ParseProviderError("openai", resp.StatusCode, data, resp.Header)
	}

	return parseOpenAIResponse(data)
}

// OpenAIChatCompletionStream parses OpenAI's SSE stream format.
// Used by the OpenAI provider and inline OpenAI-compatible providers to implement StreamProvider.
func OpenAIChatCompletionStream(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	oaiMsgs := formatOpenAIMessages(messages)

	body := map[string]any{
		"model": model, "messages": oaiMsgs,
		"max_tokens": cfg.MaxTokens, "temperature": cfg.Temperature,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "text/event-stream")

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream request: %w", err)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}

	ch := make(chan StreamEvent, 50)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		toolCalls := map[int]*ToolCallDelta{}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				for _, tc := range toolCalls {
					ch <- StreamEvent{ToolCall: tc}
				}
				ch <- StreamEvent{Done: true}
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content   *string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			if delta.Content != nil && *delta.Content != "" {
				ch <- StreamEvent{Text: *delta.Content}
			}
			for _, tc := range delta.ToolCalls {
				if _, ok := toolCalls[tc.Index]; !ok {
					toolCalls[tc.Index] = &ToolCallDelta{Index: tc.Index}
				}
				tcd := toolCalls[tc.Index]
				if tc.ID != "" {
					tcd.ID = tc.ID
				}
				if tc.Function.Name != "" {
					tcd.Name = tc.Function.Name
				}
				tcd.Arguments += tc.Function.Arguments
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: err}
		}
	}()

	return ch, nil
}

// ─── Anthropic chat completion ─────────────────────────────

// formatAnthropicRequest builds the Anthropic API request body from agnogo Messages.
func formatAnthropicRequest(model string, cfg ModelConfig, messages []Message, tools []map[string]any) (systemPrompt string, apiMsgs []map[string]any, body map[string]any) {
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt += m.Content + "\n"
			continue
		}
		msg := map[string]any{"role": m.Role, "content": m.Content}
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

	body = map[string]any{
		"model":      model,
		"max_tokens": cfg.MaxTokens,
		"messages":   apiMsgs,
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if len(tools) > 0 {
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
	return
}

// parseAnthropicResponse parses the Anthropic-format JSON response into a ModelResponse.
func parseAnthropicResponse(data []byte) (*ModelResponse, error) {
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

// AnthropicChatCompletion sends a chat completion to Anthropic's API.
// Used by auto_providers.go and providers/anthropic.
func AnthropicChatCompletion(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	_, _, body := formatAnthropicRequest(model, cfg, messages, tools)

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
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

	return parseAnthropicResponse(data)
}

// AnthropicChatCompletionStream sends a streaming chat completion to Anthropic's SSE API.
// Returns a channel of StreamEvents for real-time token-level streaming.
func AnthropicChatCompletionStream(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	_, _, body := formatAnthropicRequest(model, cfg, messages, tools)
	body["stream"] = true

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "text/event-stream")

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: stream request: %w", err)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}

	ch := make(chan StreamEvent, 50)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Track tool_use blocks being streamed.
		type toolBlock struct {
			id   string
			name string
			args strings.Builder
		}
		var currentTool *toolBlock
		toolIndex := 0

		scanner := bufio.NewScanner(resp.Body)
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			// SSE event type line
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			// SSE data line
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			switch eventType {
			case "content_block_start":
				var block struct {
					ContentBlock struct {
						Type string `json:"type"`
						ID   string `json:"id"`
						Name string `json:"name"`
						Text string `json:"text"`
					} `json:"content_block"`
				}
				if json.Unmarshal([]byte(data), &block) == nil {
					if block.ContentBlock.Type == "tool_use" {
						currentTool = &toolBlock{
							id:   block.ContentBlock.ID,
							name: block.ContentBlock.Name,
						}
					}
					if block.ContentBlock.Type == "text" && block.ContentBlock.Text != "" {
						ch <- StreamEvent{Text: block.ContentBlock.Text}
					}
				}

			case "content_block_delta":
				var delta struct {
					Delta struct {
						Type         string `json:"type"`
						Text         string `json:"text"`
						PartialJSON string `json:"partial_json"`
					} `json:"delta"`
				}
				if json.Unmarshal([]byte(data), &delta) == nil {
					if delta.Delta.Type == "text_delta" && delta.Delta.Text != "" {
						ch <- StreamEvent{Text: delta.Delta.Text}
					}
					if delta.Delta.Type == "input_json_delta" && currentTool != nil {
						currentTool.args.WriteString(delta.Delta.PartialJSON)
					}
				}

			case "content_block_stop":
				if currentTool != nil {
					ch <- StreamEvent{ToolCall: &ToolCallDelta{
						Index:     toolIndex,
						ID:        currentTool.id,
						Name:      currentTool.name,
						Arguments: currentTool.args.String(),
					}}
					toolIndex++
					currentTool = nil
				}

			case "message_stop":
				ch <- StreamEvent{Done: true}
				return

			case "error":
				ch <- StreamEvent{Error: fmt.Errorf("anthropic stream error: %s", data)}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: err}
		}
	}()

	return ch, nil
}

// ─── Gemini chat completion ────────────────────────────────

// formatGeminiRequest builds the Gemini API request body from agnogo Messages.
func formatGeminiRequest(_ string, cfg ModelConfig, messages []Message, tools []map[string]any) (body map[string]any) {
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

	body = map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"maxOutputTokens": cfg.MaxTokens,
			"temperature":     cfg.Temperature,
		},
	}
	if systemInstruction != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemInstruction}},
		}
	}
	if len(tools) > 0 {
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
	return
}

// parseGeminiResponse parses the Gemini-format JSON response into a ModelResponse.
func parseGeminiResponse(data []byte) (*ModelResponse, error) {
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
		return nil, fmt.Errorf("gemini: parse response JSON: %w", err)
	}
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("gemini: empty response")
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

// GeminiChatCompletion sends a chat completion to Google Gemini's API.
// Used by auto_providers.go and providers/gemini.
func GeminiChatCompletion(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	body := formatGeminiRequest(model, cfg, messages, tools)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, apiKey)
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
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

	return parseGeminiResponse(data)
}

// GeminiChatCompletionStream sends a streaming chat completion to Gemini's API.
// Gemini uses streamGenerateContent endpoint; response is newline-delimited JSON chunks,
// each being a partial GenerateContentResponse.
func GeminiChatCompletionStream(ctx context.Context, apiKey, model, baseURL string, cfg ModelConfig, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	body := formatGeminiRequest(model, cfg, messages, tools)

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", baseURL, model, apiKey)
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := newHTTPClient(cfg.Timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini: stream request: %w", err)
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("gemini %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}

	ch := make(chan StreamEvent, 50)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		toolIndex := 0

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Gemini streaming with alt=sse uses SSE format: "data: {json}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var chunk struct {
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
			}
			if json.Unmarshal([]byte(data), &chunk) != nil {
				continue
			}

			for _, cand := range chunk.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						ch <- StreamEvent{Text: part.Text}
					}
					if part.FunctionCall != nil {
						ch <- StreamEvent{ToolCall: &ToolCallDelta{
							Index:     toolIndex,
							ID:        fmt.Sprintf("call_%s", part.FunctionCall.Name),
							Name:      part.FunctionCall.Name,
							Arguments: string(part.FunctionCall.Args),
						}}
						toolIndex++
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Error: err}
		} else {
			ch <- StreamEvent{Done: true}
		}
	}()

	return ch, nil
}
