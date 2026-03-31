package agnogo

import (
	"context"
	"fmt"
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
	Text     string         // incremental text content
	ToolCall *ToolCallDelta // partial tool call data
	Done     bool           // true when stream is complete
	Error    error          // non-nil on stream error
}

// ToolCallDelta is a partial tool call accumulated across stream chunks.
type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string // accumulated JSON fragment
}

// OpenAIStreamResponse is a backward-compatible alias for OpenAIChatCompletionStream.
// Deprecated: Use OpenAIChatCompletionStream instead.
func OpenAIStreamResponse(ctx context.Context, apiKey, model, baseURL string, messages []Message, tools []map[string]any, cfg ModelConfig) (<-chan StreamEvent, error) {
	return OpenAIChatCompletionStream(ctx, apiKey, model, baseURL, cfg, messages, tools)
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
