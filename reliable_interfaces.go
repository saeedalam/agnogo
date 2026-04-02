package agnogo

import "context"

// ── Pluggable Reliability Interfaces ────────────────────────────────
//
// Each interface represents one component of the reliability layer.
// agnogo ships high-quality default implementations for all of them.
// Users can swap any component with their own via Reliable() options:
//
//	agent := agnogo.Agent("...", agnogo.Reliable(
//	    agnogo.WithCustomPII(myGDPRLib),
//	    agnogo.WithCustomHallucination(myDetector),
//	))

// HallucinationChecker detects when the LLM fabricates information
// instead of using available tools. Return an error to block the response
// and trigger a retry with tool instructions.
type HallucinationChecker interface {
	Check(ctx context.Context, session *Session, response string) error
}

// HallucinationCheckerFunc adapts a function into a HallucinationChecker.
type HallucinationCheckerFunc func(ctx context.Context, session *Session, response string) error

func (f HallucinationCheckerFunc) Check(ctx context.Context, s *Session, resp string) error {
	return f(ctx, s, resp)
}

// PIIScanner detects and redacts personally identifiable information.
// Used for both input redaction and output blocking.
type PIIScanner interface {
	Detect(text string) []PIIMatch
	Redact(text string) string
}

// CostChecker enforces spending limits during agent execution.
// Implement this to integrate with your own billing system.
type CostChecker interface {
	AddUsage(model string, usage *Usage)
	CheckBudget(session *Session) error
	TotalCost() float64
}

// ToolOutputValidator validates tool results before feeding them back
// to the LLM. Can reject oversized, empty, or malformed output.
type ToolOutputValidator interface {
	Validate(toolName, result string) (string, error)
}

// ToolOutputValidatorFunc adapts a function into a ToolOutputValidator.
type ToolOutputValidatorFunc func(toolName, result string) (string, error)

func (f ToolOutputValidatorFunc) Validate(toolName, result string) (string, error) {
	return f(toolName, result)
}

// ConfidenceScorer evaluates how trustworthy a response is.
// Returns a score between 0.0 (no confidence) and 1.0 (fully confident).
type ConfidenceScorer interface {
	Score(resp *Response, session *Session, toolCount int) ConfidenceScore
}

// ConfidenceScorerFunc adapts a function into a ConfidenceScorer.
type ConfidenceScorerFunc func(resp *Response, session *Session, toolCount int) ConfidenceScore

func (f ConfidenceScorerFunc) Score(resp *Response, s *Session, tc int) ConfidenceScore {
	return f(resp, s, tc)
}

// ── Default Implementations ─────────────────────────────────────────

// DefaultPIIScanner wraps the built-in regex-based PII detection.
type DefaultPIIScanner struct {
	AllowedTypes []PIIType
}

func (d *DefaultPIIScanner) Detect(text string) []PIIMatch {
	return detectPIIWithCustom(text, nil)
}

func (d *DefaultPIIScanner) Redact(text string) string {
	return RedactPIIExcept(text, d.AllowedTypes)
}

// DefaultConfidenceScorer wraps the built-in heuristic confidence scoring.
type DefaultConfidenceScorer struct{}

func (d *DefaultConfidenceScorer) Score(resp *Response, session *Session, toolCount int) ConfidenceScore {
	return ScoreConfidence(resp, session, toolCount)
}

// DefaultToolOutputValidator wraps the built-in ToolValidator.
type DefaultToolOutputValidator struct {
	Validator *ToolValidator
}

func (d *DefaultToolOutputValidator) Validate(toolName, result string) (string, error) {
	return d.Validator.validateToolOutput(toolName, result)
}
