package agnogo

// ReliableOption customizes a component of the Reliable() preset.
type ReliableOption func(*reliableConfig)

// reliableConfig holds the pluggable components + budget values.
type reliableConfig struct {
	// Budget values (used to create defaults when custom interfaces are nil)
	maxPerRun     float64
	maxPerSession float64

	// Confidence
	confidenceThreshold float64

	// PII config (used to create default scanner + guardrails)
	piiConfig *PIIConfig

	// Tool validator config (used to create default validator)
	toolValidatorConfig *ToolValidator

	// Pluggable interfaces — nil means use default
	hallucinationChecker HallucinationChecker
	piiScanner           PIIScanner
	toolOutputValidator  ToolOutputValidator
	confidenceScorer     ConfidenceScorer
}

// ── Reliable() — one-liner with pluggable components ────────────────

// Reliable enables all reliability features with sensible defaults.
// Pass ReliableOption values to customize individual components:
//
//	agent := agnogo.Agent("...", agnogo.Reliable())                                    // all defaults
//	agent := agnogo.Agent("...", agnogo.Reliable(agnogo.WithCustomPII(myScanner)))     // custom PII
//	agent := agnogo.Agent("...", agnogo.Reliable(agnogo.WithReliableBudget(0.50, 5)))  // custom budget
func Reliable(opts ...ReliableOption) Option {
	return optionFunc(func(sc *smartConfig) {
		rc := reliableConfig{
			maxPerRun:           1.00,
			maxPerSession:       10.00,
			confidenceThreshold: 0.5,
			piiConfig: &PIIConfig{
				BlockOutput: true,
				RedactInput: true,
			},
			toolValidatorConfig: &ToolValidator{
				MaxOutputSize:   50000,
				RequireNonEmpty: true,
				JSONValidate:    true,
			},
		}
		for _, opt := range opts {
			opt(&rc)
		}

		// Wire cost budget
		sc.costBudget = &CostBudget{
			MaxPerRun:     rc.maxPerRun,
			MaxPerSession: rc.maxPerSession,
		}

		// Wire PII
		sc.piiConfig = rc.piiConfig

		// Wire tool validator
		sc.toolValidator = rc.toolValidatorConfig

		// Wire confidence
		sc.confidenceThreshold = rc.confidenceThreshold

		// Wire pluggable interfaces
		sc.hallucinationChecker = rc.hallucinationChecker
		sc.piiScanner = rc.piiScanner
		sc.toolOutputValidator = rc.toolOutputValidator
		sc.confidenceScorer = rc.confidenceScorer
	})
}

// ReliableWith enables reliability with custom budget limits.
// Deprecated: use Reliable(WithReliableBudget(maxPerRun, maxPerSession)) instead.
//
//	agent := agnogo.Agent("...", agnogo.ReliableWith(0.50, 5.00))
func ReliableWith(maxPerRun, maxPerSession float64) Option {
	return Reliable(WithReliableBudget(maxPerRun, maxPerSession))
}

// ── ReliableOption constructors ─────────────────────────────────────

// WithCustomHallucination replaces the built-in hallucination detector.
//
//	agent := agnogo.Agent("...", agnogo.Reliable(
//	    agnogo.WithCustomHallucination(myDetector),
//	))
func WithCustomHallucination(h HallucinationChecker) ReliableOption {
	return func(rc *reliableConfig) {
		rc.hallucinationChecker = h
	}
}

// WithCustomPII replaces the built-in PII scanner.
//
//	agent := agnogo.Agent("...", agnogo.Reliable(
//	    agnogo.WithCustomPII(myGDPRLib),
//	))
func WithCustomPII(s PIIScanner) ReliableOption {
	return func(rc *reliableConfig) {
		rc.piiScanner = s
	}
}

// WithCustomCost sets a custom cost budget.
// The CostChecker interface is available for custom implementations,
// but is wired through WithBudget() rather than the Run() loop directly.
func WithCustomCost(budget CostBudget) ReliableOption {
	return func(rc *reliableConfig) {
		rc.maxPerRun = budget.MaxPerRun
		rc.maxPerSession = budget.MaxPerSession
	}
}

// WithCustomToolValidator replaces the built-in tool output validator.
func WithCustomToolValidator(v ToolOutputValidator) ReliableOption {
	return func(rc *reliableConfig) {
		rc.toolOutputValidator = v
	}
}

// WithCustomConfidence replaces the built-in confidence scorer.
func WithCustomConfidence(s ConfidenceScorer) ReliableOption {
	return func(rc *reliableConfig) {
		rc.confidenceScorer = s
	}
}

// WithReliableBudget sets custom cost budget limits.
//
//	agnogo.Reliable(agnogo.WithReliableBudget(0.50, 5.00))
func WithReliableBudget(maxPerRun, maxPerSession float64) ReliableOption {
	return func(rc *reliableConfig) {
		rc.maxPerRun = maxPerRun
		rc.maxPerSession = maxPerSession
	}
}

// WithReliableConfidenceThreshold sets the minimum confidence score.
// Responses below this trigger an automatic retry with tool instructions.
//
//	agnogo.Reliable(agnogo.WithReliableConfidenceThreshold(0.7))
func WithReliableConfidenceThreshold(min float64) ReliableOption {
	return func(rc *reliableConfig) {
		rc.confidenceThreshold = min
	}
}

// WithReliablePII customizes PII detection settings within Reliable().
//
//	agnogo.Reliable(agnogo.WithReliablePII(agnogo.PIIConfig{
//	    BlockOutput:  true,
//	    RedactInput:  true,
//	    AllowedTypes: []agnogo.PIIType{agnogo.PIIEmail},
//	}))
func WithReliablePII(config PIIConfig) ReliableOption {
	return func(rc *reliableConfig) {
		rc.piiConfig = &config
	}
}

// WithReliableToolValidation customizes tool output validation within Reliable().
func WithReliableToolValidation(v ToolValidator) ReliableOption {
	return func(rc *reliableConfig) {
		rc.toolValidatorConfig = &v
	}
}
