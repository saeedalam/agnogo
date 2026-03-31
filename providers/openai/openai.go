// Package openai provides an OpenAI model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/openai"
//	model := openai.New("sk-...", "gpt-4.1-mini")
package openai

import (
	"context"

	"github.com/saeedalam/agnogo"
)

// Provider implements agnogo.ModelProvider for OpenAI.
type Provider struct {
	apiKey  string
	model   string
	baseURL string
	cfg     agnogo.ModelConfig
}

// New creates an OpenAI provider.
//
//	model := openai.New("sk-...", "gpt-4.1-mini")
//	model := openai.New("sk-...", "gpt-4o", agnogo.ModelConfig{MaxTokens: 2000})
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 {
		c := cfgs[0]
		if c.MaxTokens > 0 {
			cfg.MaxTokens = c.MaxTokens
		}
		if c.Temperature > 0 {
			cfg.Temperature = c.Temperature
		}
		if c.Timeout > 0 {
			cfg.Timeout = c.Timeout
		}
	}
	return &Provider{
		apiKey: apiKey, model: model,
		baseURL: "https://api.openai.com/v1",
		cfg:     cfg,
	}
}

// WithBaseURL sets a custom base URL (for proxies, Azure, etc.)
func (p *Provider) WithBaseURL(url string) *Provider {
	p.baseURL = url
	return p
}

func (p *Provider) ChatCompletion(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
	return agnogo.OpenAIChatCompletion(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}

// ChatCompletionStream implements agnogo.StreamProvider for real token-level streaming.
func (p *Provider) ChatCompletionStream(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (<-chan agnogo.StreamEvent, error) {
	return agnogo.OpenAIChatCompletionStream(ctx, p.apiKey, p.model, p.baseURL, p.cfg, messages, tools)
}
