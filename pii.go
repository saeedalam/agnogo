package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// PIIType identifies a category of personally identifiable information.
type PIIType string

const (
	PIIEmail      PIIType = "email"
	PIIPhone      PIIType = "phone"
	PIICreditCard PIIType = "credit_card"
	PIISSN        PIIType = "ssn"
	PIIIPAddress  PIIType = "ip_address"
	PIICustom     PIIType = "custom"
)

// PIIMatch describes a detected PII occurrence.
type PIIMatch struct {
	Type     PIIType
	Match    string
	Position int
	Redacted string // the replacement text (e.g. "[EMAIL REDACTED]")
}

// PIIConfig configures PII detection and GDPR compliance.
type PIIConfig struct {
	BlockOutput    bool                     // block responses containing PII
	RedactInput    bool                     // redact PII from stored history
	AllowedTypes   []PIIType               // these PII types are OK (user consented)
	CustomPatterns []PIIPattern            // additional regex patterns
	OnDetected     func(matches []PIIMatch) // audit callback
}

// PIIPattern defines a custom PII regex pattern.
type PIIPattern struct {
	Type    PIIType
	Pattern string // regex
}

// compiled PII patterns, initialized once.
var (
	piiOnce     sync.Once
	piiPatterns []compiledPII
)

type compiledPII struct {
	piiType  PIIType
	re       *regexp.Regexp
	redacted string
}

func initPIIPatterns() {
	piiOnce.Do(func() {
		piiPatterns = []compiledPII{
			{PIIEmail, regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`), "[EMAIL REDACTED]"},
			{PIIPhone, regexp.MustCompile(`\b(?:\+?1[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}\b`), "[PHONE REDACTED]"},
			{PIIPhone, regexp.MustCompile(`\b\+\d{1,3}[-.\s]?\d{4,14}\b`), "[PHONE REDACTED]"},
			{PIICreditCard, regexp.MustCompile(`\b(?:\d[ \-]*?){13,19}\b`), "[CREDIT CARD REDACTED]"},
			{PIISSN, regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), "[SSN REDACTED]"},
			{PIIIPAddress, regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "[IP REDACTED]"},
		}
	})
}

// luhnValid checks whether a numeric string passes the Luhn algorithm.
func luhnValid(number string) bool {
	// Strip spaces and dashes.
	var digits []int
	for _, ch := range number {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, int(ch-'0'))
		} else if ch != ' ' && ch != '-' {
			return false
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

// DetectPII scans text for PII. Returns all matches.
func DetectPII(text string) []PIIMatch {
	return detectPIIWithCustom(text, nil)
}

func detectPIIWithCustom(text string, custom []compiledPII) []PIIMatch {
	initPIIPatterns()
	var matches []PIIMatch

	allPatterns := make([]compiledPII, len(piiPatterns))
	copy(allPatterns, piiPatterns)
	allPatterns = append(allPatterns, custom...)

	for _, p := range allPatterns {
		locs := p.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matched := text[loc[0]:loc[1]]

			// Credit card needs Luhn validation.
			if p.piiType == PIICreditCard {
				if !luhnValid(matched) {
					continue
				}
			}

			matches = append(matches, PIIMatch{
				Type:     p.piiType,
				Match:    matched,
				Position: loc[0],
				Redacted: p.redacted,
			})
		}
	}
	return matches
}

// RedactPII replaces all PII with redaction markers.
func RedactPII(text string) string {
	return RedactPIIExcept(text, nil)
}

// RedactPIIExcept redacts all PII except the specified allowed types.
func RedactPIIExcept(text string, allowed []PIIType) string {
	matches := DetectPII(text)
	if len(matches) == 0 {
		return text
	}

	allowSet := make(map[PIIType]bool, len(allowed))
	for _, t := range allowed {
		allowSet[t] = true
	}

	// Replace from end to start to preserve positions.
	result := text
	// Process in reverse order of position to avoid offset issues.
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if allowSet[m.Type] {
			continue
		}
		end := m.Position + len(m.Match)
		if end > len(result) {
			end = len(result)
		}
		result = result[:m.Position] + m.Redacted + result[end:]
	}
	return result
}

// WithPIIGuard adds PII detection guardrails to the agent.
// If RedactInput is set, user messages are redacted before storage.
// If BlockOutput is set, agent responses containing PII are blocked.
//
//	agent := agnogo.Agent("...", agnogo.WithPIIGuard(agnogo.PIIConfig{
//	    BlockOutput: true,
//	    RedactInput: true,
//	}))
func WithPIIGuard(config PIIConfig) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.piiConfig = &config
	})
}

// compilePIICustom converts PIIConfig custom patterns into compiled regexes.
func compilePIICustom(patterns []PIIPattern) []compiledPII {
	var result []compiledPII
	for _, p := range patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			continue
		}
		redacted := fmt.Sprintf("[%s REDACTED]", strings.ToUpper(string(p.Type)))
		result = append(result, compiledPII{piiType: p.Type, re: re, redacted: redacted})
	}
	return result
}

// piiOutputGuardrail returns a guardrail that blocks responses containing PII.
func piiOutputGuardrail(config *PIIConfig) Guardrail {
	custom := compilePIICustom(config.CustomPatterns)
	allowSet := make(map[PIIType]bool, len(config.AllowedTypes))
	for _, t := range config.AllowedTypes {
		allowSet[t] = true
	}
	return Guardrail{
		Name: "pii-output-guard",
		Check: func(_ context.Context, _ *Session, msg string) error {
			matches := detectPIIWithCustom(msg, custom)
			// Filter out allowed types.
			var blocked []PIIMatch
			for _, m := range matches {
				if !allowSet[m.Type] {
					blocked = append(blocked, m)
				}
			}
			if len(blocked) == 0 {
				return nil
			}
			if config.OnDetected != nil {
				config.OnDetected(blocked)
			}
			return fmt.Errorf("response blocked: contains %d PII item(s)", len(blocked))
		},
	}
}

// piiInputGuardrail returns a guardrail that redacts PII from user input.
func piiInputGuardrail(config *PIIConfig) Guardrail {
	allowSet := make(map[PIIType]bool, len(config.AllowedTypes))
	for _, t := range config.AllowedTypes {
		allowSet[t] = true
	}
	allowed := make([]PIIType, 0, len(config.AllowedTypes))
	for _, t := range config.AllowedTypes {
		allowed = append(allowed, t)
	}
	return Guardrail{
		Name: "pii-input-guard",
		Check: func(_ context.Context, session *Session, msg string) error {
			matches := DetectPII(msg)
			// Filter out allowed types.
			var relevant []PIIMatch
			for _, m := range matches {
				if !allowSet[m.Type] {
					relevant = append(relevant, m)
				}
			}
			if len(relevant) == 0 {
				return nil
			}
			if config.OnDetected != nil {
				config.OnDetected(relevant)
			}
			// Redact the last user message in session history.
			redacted := RedactPIIExcept(msg, allowed)
			session.mu.Lock()
			for i := len(session.History) - 1; i >= 0; i-- {
				if session.History[i].Role == "user" && session.History[i].Content == msg {
					session.History[i].Content = redacted
					break
				}
			}
			session.mu.Unlock()
			return nil // allow the message through (redacted in history)
		},
	}
}

// ── GDPR helpers ────────────────────────────────────────

// PurgeUserData removes all sessions and data for a user.
func PurgeUserData(ctx context.Context, storage Storage, userID string) error {
	if storage == nil {
		return fmt.Errorf("agnogo: storage is nil")
	}
	sessions, err := storage.List(ctx, 0)
	if err != nil {
		return fmt.Errorf("agnogo: list sessions: %w", err)
	}
	for _, s := range sessions {
		if s.UserID == userID {
			if err := storage.Delete(ctx, s.ID); err != nil {
				return fmt.Errorf("agnogo: delete session %s: %w", s.ID, err)
			}
		}
	}
	return nil
}

// ExportUserData exports all sessions for a user as JSON.
func ExportUserData(ctx context.Context, storage Storage, userID string) ([]byte, error) {
	if storage == nil {
		return nil, fmt.Errorf("agnogo: storage is nil")
	}
	sessions, err := storage.List(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("agnogo: list sessions: %w", err)
	}
	var userSessions []*Session
	for _, s := range sessions {
		if s.UserID == userID {
			userSessions = append(userSessions, s)
		}
	}
	return json.Marshal(userSessions)
}

// SetConsent records consent for a specific purpose.
func (s *Session) SetConsent(purpose string, granted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Metadata == nil {
		s.Metadata = map[string]string{}
	}
	if granted {
		s.Metadata["_consent_"+purpose] = "true"
	} else {
		delete(s.Metadata, "_consent_"+purpose)
	}
}

// HasConsent checks whether consent was granted for a specific purpose.
func (s *Session) HasConsent(purpose string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Metadata["_consent_"+purpose] == "true"
}

// ── Pluggable PII Scanner guardrails ────────────────────

// piiOutputGuardrailWithScanner returns a guardrail that uses a custom PIIScanner.
func piiOutputGuardrailWithScanner(scanner PIIScanner, config *PIIConfig) Guardrail {
	return Guardrail{
		Name: "pii-output-guard",
		Check: func(_ context.Context, _ *Session, msg string) error {
			matches := scanner.Detect(msg)
			if len(matches) == 0 {
				return nil
			}
			// Filter allowed types
			allowSet := make(map[PIIType]bool, len(config.AllowedTypes))
			for _, t := range config.AllowedTypes {
				allowSet[t] = true
			}
			var blocked []PIIMatch
			for _, m := range matches {
				if !allowSet[m.Type] {
					blocked = append(blocked, m)
				}
			}
			if len(blocked) == 0 {
				return nil
			}
			if config.OnDetected != nil {
				config.OnDetected(blocked)
			}
			return fmt.Errorf("response blocked: contains %d PII item(s)", len(blocked))
		},
	}
}

// piiInputGuardrailWithScanner returns a guardrail that uses a custom PIIScanner.
func piiInputGuardrailWithScanner(scanner PIIScanner, config *PIIConfig) Guardrail {
	return Guardrail{
		Name: "pii-input-guard",
		Check: func(_ context.Context, session *Session, msg string) error {
			matches := scanner.Detect(msg)
			// Filter allowed types
			allowSet := make(map[PIIType]bool, len(config.AllowedTypes))
			for _, t := range config.AllowedTypes {
				allowSet[t] = true
			}
			var relevant []PIIMatch
			for _, m := range matches {
				if !allowSet[m.Type] {
					relevant = append(relevant, m)
				}
			}
			if len(relevant) == 0 {
				return nil
			}
			if config.OnDetected != nil {
				config.OnDetected(relevant)
			}
			// Redact the last user message in session history
			redacted := scanner.Redact(msg)
			session.mu.Lock()
			for i := len(session.History) - 1; i >= 0; i-- {
				if session.History[i].Role == "user" && session.History[i].Content == msg {
					session.History[i].Content = redacted
					break
				}
			}
			session.mu.Unlock()
			return nil // allow the message through (redacted in history)
		},
	}
}
