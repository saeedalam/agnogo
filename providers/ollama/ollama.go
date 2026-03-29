// Package ollama provides a local Ollama model provider for agnogo.
// Ollama uses an OpenAI-compatible API, so this is a thin wrapper.
//
//	import "github.com/saeedalam/agnogo/providers/ollama"
//	model := ollama.New("llama3.1")  // uses localhost:11434
//	model := ollama.New("mistral", "http://gpu-server:11434")
package ollama

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

// New creates an Ollama provider. Ollama is OpenAI-compatible.
// Default URL: http://localhost:11434
func New(model string, baseURLs ...string) *openai.Provider {
	baseURL := "http://localhost:11434"
	if len(baseURLs) > 0 && baseURLs[0] != "" {
		baseURL = baseURLs[0]
	}
	return openai.New("ollama", model, agnogo.ModelConfig{
		MaxTokens: 2000, Temperature: 0.3,
	}).WithBaseURL(baseURL + "/v1")
}
