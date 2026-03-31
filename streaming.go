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

// StreamProvider is an optional interface for models that support token-level streaming.
// If a ModelProvider also implements StreamProvider, RunStream uses real SSE streaming.
type StreamProvider interface {
	// ChatCompletionStream returns a channel of streaming chunks.
	ChatCompletionStream(ctx context.Context, messages []Message, tools []map[string]any) (<-chan StreamEvent, error)
}

// StreamEvent is one event from the model's SSE stream.
// Matches Agno's ModelResponse streaming: text chunks, tool call fragments, done signal.
type StreamEvent struct {
	Text      string // incremental text content
	ToolCall  *ToolCallDelta // partial tool call data
	Done      bool   // true when stream is complete
	Error     error  // non-nil on stream error
}

// ToolCallDelta is a partial tool call accumulated across stream chunks.
type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string // accumulated JSON fragment
}

// OpenAIStreamResponse parses OpenAI's SSE stream format.
// Used by the OpenAI provider to implement StreamProvider.
func OpenAIStreamResponse(ctx context.Context, apiKey, model, baseURL string, messages []Message, tools []map[string]any, cfg ModelConfig) (<-chan StreamEvent, error) {
	// Build request same as non-streaming
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
		"model": model, "messages": oaiMsgs,
		"max_tokens": cfg.MaxTokens, "temperature": cfg.Temperature,
		"stream": true,
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

	client := &http.Client{Timeout: cfg.Timeout}
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

		toolCalls := map[int]*ToolCallDelta{} // accumulate tool calls by index

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				// Emit accumulated tool calls
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

// RunStreamReal uses real token-level streaming if the provider supports it.
// Falls back to word-level streaming if not.
func (a *Core) RunStreamReal(ctx context.Context, session *Session, userMessage string) <-chan StreamChunk {
	ch := make(chan StreamChunk, 50)

	sp, ok := a.model.(StreamProvider)
	if !ok {
		// Fallback to word-level streaming
		return a.RunStream(ctx, session, userMessage)
	}

	go func() {
		defer close(ch)

		// Build messages (same as Run)
		systemPrompt := a.instructions
		if a.promptFunc != nil {
			systemPrompt = a.promptFunc(session)
		}
		messages := []Message{{Role: "system", Content: systemPrompt}}
		messages = append(messages, session.History...)
		session.AddMessage("user", userMessage)
		messages = append(messages, Message{Role: "user", Content: userMessage})

		if a.knowledge != nil {
			messages = injectKnowledge(ctx, a.knowledge, userMessage, messages, a.knowledgeN)
		}
		if a.history != nil {
			messages = trimHistory(messages, *a.history)
		}

		toolDefs := a.tools.FunctionDefs()

		// Stream from model
		eventCh, err := sp.ChatCompletionStream(ctx, messages, toolDefs)
		if err != nil {
			ch <- StreamChunk{Error: err, Done: true}
			return
		}

		var fullText string
		var toolCallDeltas []*ToolCallDelta

		for evt := range eventCh {
			if evt.Error != nil {
				ch <- StreamChunk{Error: evt.Error, Done: true}
				return
			}
			if evt.Text != "" {
				fullText += evt.Text
				ch <- StreamChunk{Text: evt.Text}
			}
			if evt.ToolCall != nil {
				toolCallDeltas = append(toolCallDeltas, evt.ToolCall)
			}
			if evt.Done {
				break
			}
		}

		// If model returned tool calls, execute them and continue
		if len(toolCallDeltas) > 0 {
			// Execute tools (not streamed — tool results come as a batch)
			for _, tcd := range toolCallDeltas {
				args := ParseArgs(tcd.Arguments)
				result, err := a.tools.Invoke(ctx, tcd.Name, args)
				if err != nil {
					result = fmt.Sprintf("Tool '%s' failed: %s", tcd.Name, err.Error())
				}
				session.AddToolResult(tcd.ID, result)
			}
			// After tool execution, run again (non-streaming) for the final response
			resp, err := a.Run(ctx, session, "[Tool results processed. Continue responding.]")
			if err != nil {
				ch <- StreamChunk{Error: err, Done: true}
				return
			}
			if resp != nil {
				for _, word := range strings.Fields(resp.Text) {
					ch <- StreamChunk{Text: word + " "}
					time.Sleep(10 * time.Millisecond)
				}
			}
		} else {
			session.AddMessage("assistant", fullText)
		}

		ch <- StreamChunk{Done: true}
	}()

	return ch
}
