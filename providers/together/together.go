// Package together provides a Together AI model provider for agnogo.
// Together hosts open-source models with OpenAI-compatible API.
//
//	import "github.com/saeedalam/agnogo/providers/together"
//	model := together.New("...", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
//	model := together.New("...", "mistralai/Mixtral-8x7B-Instruct-v0.1")
package together

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a Together AI provider. Uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 { cfg = cfgs[0] }
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.together.xyz/v1")
}
