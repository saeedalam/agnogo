package agnogo

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// HallucinationSeverity indicates how confident we are that the LLM hallucinated.
type HallucinationSeverity string

const (
	// SeverityLikely means multiple pattern matches or real-time keywords combined
	// with factual claims — the response should be blocked and retried.
	SeverityLikely HallucinationSeverity = "likely"

	// SeverityPossible means a single ambiguous match — a warning is logged
	// but the response is allowed through.
	SeverityPossible HallucinationSeverity = "possible"
)

// HallucinationMatch describes a single pattern match found in LLM output.
type HallucinationMatch struct {
	Pattern  string `json:"pattern"`
	Category string `json:"category"` // "date", "time", "currency", "temperature", "weather", "financial", "relative_time"
	Match    string `json:"match"`    // the actual matched text
	Severity string `json:"severity"` // "likely" or "possible"
}

// hallucinationPattern pairs a compiled regex with its category name.
type hallucinationPattern struct {
	re       *regexp.Regexp
	category string
}

// ── Cached default patterns ──────────────────────────────────────────

var (
	defaultPatterns     []hallucinationPattern
	defaultPatternsOnce sync.Once
)

// defaultPatternDefs maps category → list of regex source strings.
// Every pattern MUST use \b (word boundaries) to avoid false positives.
var defaultPatternDefs = []struct {
	category string
	pattern  string
}{
	// Dates: "March 29, 2026", "2026-03-29", "29/03/2026"
	{"date", `\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+\d{4}\b`},
	{"date", `\b\d{4}[-/]\d{2}[-/]\d{2}\b`},
	{"date", `\b\d{1,2}[-/]\d{1,2}[-/]\d{4}\b`},

	// Times: "14:30", "2:30 PM" — require word boundary before the digit
	{"time", `\b\d{1,2}:\d{2}\s*(?:AM|PM|am|pm)?\b`},

	// Temperature: "22°C", "-5°F"
	{"temperature", `\b-?\d+\s*°[CF]\b`},

	// Currency amounts: "$100", "350 SEK", "€50"
	// Note: $ € £ ¥ are non-word chars so \b before them won't work; use (?:^|\s) instead.
	{"currency", `(?:^|[\s(])[$€£¥]\d[\d,.]*\b`},
	{"currency", `\b\d[\d,.]*\s*(?:SEK|USD|EUR|GBP|NOK|DKK|kr)\b`},

	// Relative time expressions
	{"relative_time", `\b(?:today|tomorrow|yesterday|right now|currently|at the moment|as of now)\b`},
	{"relative_time", `\bnext\s+(?:week|month|year|monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`},
	{"relative_time", `\blast\s+(?:week|month|year)\b`},

	// Weather words
	{"weather", `\b(?:sunny|cloudy|rainy|snowy|foggy|windy|overcast|partly cloudy|clear skies?|thunderstorm|drizzle|hail|sleet|humid|dry)\b`},

	// Financial: "stock price … 142.50", "$AAPL"
	{"financial", `\b(?:stock price|share price|market cap|trading at|up|down)\s+[\d.]+%?`},
	{"financial", `\$[A-Z]{1,5}\b`},
}

func compileDefaultPatterns() {
	defaultPatterns = make([]hallucinationPattern, 0, len(defaultPatternDefs))
	for _, def := range defaultPatternDefs {
		re, err := regexp.Compile("(?i)" + def.pattern)
		if err != nil {
			continue // skip malformed patterns
		}
		defaultPatterns = append(defaultPatterns, hallucinationPattern{
			re:       re,
			category: def.category,
		})
	}
}

func getDefaultPatterns() []hallucinationPattern {
	defaultPatternsOnce.Do(compileDefaultPatterns)
	return defaultPatterns
}

// ── Detector ─────────────────────────────────────────────────────────

// hallucinationDetector runs regex-based hallucination detection on LLM output.
type hallucinationDetector struct {
	tools         *ToolRegistry
	patterns      []hallucinationPattern
	extraPatterns []*regexp.Regexp
}

// ── Public API on Core ───────────────────────────────────────────────

// HallucinationGuard enables automatic hallucination detection.
// Blocks responses that contain real-time information when no tools were called.
func (a *Core) HallucinationGuard() *Core {
	return a.addHallucinationGuard(nil)
}

// HallucinationGuardWithPatterns enables hallucination detection with additional
// custom regex patterns on top of the built-in set.
func (a *Core) HallucinationGuardWithPatterns(extraPatterns []string) *Core {
	return a.addHallucinationGuard(extraPatterns)
}

func (a *Core) addHallucinationGuard(extraPatterns []string) *Core {
	extra := make([]*regexp.Regexp, 0, len(extraPatterns))
	for _, p := range extraPatterns {
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			extra = append(extra, re)
		}
	}

	detector := &hallucinationDetector{
		tools:         a.tools,
		patterns:      getDefaultPatterns(),
		extraPatterns: extra,
	}

	a.outputGuards = append(a.outputGuards, Guardrail{
		Name: "hallucination-guard",
		Check: func(ctx context.Context, session *Session, msg string) error {
			return detector.check(ctx, session, msg)
		},
	})

	return a
}

// ── Detection logic ──────────────────────────────────────────────────

func (d *hallucinationDetector) check(_ context.Context, session *Session, msg string) error {
	// Only trigger if agent has tools registered.
	if d.tools.Count() == 0 {
		return nil
	}

	// Check if any tools were called in this turn.
	history := session.GetHistory()
	toolsCalledThisTurn := false
	for i := len(history) - 1; i >= 0 && i >= len(history)-10; i-- {
		if history[i].Role == "tool" {
			toolsCalledThisTurn = true
			break
		}
		if history[i].Role == "user" {
			break
		}
	}
	if toolsCalledThisTurn {
		return nil
	}

	// Run pattern matching.
	matches := d.findAllMatches(msg)
	if len(matches) == 0 {
		return nil
	}

	// Check which tools might be relevant.
	relevantTools := d.findRelevantTools(msg)
	if len(relevantTools) == 0 {
		return nil
	}

	// Determine severity.
	severity := determineSeverity(matches)

	// Annotate every match with the final severity.
	for i := range matches {
		matches[i].Severity = string(severity)
	}

	if severity == SeverityPossible {
		matchStrs := make([]string, len(matches))
		for i, m := range matches {
			matchStrs[i] = m.Match
		}
		slog.Warn("hallucination-guard: possible hallucination detected",
			"matches", matchStrs,
			"severity", string(severity),
		)
		return nil // allow through
	}

	// severity == SeverityLikely → block and retry
	matchStrs := make([]string, 0, len(matches))
	for _, m := range matches {
		matchStrs = append(matchStrs, m.Match)
	}
	return fmt.Errorf("I need to verify that information. Let me check using my tools. [hallucination-guard: detected %s, should use: %s]",
		strings.Join(matchStrs, ", "),
		strings.Join(relevantTools, ", "))
}

// findAllMatches runs all patterns (built-in + extra) and returns structured results.
func (d *hallucinationDetector) findAllMatches(msg string) []HallucinationMatch {
	var matches []HallucinationMatch
	seen := make(map[string]bool)

	for _, hp := range d.patterns {
		for _, m := range hp.re.FindAllString(msg, -1) {
			if !seen[m] {
				seen[m] = true
				matches = append(matches, HallucinationMatch{
					Pattern:  hp.re.String(),
					Category: hp.category,
					Match:    m,
				})
			}
		}
	}

	for _, re := range d.extraPatterns {
		for _, m := range re.FindAllString(msg, -1) {
			if !seen[m] {
				seen[m] = true
				matches = append(matches, HallucinationMatch{
					Pattern:  re.String(),
					Category: "custom",
					Match:    m,
				})
			}
		}
	}

	return matches
}

// determineSeverity classifies the overall severity from a set of matches.
//
// Rules:
//   - Multiple matches (2+) → likely
//   - A relative_time match combined with any factual category (date, time,
//     currency, temperature, financial) → likely
//   - Single match → possible
func determineSeverity(matches []HallucinationMatch) HallucinationSeverity {
	if len(matches) == 0 {
		return SeverityPossible
	}
	if len(matches) >= 2 {
		return SeverityLikely
	}

	// Single match — check if it's in a high-confidence category.
	cat := matches[0].Category
	if cat == "relative_time" {
		return SeverityLikely // real-time keyword alone is strong signal
	}

	return SeverityPossible
}

// findRelevantTools checks which registered tools could provide the information
// the model is trying to answer about.
func (d *hallucinationDetector) findRelevantTools(msg string) []string {
	lower := strings.ToLower(msg)
	var relevant []string

	for _, t := range d.tools.List() {
		toolDesc := strings.ToLower(t.Name + " " + t.Description)

		if (strings.Contains(toolDesc, "time") || strings.Contains(toolDesc, "date")) &&
			containsAny(lower, "today", "yesterday", "tomorrow", "monday", "tuesday",
				"wednesday", "thursday", "friday", "saturday", "sunday",
				"january", "february", "march", "april", "may", "june",
				"july", "august", "september", "october", "november", "december",
				"2024", "2025", "2026", "2027") {
			relevant = append(relevant, t.Name)
		}

		if strings.Contains(toolDesc, "weather") &&
			containsAny(lower, "°", "sunny", "cloudy", "rain", "snow", "wind",
				"temperature", "forecast", "foggy", "overcast", "humid") {
			relevant = append(relevant, t.Name)
		}

		if (strings.Contains(toolDesc, "price") || strings.Contains(toolDesc, "cost") || strings.Contains(toolDesc, "stock") || strings.Contains(toolDesc, "financial")) &&
			containsAny(lower, "$", "€", "£", "sek", "usd", "eur", "kr",
				"cost", "price", "stock", "market cap", "trading") {
			relevant = append(relevant, t.Name)
		}
	}

	return relevant
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
