// Package perplexity provides a Perplexity AI model provider for agnogo.
// Perplexity combines search + LLM with OpenAI-compatible API.
//
//	import "github.com/saeedalam/agnogo/providers/perplexity"
//	model := perplexity.New("pplx-...", "sonar-pro")
//	model := perplexity.New("pplx-...", "sonar")
package perplexity

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a Perplexity provider. Uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 { cfg = cfgs[0] }
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.perplexity.ai")
}
