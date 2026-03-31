// Package anthropic provides a Claude model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/anthropic"
//	model := anthropic.New("sk-ant-...", "claude-sonnet-4-5-20250514")
package anthropic

import (
	"context"

	"github.com/saeedalam/agnogo"
)

const defaultBaseURL = "https://api.anthropic.com/v1/messages"

// Provider implements agnogo.ModelProvider for Anthropic Claude.
type Provider struct {
	apiKey string
	model  string
	cfg    agnogo.ModelConfig
}

// New creates a Claude provider.
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
	return &Provider{apiKey: apiKey, model: model, cfg: cfg}
}

func (p *Provider) ChatCompletion(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
	return agnogo.AnthropicChatCompletion(ctx, p.apiKey, p.model, defaultBaseURL, p.cfg, messages, tools)
}

// ChatCompletionStream implements agnogo.StreamProvider for real token-level streaming.
func (p *Provider) ChatCompletionStream(ctx context.Context, messages []agnogo.Message, tools []map[string]any) (<-chan agnogo.StreamEvent, error) {
	return agnogo.AnthropicChatCompletionStream(ctx, p.apiKey, p.model, defaultBaseURL, p.cfg, messages, tools)
}
