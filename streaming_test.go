package agnogo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnthropicStreamParsing(t *testing.T) {
	sseBody := `event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-5-20250514","usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ModelConfig{MaxTokens: 1000, Temperature: 0.3, Timeout: 5 * time.Second}
	msgs := []Message{{Role: "user", Content: "hi"}}

	ch, err := AnthropicChatCompletionStream(ctx, "test-key", "claude-sonnet-4-5-20250514", srv.URL, cfg, msgs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var texts []string
	var gotDone bool
	for ev := range ch {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Text != "" {
			texts = append(texts, ev.Text)
		}
		if ev.Done {
			gotDone = true
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("texts = %v, want [Hello, ' world']", texts)
	}
	if !gotDone {
		t.Error("expected Done=true event")
	}
}

func TestGeminiStreamParsing(t *testing.T) {
	sseBody := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}

data: {"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ModelConfig{MaxTokens: 1000, Temperature: 0.3, Timeout: 5 * time.Second}
	msgs := []Message{{Role: "user", Content: "hi"}}

	// GeminiChatCompletionStream builds URL as: baseURL/models/MODEL:streamGenerateContent?alt=sse&key=KEY
	// So we need to set baseURL so the constructed URL routes to our test server.
	// The server ignores the path, so we just pass srv.URL as the baseURL.
	ch, err := GeminiChatCompletionStream(ctx, "test-key", "gemini-2.5-flash", srv.URL, cfg, msgs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var texts []string
	var gotDone bool
	for ev := range ch {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Text != "" {
			texts = append(texts, ev.Text)
		}
		if ev.Done {
			gotDone = true
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("texts = %v, want [Hello, ' world']", texts)
	}
	if !gotDone {
		t.Error("expected Done=true event")
	}
}

func TestOpenAIStreamParsing(t *testing.T) {
	sseBody := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"}}]}

data: [DONE]

`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ModelConfig{MaxTokens: 1000, Temperature: 0.3, Timeout: 5 * time.Second}
	msgs := []Message{{Role: "user", Content: "hi"}}

	// OpenAIChatCompletionStream appends /chat/completions to baseURL
	ch, err := OpenAIChatCompletionStream(ctx, "test-key", "gpt-4.1-mini", srv.URL, cfg, msgs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var texts []string
	var gotDone bool
	for ev := range ch {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Text != "" {
			texts = append(texts, ev.Text)
		}
		if ev.Done {
			gotDone = true
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("texts = %v, want [Hello, ' world']", texts)
	}
	if !gotDone {
		t.Error("expected Done=true event")
	}
}
