// Package groq provides a Groq model provider for agnogo.
// Groq offers extremely fast inference with OpenAI-compatible API.
//
//	import "github.com/saeedalam/agnogo/providers/groq"
//	model := groq.New("gsk_...", "llama-3.3-70b-versatile")
//	model := groq.New("gsk_...", "mixtral-8x7b-32768")
package groq

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates a Groq provider. Uses OpenAI-compatible API.
func New(apiKey, model string, cfgs ...agnogo.ModelConfig) *openai.Provider {
	cfg := agnogo.DefaultModelConfig()
	if len(cfgs) > 0 { cfg = cfgs[0] }
	return openai.New(apiKey, model, cfg).WithBaseURL("https://api.groq.com/openai/v1")
}
