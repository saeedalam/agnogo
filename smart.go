package agnogo

import (
	"fmt"
	"os"
)

// Option is a functional option for the Core convenience constructor.
type Option func(*smartConfig)

// smartConfig is an internal configuration struct that extends Config
// with fields used only by the Agent() constructor (e.g., tools, unsafe flag).
type smartConfig struct {
	Config
	tools  []ToolDef
	unsafe bool
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
// This is called from the autodetect package's init() functions.
// envVar is the environment variable to check (e.g. "OPENAI_API_KEY"),
// defaultModel is the model to use when auto-detected, and factory
// creates a ModelProvider given the API key.
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
		return nil, fmt.Errorf("agnogo: no providers registered; import _ \"github.com/saeedalam/agnogo/autodetect\"")
	}
	var tried []string
	for _, rp := range providerRegistry {
		tried = append(tried, rp.envVar)
	}
	return nil, fmt.Errorf("agnogo: no API key found; set one of: %v", tried)
}

// Agent creates an agent with smart defaults. It auto-detects the model
// provider from environment variables if none is provided via WithModel.
//
// Safe defaults are ON by default:
//   - Retry with DefaultRetryConfig()
//   - History trimming with DefaultHistoryConfig()
//   - HallucinationGuard enabled
//
// Use Unsafe() to disable all safe defaults.
//
//	a := agnogo.Agent("You are a helpful assistant.")
//	a := agnogo.Agent("You are a coder.", agnogo.WithTools(myTools...), agnogo.WithDebug())
func Agent(instructions string, opts ...Option) *Core {
	sc := smartConfig{
		Config: Config{
			Instructions: instructions,
		},
	}

	for _, opt := range opts {
		opt(&sc)
	}

	// Auto-detect provider if none was set.
	if sc.Model == nil {
		p, err := DetectProvider()
		if err != nil {
			panic(err)
		}
		sc.Model = p
	}

	// Apply safe defaults unless Unsafe() was used.
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

	// Add tools if any were provided.
	if len(sc.tools) > 0 {
		a.AddTools(sc.tools...)
	}

	// Enable hallucination guard unless unsafe mode.
	if !sc.unsafe {
		a.HallucinationGuard()
	}

	return a
}

// WithModel sets a specific model provider, bypassing auto-detection.
func WithModel(p ModelProvider) Option {
	return func(sc *smartConfig) {
		sc.Model = p
	}
}

// WithTools adds tool definitions to the agent.
func WithTools(tools ...ToolDef) Option {
	return func(sc *smartConfig) {
		sc.tools = append(sc.tools, tools...)
	}
}

// WithStorage sets a storage backend for session persistence.
func WithStorage(s Storage) Option {
	return func(sc *smartConfig) {
		sc.Storage = s
	}
}

// WithKnowledge sets the knowledge base for RAG-style retrieval.
func WithKnowledge(k Knowledge) Option {
	return func(sc *smartConfig) {
		sc.Knowledge = k
	}
}

// WithMemory enables automatic pattern-based memory extraction.
func WithMemory() Option {
	return func(sc *smartConfig) {
		sc.AutoMemory = true
	}
}

// WithDebug enables debug output at the default level.
func WithDebug() Option {
	return func(sc *smartConfig) {
		dbg := DefaultDebug()
		sc.Debug = &dbg
	}
}

// WithMaxLoops sets the maximum number of tool-calling loops per Run.
func WithMaxLoops(n int) Option {
	return func(sc *smartConfig) {
		sc.MaxLoops = n
	}
}

// WithReasoning enables chain-of-thought reasoning before the agent responds.
func WithReasoning() Option {
	return func(sc *smartConfig) {
		sc.Reasoning = &ReasoningConfig{}
	}
}

// WithTrace sets observability trace hooks.
func WithTrace(t *Trace) Option {
	return func(sc *smartConfig) {
		sc.Trace = t
	}
}

// Unsafe disables all safe defaults (retry, history trimming, hallucination guard).
func Unsafe() Option {
	return func(sc *smartConfig) {
		sc.unsafe = true
	}
}
