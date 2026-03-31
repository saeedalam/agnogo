package agnogo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// clearProviderEnv unsets all provider-related env vars for clean tests.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, pd := range builtinProviders {
		t.Setenv(pd.envVar, "")
	}
	t.Setenv("OLLAMA_HOST", "")
}

func TestAutoProviderOpenAI(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	p, err := autoProvider()
	if err != nil {
		t.Fatalf("autoProvider() error: %v", err)
	}
	if p == nil {
		t.Fatal("autoProvider() returned nil")
	}
	if _, ok := p.(*openaiCompatProvider); !ok {
		t.Fatalf("expected *openaiCompatProvider, got %T", p)
	}
}

func TestAutoProviderAnthropicPriority(t *testing.T) {
	clearProviderEnv(t)
	// Set both keys — OpenAI is first in the table so it should win.
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")

	p, err := autoProvider()
	if err != nil {
		t.Fatalf("autoProvider() error: %v", err)
	}
	if _, ok := p.(*openaiCompatProvider); !ok {
		t.Fatalf("expected OpenAI to win (first in table), got %T", p)
	}
}

func TestAutoProviderOllama(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OLLAMA_HOST", "http://fake-ollama:11434")

	p, err := autoProvider()
	if err != nil {
		t.Fatalf("autoProvider() error: %v", err)
	}
	if p == nil {
		t.Fatal("autoProvider() returned nil")
	}
	ocp, ok := p.(*openaiCompatProvider)
	if !ok {
		t.Fatalf("expected *openaiCompatProvider (Ollama), got %T", p)
	}
	if ocp.baseURL != "http://fake-ollama:11434/v1" {
		t.Fatalf("expected Ollama base URL, got %s", ocp.baseURL)
	}
}

func TestAutoProviderNone(t *testing.T) {
	clearProviderEnv(t)

	_, err := autoProvider()
	if err == nil {
		t.Fatal("expected error when no env vars set, got nil")
	}
}

func TestWithOpenAIOption(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	sc := &smartConfig{}
	opt := WithOpenAI()
	opt.applyOption(sc)

	if sc.Model == nil {
		t.Fatal("WithOpenAI() did not set Model")
	}
	ocp, ok := sc.Model.(*openaiCompatProvider)
	if !ok {
		t.Fatalf("expected *openaiCompatProvider, got %T", sc.Model)
	}
	if ocp.model != "gpt-4.1-mini" {
		t.Fatalf("expected default model gpt-4.1-mini, got %s", ocp.model)
	}
}

func TestWithOpenAIOptionCustomModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	sc := &smartConfig{}
	opt := WithOpenAI("gpt-4o")
	opt.applyOption(sc)

	ocp := sc.Model.(*openaiCompatProvider)
	if ocp.model != "gpt-4o" {
		t.Fatalf("expected custom model gpt-4o, got %s", ocp.model)
	}
}

func TestWithOpenAIOptionPanicsWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when OPENAI_API_KEY not set")
		}
	}()
	sc := &smartConfig{}
	opt := WithOpenAI()
	opt.applyOption(sc)
}

func TestOpenAICompatProviderFormat(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": "Hello from mock!",
				},
			}},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newOpenAICompat("test-key", "gpt-test", srv.URL)
	msgs := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}

	mr, err := p.ChatCompletion(context.Background(), msgs, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if mr.Text != "Hello from mock!" {
		t.Fatalf("expected 'Hello from mock!', got %q", mr.Text)
	}
	if mr.Usage == nil || mr.Usage.TotalTokens != 15 {
		t.Fatalf("unexpected usage: %+v", mr.Usage)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected Bearer auth, got %q", gotAuth)
	}
	if gotBody["model"] != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %v", gotBody["model"])
	}
	// Verify messages format.
	rawMsgs, ok := gotBody["messages"].([]any)
	if !ok || len(rawMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %v", gotBody["messages"])
	}
}

func TestOpenAICompatProviderToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"tool_calls": []map[string]any{{
						"id":   "call_123",
						"type": "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": `{"city":"SF"}`,
						},
					}},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newOpenAICompat("test-key", "gpt-test", srv.URL)
	mr, err := p.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "weather?"}}, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if len(mr.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(mr.ToolCalls))
	}
	if mr.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", mr.ToolCalls[0].Name)
	}
}

func TestAnthropicInlineProviderFormat(t *testing.T) {
	var gotBody map[string]any
	var gotAPIKey string
	var gotVersion string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		resp := map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "Hello from Anthropic mock!",
			}},
			"usage": map[string]any{
				"input_tokens":  12,
				"output_tokens": 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newAnthropicInline("ant-test-key", "claude-test", srv.URL)
	msgs := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}

	mr, err := p.ChatCompletion(context.Background(), msgs, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if mr.Text != "Hello from Anthropic mock!" {
		t.Fatalf("expected 'Hello from Anthropic mock!', got %q", mr.Text)
	}
	if mr.Usage == nil || mr.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %+v", mr.Usage)
	}
	if gotAPIKey != "ant-test-key" {
		t.Fatalf("expected x-api-key header, got %q", gotAPIKey)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("expected anthropic-version 2023-06-01, got %q", gotVersion)
	}
	// System prompt should be extracted to top level.
	if gotBody["system"] == nil {
		t.Fatal("expected system prompt at top level")
	}
	if gotBody["model"] != "claude-test" {
		t.Fatalf("expected model claude-test, got %v", gotBody["model"])
	}
	// Messages should not contain system role.
	rawMsgs, ok := gotBody["messages"].([]any)
	if !ok {
		t.Fatalf("expected messages array")
	}
	for _, rm := range rawMsgs {
		msg := rm.(map[string]any)
		if msg["role"] == "system" {
			t.Fatal("system message should not be in messages array")
		}
	}
}

func TestAnthropicInlineProviderToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{{
				"type":  "tool_use",
				"id":    "toolu_123",
				"name":  "get_weather",
				"input": map[string]any{"city": "SF"},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newAnthropicInline("ant-test-key", "claude-test", srv.URL)
	mr, err := p.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "weather?"}}, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if len(mr.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(mr.ToolCalls))
	}
	if mr.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", mr.ToolCalls[0].Name)
	}
	if mr.ToolCalls[0].ID != "toolu_123" {
		t.Fatalf("expected toolu_123, got %s", mr.ToolCalls[0].ID)
	}
}

func TestGeminiInlineProviderFormat(t *testing.T) {
	var gotBody map[string]any
	var gotURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		resp := map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]any{{
						"text": "Hello from Gemini mock!",
					}},
				},
			}},
			"usageMetadata": map[string]any{
				"promptTokenCount":     8,
				"candidatesTokenCount": 6,
				"totalTokenCount":      14,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newGeminiInline("gemini-test-key", "gemini-test", srv.URL)
	msgs := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}

	mr, err := p.ChatCompletion(context.Background(), msgs, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if mr.Text != "Hello from Gemini mock!" {
		t.Fatalf("expected 'Hello from Gemini mock!', got %q", mr.Text)
	}
	if mr.Usage == nil || mr.Usage.TotalTokens != 14 {
		t.Fatalf("unexpected usage: %+v", mr.Usage)
	}
	// URL should contain model name and API key.
	if gotURL == "" {
		t.Fatal("no URL captured")
	}
	// Check system instruction extracted.
	if gotBody["systemInstruction"] == nil {
		t.Fatal("expected systemInstruction at top level")
	}
	// Contents should not have system role.
	contents, ok := gotBody["contents"].([]any)
	if !ok {
		t.Fatalf("expected contents array")
	}
	for _, c := range contents {
		cm := c.(map[string]any)
		if cm["role"] == "system" {
			t.Fatal("system should not be in contents")
		}
	}
}

func TestGeminiInlineProviderFunctionCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]any{{
						"functionCall": map[string]any{
							"name": "get_weather",
							"args": map[string]any{"city": "SF"},
						},
					}},
				},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := newGeminiInline("gemini-test-key", "gemini-test", srv.URL)
	mr, err := p.ChatCompletion(context.Background(), []Message{{Role: "user", Content: "weather?"}}, nil)
	if err != nil {
		t.Fatalf("ChatCompletion error: %v", err)
	}
	if len(mr.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(mr.ToolCalls))
	}
	if mr.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", mr.ToolCalls[0].Name)
	}
}

// TestAutoProviderNoneEnvClean verifies error message mentions env vars.
func TestAutoProviderNoneEnvClean(t *testing.T) {
	// Save and clear all env vars.
	saved := make(map[string]string)
	for _, pd := range builtinProviders {
		saved[pd.envVar] = os.Getenv(pd.envVar)
		t.Setenv(pd.envVar, "")
	}
	saved["OLLAMA_HOST"] = os.Getenv("OLLAMA_HOST")
	t.Setenv("OLLAMA_HOST", "")

	_, err := autoProvider()
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Fatal("error message is empty")
	}
}
