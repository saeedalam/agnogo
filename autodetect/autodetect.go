// Package autodetect registers all built-in provider factories with agnogo
// for automatic provider detection from environment variables.
//
// Import this package with a blank identifier to enable auto-detection:
//
//	import _ "github.com/saeedalam/agnogo/autodetect"
//
// Then use agnogo.SmartAgent or agnogo.DetectProvider to automatically
// select a provider based on available API keys.
package autodetect

import (
	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/anthropic"
	"github.com/saeedalam/agnogo/providers/deepseek"
	"github.com/saeedalam/agnogo/providers/gemini"
	"github.com/saeedalam/agnogo/providers/grok"
	"github.com/saeedalam/agnogo/providers/groq"
	"github.com/saeedalam/agnogo/providers/mistral"
	"github.com/saeedalam/agnogo/providers/ollama"
	"github.com/saeedalam/agnogo/providers/openai"
	"github.com/saeedalam/agnogo/providers/perplexity"
	"github.com/saeedalam/agnogo/providers/together"
)

func init() {
	agnogo.RegisterProvider("OPENAI_API_KEY", "gpt-4.1-mini", func(apiKey string) agnogo.ModelProvider {
		return openai.New(apiKey, "gpt-4.1-mini")
	})

	agnogo.RegisterProvider("ANTHROPIC_API_KEY", "claude-sonnet-4-5-20250514", func(apiKey string) agnogo.ModelProvider {
		return anthropic.New(apiKey, "claude-sonnet-4-5-20250514")
	})

	agnogo.RegisterProvider("GEMINI_API_KEY", "gemini-2.0-flash", func(apiKey string) agnogo.ModelProvider {
		return gemini.New(apiKey, "gemini-2.0-flash")
	})

	agnogo.RegisterProvider("GROQ_API_KEY", "llama-3.3-70b-versatile", func(apiKey string) agnogo.ModelProvider {
		return groq.New(apiKey, "llama-3.3-70b-versatile")
	})

	agnogo.RegisterProvider("DEEPSEEK_API_KEY", "deepseek-chat", func(apiKey string) agnogo.ModelProvider {
		return deepseek.New(apiKey, "deepseek-chat")
	})

	agnogo.RegisterProvider("MISTRAL_API_KEY", "mistral-large-latest", func(apiKey string) agnogo.ModelProvider {
		return mistral.New(apiKey, "mistral-large-latest")
	})

	agnogo.RegisterProvider("TOGETHER_API_KEY", "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", func(apiKey string) agnogo.ModelProvider {
		return together.New(apiKey, "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo")
	})

	agnogo.RegisterProvider("PERPLEXITY_API_KEY", "sonar", func(apiKey string) agnogo.ModelProvider {
		return perplexity.New(apiKey, "sonar")
	})

	agnogo.RegisterProvider("GROK_API_KEY", "grok-3-mini-fast", func(apiKey string) agnogo.ModelProvider {
		return grok.New(apiKey, "grok-3-mini-fast")
	})

	// Ollama doesn't use an API key. It checks OLLAMA_HOST for the server
	// address, falling back to localhost:11434.
	agnogo.RegisterProvider("OLLAMA_HOST", "llama3.1", func(host string) agnogo.ModelProvider {
		return ollama.New("llama3.1", host)
	})

}
