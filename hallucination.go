package agnogo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// HallucinationGuard detects when the LLM generates information that
// a registered tool should have provided. If the response contains
// real-time data (dates, times, weather, prices) but no tool was called,
// it blocks the response and forces a retry.
//
// Usage:
//
//	agent := agnogo.New(agnogo.Config{...})
//	agent.Tool("get_time", "Get current date and time", ...)
//	agent.HallucinationGuard() // enables automatic detection
//
// Or with custom patterns:
//
//	agent.HallucinationGuardWithPatterns([]string{`\b202\d\b`, `\d+\s*SEK`})
type hallucinationDetector struct {
	tools    *ToolRegistry
	patterns []*regexp.Regexp
}

// Built-in patterns that indicate real-time/factual claims
var defaultHallucinationPatterns = []string{
	// Dates that look specific: "March 29, 2026", "2026-03-29", "29/03/2026"
	`\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+\d{4}\b`,
	`\b\d{4}[-/]\d{2}[-/]\d{2}\b`,
	`\b\d{1,2}[-/]\d{1,2}[-/]\d{4}\b`,
	// Times: "14:30", "2:30 PM"
	`\b\d{1,2}:\d{2}\s*(AM|PM|am|pm)?\b`,
	// Weather claims: "22°C", "sunny", "partly cloudy"
	`\b-?\d+\s*°[CF]\b`,
	// Currency amounts: "$100", "350 SEK", "€50"
	`\b[$€£¥]\s*[\d,.]+\b`,
	`\b[\d,.]+\s*(SEK|USD|EUR|GBP|NOK|DKK|kr)\b`,
	// Stock prices, percentages with context
	`\b(current|today|now|right now|at the moment)\b`,
}

// HallucinationGuard enables automatic hallucination detection.
// Blocks responses that contain real-time information when no tools were called.
func (a *Agent) HallucinationGuard() *Agent {
	return a.addHallucinationGuard(nil)
}

// HallucinationGuardWithPatterns enables hallucination detection with custom regex patterns.
func (a *Agent) HallucinationGuardWithPatterns(extraPatterns []string) *Agent {
	return a.addHallucinationGuard(extraPatterns)
}

func (a *Agent) addHallucinationGuard(extraPatterns []string) *Agent {
	allPatterns := make([]string, 0, len(defaultHallucinationPatterns)+len(extraPatterns))
	allPatterns = append(allPatterns, defaultHallucinationPatterns...)
	allPatterns = append(allPatterns, extraPatterns...)

	compiled := make([]*regexp.Regexp, 0, len(allPatterns))
	for _, p := range allPatterns {
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			compiled = append(compiled, re)
		}
	}

	detector := &hallucinationDetector{
		tools:    a.tools,
		patterns: compiled,
	}

	a.outputGuards = append(a.outputGuards, Guardrail{
		Name: "hallucination-guard",
		Check: func(ctx context.Context, session *Session, msg string) error {
			return detector.check(ctx, session, msg)
		},
	})

	return a
}

func (d *hallucinationDetector) check(ctx context.Context, session *Session, msg string) error {
	// Only trigger if agent has tools registered
	if d.tools.Count() == 0 {
		return nil
	}

	// Check if any tools were called in this turn
	// Look at the last few messages for tool results
	history := session.GetHistory()
	toolsCalledThisTurn := false
	for i := len(history) - 1; i >= 0 && i >= len(history)-10; i-- {
		if history[i].Role == "tool" {
			toolsCalledThisTurn = true
			break
		}
		// Stop looking back past the last user message
		if history[i].Role == "user" {
			break
		}
	}

	// If tools were called, trust the response
	if toolsCalledThisTurn {
		return nil
	}

	// Check if response contains patterns that suggest hallucinated real-time data
	matches := d.findMatches(msg)
	if len(matches) == 0 {
		return nil
	}

	// Check which tools might be relevant
	relevantTools := d.findRelevantTools(msg)
	if len(relevantTools) == 0 {
		return nil // no tools would help, allow it
	}

	return fmt.Errorf("I need to verify that information. Let me check using my tools. [hallucination-guard: detected %s, should use: %s]",
		strings.Join(matches, ", "),
		strings.Join(relevantTools, ", "))
}

func (d *hallucinationDetector) findMatches(msg string) []string {
	var matches []string
	seen := map[string]bool{}
	for _, re := range d.patterns {
		if m := re.FindString(msg); m != "" && !seen[m] {
			matches = append(matches, m)
			seen[m] = true
			if len(matches) >= 3 {
				break
			}
		}
	}
	return matches
}

// findRelevantTools checks which registered tools could provide the information
// the model is trying to answer about.
func (d *hallucinationDetector) findRelevantTools(msg string) []string {
	lower := strings.ToLower(msg)
	var relevant []string

	for _, t := range d.tools.List() {
		toolDesc := strings.ToLower(t.Name + " " + t.Description)

		// Date/time tool matches date patterns
		if (strings.Contains(toolDesc, "time") || strings.Contains(toolDesc, "date")) &&
			containsAny(lower, "today", "yesterday", "tomorrow", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december", "2024", "2025", "2026") {
			relevant = append(relevant, t.Name)
		}

		// Weather tool matches weather patterns
		if strings.Contains(toolDesc, "weather") &&
			containsAny(lower, "°", "sunny", "cloudy", "rain", "snow", "wind", "temperature", "forecast") {
			relevant = append(relevant, t.Name)
		}

		// Price/cost tool matches currency patterns
		if (strings.Contains(toolDesc, "price") || strings.Contains(toolDesc, "cost")) &&
			containsAny(lower, "$", "€", "£", "sek", "usd", "eur", "kr", "cost", "price") {
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
