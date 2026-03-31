package agnogo

// Reliable enables all reliability features with sensible defaults.
// This is the one-line production safety preset.
//
//	agent := agnogo.Agent("You are helpful.", agnogo.Reliable())
//
// Equivalent to:
//   - Cost budget: $1/run, $10/session
//   - PII guard: block PII in output, redact in stored history
//   - Tool validation: non-empty, JSON check, 50KB limit
//   - Confidence threshold: 0.5 (retry if below)
//   - Hallucination guard (already ON by default)
func Reliable() Option {
	return optionFunc(func(sc *smartConfig) {
		// Cost limits
		sc.costBudget = &CostBudget{
			MaxPerRun:     1.00,
			MaxPerSession: 10.00,
		}

		// PII protection
		sc.piiConfig = &PIIConfig{
			BlockOutput: true,
			RedactInput: true,
		}

		// Tool output checks
		sc.toolValidator = &ToolValidator{
			MaxOutputSize:   50000,
			RequireNonEmpty: true,
			JSONValidate:    true,
		}

		// Confidence threshold
		sc.confidenceThreshold = 0.5
	})
}

// ReliableWith enables reliability with custom budget limits.
//
//	agent := agnogo.Agent("...", agnogo.ReliableWith(0.50, 5.00))
func ReliableWith(maxPerRun, maxPerSession float64) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.costBudget = &CostBudget{
			MaxPerRun:     maxPerRun,
			MaxPerSession: maxPerSession,
		}
		sc.piiConfig = &PIIConfig{
			BlockOutput: true,
			RedactInput: true,
		}
		sc.toolValidator = &ToolValidator{
			MaxOutputSize:   50000,
			RequireNonEmpty: true,
			JSONValidate:    true,
		}
		sc.confidenceThreshold = 0.5
	})
}
