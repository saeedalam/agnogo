package agnogo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

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
}

func newOpenAICompat(apiKey, model, baseURL string) *openaiCompatProvider {
	return &openaiCompatProvider{
		apiKey: apiKey, model: model, baseURL: baseURL, cfg: DefaultModelConfig(),
	}
}

func (p *openaiCompatProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	return OpenAIChatCompletion(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

func (p *openaiCompatProvider) ChatCompletionStream(ctx context.Context, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	return OpenAIChatCompletionStream(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

// ─── Anthropic inline provider ──────────────────────────────

type anthropicInlineProvider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     ModelConfig
}

func newAnthropicInline(apiKey, model, baseURL string) *anthropicInlineProvider {
	return &anthropicInlineProvider{
		apiKey: apiKey, model: model, baseURL: baseURL, cfg: DefaultModelConfig(),
	}
}

func (p *anthropicInlineProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	return AnthropicChatCompletion(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

func (p *anthropicInlineProvider) ChatCompletionStream(ctx context.Context, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	return AnthropicChatCompletionStream(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

// ─── Gemini inline provider ─────────────────────────────────

type geminiInlineProvider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     ModelConfig
}

func newGeminiInline(apiKey, model, baseURL string) *geminiInlineProvider {
	return &geminiInlineProvider{
		apiKey: apiKey, model: model, baseURL: baseURL, cfg: DefaultModelConfig(),
	}
}

func (p *geminiInlineProvider) ChatCompletion(ctx context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	return GeminiChatCompletion(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

func (p *geminiInlineProvider) ChatCompletionStream(ctx context.Context, messages []Message, tools []map[string]any) (<-chan StreamEvent, error) {
	return GeminiChatCompletionStream(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

// ─── Ollama inline provider (OpenAI-compatible, no API key) ─

func newOllamaInline(model, host string) *openaiCompatProvider {
	return &openaiCompatProvider{
		apiKey:  "ollama",
		model:   model,
		baseURL: host + "/v1",
		cfg:     ModelConfig{MaxTokens: 2000, Temperature: 0.3, Timeout: 60 * time.Second},
	}
}
