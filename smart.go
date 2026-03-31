package agnogo

import (
	"fmt"
	"os"
)

// Option configures the Agent() constructor. Tools, flags, and With*() functions
// all implement this interface, so they can be passed directly:
//
//	agent := agnogo.Agent("You are helpful.", weather, getTime, agnogo.Debug)
//	agent := agnogo.Agent("You are helpful.", agnogo.WithOpenAI(), agnogo.WithStorage(db))
type Option interface {
	applyOption(*smartConfig)
}

// optionFunc adapts a function into an Option.
type optionFunc func(*smartConfig)

func (f optionFunc) applyOption(sc *smartConfig) { f(sc) }


// ── Predefined flag options (no parentheses needed) ──────────

// Debug enables debug output.
//
//	agent := agnogo.Agent("...", agnogo.Debug)
var Debug Option = optionFunc(func(sc *smartConfig) {
	dbg := DefaultDebug()
	sc.Debug = &dbg
})

// Memory enables automatic pattern-based memory extraction.
//
//	agent := agnogo.Agent("...", agnogo.Memory)
var Memory Option = optionFunc(func(sc *smartConfig) {
	sc.AutoMemory = true
})

// Reasoning enables chain-of-thought reasoning before responding.
var Reasoning Option = optionFunc(func(sc *smartConfig) {
	sc.Reasoning = &ReasoningConfig{}
})

// UnsafeMode disables all safe defaults (retry, history trimming, hallucination guard).
var UnsafeMode Option = optionFunc(func(sc *smartConfig) {
	sc.unsafe = true
})

// smartConfig is an internal configuration struct that extends Config
// with fields used only by the Agent() constructor.
type smartConfig struct {
	Config
	tools              []ToolDef
	hooks              []Hook
	summarizeThreshold  int
	summarizeKeepRecent int
	unsafe              bool
	costBudget          *CostBudget
	piiConfig           *PIIConfig
	toolValidator       *ToolValidator
	confidenceThreshold float64
}

// registeredProvider holds the information needed to auto-detect a provider
// from environment variables.
type registeredProvider struct {
	envVar       string
	defaultModel string
	factory      func(apiKey string) ModelProvider
}

var providerRegistry []registeredProvider

// RegisterProvider registers a provider factory for auto-detection.
func RegisterProvider(envVar, defaultModel string, factory func(apiKey string) ModelProvider) {
	providerRegistry = append(providerRegistry, registeredProvider{
		envVar:       envVar,
		defaultModel: defaultModel,
		factory:      factory,
	})
}

// DetectProvider scans environment variables and returns the first available
// registered provider. Returns an error if no provider can be detected.
func DetectProvider() (ModelProvider, error) {
	for _, rp := range providerRegistry {
		if key := os.Getenv(rp.envVar); key != "" {
			return rp.factory(key), nil
		}
	}
	if len(providerRegistry) == 0 {
		return nil, fmt.Errorf("agnogo: no providers registered")
	}
	var tried []string
	for _, rp := range providerRegistry {
		tried = append(tried, rp.envVar)
	}
	return nil, fmt.Errorf("agnogo: no API key found; set one of: %v", tried)
}

// Agent creates an agent with smart defaults. Auto-detects provider from env vars.
// Tools, flags, and options can all be passed directly:
//
//	a := agnogo.Agent("You are helpful.")
//	a := agnogo.Agent("You are helpful.", weather, getTime, agnogo.Debug)
//	a := agnogo.Agent("You are helpful.", agnogo.WithOpenAI("gpt-4o"), agnogo.WithStorage(db))
func Agent(instructions string, opts ...Option) *Core {
	sc := smartConfig{
		Config: Config{
			Instructions: instructions,
		},
	}

	for _, opt := range opts {
		opt.applyOption(&sc)
	}

	// Auto-detect provider if none was set.
	if sc.Model == nil {
		p, err := autoProvider()
		if err != nil {
			panic(err)
		}
		sc.Model = p
	}

	// Apply safe defaults unless UnsafeMode was used.
	if !sc.unsafe {
		if sc.Retry == nil {
			rc := DefaultRetryConfig()
			sc.Retry = &rc
		}
		if sc.History == nil {
			hc := DefaultHistoryConfig()
			sc.History = &hc
		}
	}

	a := New(sc.Config)

	if len(sc.hooks) > 0 {
		a.hooks = sc.hooks
	}
	if sc.summarizeThreshold > 0 {
		a.summarizeThreshold = sc.summarizeThreshold
		a.summarizeKeepRecent = sc.summarizeKeepRecent
	}

	if len(sc.tools) > 0 {
		a.AddTools(sc.tools...)
	}

	if sc.costBudget != nil {
		a.costBudget = sc.costBudget
	}

	if sc.toolValidator != nil {
		a.toolValidator = sc.toolValidator
	}
	if sc.confidenceThreshold > 0 {
		a.confidenceThreshold = sc.confidenceThreshold
	}

	if sc.piiConfig != nil {
		if sc.piiConfig.RedactInput {
			a.inputGuards = append(a.inputGuards, piiInputGuardrail(sc.piiConfig))
		}
		if sc.piiConfig.BlockOutput {
			a.outputGuards = append(a.outputGuards, piiOutputGuardrail(sc.piiConfig))
		}
	}

	if !sc.unsafe {
		a.HallucinationGuard()
	}

	return a
}

// ── Grouping helpers ─────────────────────────────────────────

// Tools groups multiple ToolDefs into a single Option.
// Use when you have many tools:
//
//	agent := agnogo.Agent("You are helpful.", agnogo.Tools(t1, t2, t3, t4, t5), agnogo.Debug)
func Tools(tools ...ToolDef) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.tools = append(sc.tools, tools...)
	})
}

// ── With*() options (for things that need parameters) ────────

// WithPromptFunc sets a dynamic system prompt that can change per session.
// Overrides the static instructions string.
//
//	agent := agnogo.Agent("default prompt", agnogo.WithPromptFunc(func(s *Session) string {
//	    return "You are helping user " + s.GetMemory("name")
//	}))
func WithPromptFunc(fn func(session *Session) string) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.PromptFunc = fn
	})
}

// WithModel sets a specific model provider, bypassing auto-detection.
func WithModel(p ModelProvider) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.Model = p
	})
}

// WithTools adds tool definitions. Also works by passing tools directly to Agent().
func WithTools(tools ...ToolDef) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.tools = append(sc.tools, tools...)
	})
}

// WithStorage sets a storage backend for session persistence.
func WithStorage(s Storage) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.Storage = s
	})
}

// WithKnowledge sets the knowledge base for RAG-style retrieval.
func WithKnowledge(k Knowledge) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.Knowledge = k
	})
}

// WithMaxLoops sets the maximum number of tool-calling loops per Run.
func WithMaxLoops(n int) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.MaxLoops = n
	})
}

// WithTrace sets observability trace hooks.
func WithTrace(t *Trace) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.Trace = t
	})
}

// Deprecated: use Debug variable instead.
func WithDebug() Option { return Debug }

// Deprecated: use Memory variable instead.
func WithMemory() Option { return Memory }

// Deprecated: use UnsafeMode variable instead.
func Unsafe() Option { return UnsafeMode }

// Deprecated: use Reasoning variable instead.
func WithReasoning() Option { return Reasoning }

// ── Provider-specific options ───────────────────────────────

// WithOpenAI selects OpenAI. Default model: "gpt-4.1-mini".
func WithOpenAI(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "gpt-4.1-mini"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			panic("agnogo: OPENAI_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.openai.com/v1")
	})
}

// WithAnthropic selects Anthropic Claude. Default model: "claude-sonnet-4-5-20250514".
func WithAnthropic(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "claude-sonnet-4-5-20250514"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("ANTHROPIC_API_KEY")
		if key == "" {
			panic("agnogo: ANTHROPIC_API_KEY not set")
		}
		sc.Model = newAnthropicInline(key, m, "https://api.anthropic.com/v1/messages")
	})
}

// WithGemini selects Google Gemini. Default model: "gemini-2.0-flash".
func WithGemini(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "gemini-2.0-flash"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			panic("agnogo: GEMINI_API_KEY not set")
		}
		sc.Model = newGeminiInline(key, m, "https://generativelanguage.googleapis.com/v1beta")
	})
}

// WithGroq selects Groq. Default model: "llama-3.3-70b-versatile".
func WithGroq(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "llama-3.3-70b-versatile"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			panic("agnogo: GROQ_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.groq.com/openai/v1")
	})
}

// WithDeepSeek selects DeepSeek. Default model: "deepseek-chat".
func WithDeepSeek(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "deepseek-chat"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("DEEPSEEK_API_KEY")
		if key == "" {
			panic("agnogo: DEEPSEEK_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.deepseek.com/v1")
	})
}

// WithMistral selects Mistral. Default model: "mistral-large-latest".
func WithMistral(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "mistral-large-latest"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("MISTRAL_API_KEY")
		if key == "" {
			panic("agnogo: MISTRAL_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.mistral.ai/v1")
	})
}

// WithTogether selects Together AI.
func WithTogether(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("TOGETHER_API_KEY")
		if key == "" {
			panic("agnogo: TOGETHER_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.together.xyz/v1")
	})
}

// WithPerplexity selects Perplexity. Default model: "sonar".
func WithPerplexity(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "sonar"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("PERPLEXITY_API_KEY")
		if key == "" {
			panic("agnogo: PERPLEXITY_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.perplexity.ai")
	})
}

// WithGrok selects Grok (xAI). Default model: "grok-3-mini-fast".
func WithGrok(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "grok-3-mini-fast"
		if len(model) > 0 {
			m = model[0]
		}
		key := os.Getenv("GROK_API_KEY")
		if key == "" {
			panic("agnogo: GROK_API_KEY not set")
		}
		sc.Model = newOpenAICompat(key, m, "https://api.x.ai/v1")
	})
}

// WithOllama selects a local Ollama instance. Default model: "llama3.1".
func WithOllama(model ...string) Option {
	return optionFunc(func(sc *smartConfig) {
		m := "llama3.1"
		if len(model) > 0 {
			m = model[0]
		}
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://localhost:11434"
		}
		sc.Model = newOllamaInline(m, host)
	})
}
