// Package mistral provides a Mistral AI model provider for agnogo.
//
//	import "github.com/saeedalam/agnogo/providers/mistral"
//	model := mistral.New("...", "mistral-large-latest")
//	model := mistral.New("...", "mistral-small-latest")
package mistral

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a Mistral provider. Uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 { cfg = cfgs[0] }
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.mistral.ai/v1")
}
