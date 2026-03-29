// Package grok provides an xAI Grok model provider for agnogo.
// Grok uses an OpenAI-compatible API.
//
//	import "github.com/saeedalam/agnogo/providers/grok"
//	model := grok.New("xai-...", "grok-3")
package grok

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a Grok provider. xAI uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.x.ai/v1")
}
