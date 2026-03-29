// Package deepseek provides a DeepSeek model provider for agnogo.
// DeepSeek uses an OpenAI-compatible API.
//
//	import "github.com/saeedalam/agnogo/providers/deepseek"
//	model := deepseek.New("sk-...", "deepseek-chat")
//	model := deepseek.New("sk-...", "deepseek-reasoner") // with reasoning
package deepseek

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a DeepSeek provider. Uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 { cfg = cfgs[0] }
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.deepseek.com/v1")
}
