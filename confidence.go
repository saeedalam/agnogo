package agnogo

import (
	"strings"
)

// ConfidenceScore represents how confident we should be in a response.
type ConfidenceScore struct {
	Score      float64  `json:"score"`       // 0.0 to 1.0
	Reasons    []string `json:"reasons"`     // why this score
	ToolBacked bool     `json:"tool_backed"` // response used tool data
	Sources    []string `json:"sources"`     // tool names that provided data
}

// hedgingPhrases are indicators of uncertain responses.
var hedgingPhrases = []string{
	"i think",
	"probably",
	"might",
	"not sure",
	"i believe",
}

// factualKeywords suggest the question expects a factual answer.
var factualKeywords = []string{
	"what",
	"when",
	"how much",
	"current",
}

// ScoreConfidence evaluates how confident we should be in a response.
func ScoreConfidence(resp *Response, session *Session, toolCount int) ConfidenceScore {
	score := 0.5
	var reasons []string
	var sources []string
	toolBacked := false

	// Tool-backed response
	if toolCount > 0 {
		score += 0.3
		toolBacked = true
		sources = resp.ToolsCalled
		reasons = append(reasons, "response backed by tool data")
	}

	// Multiple tools used
	if toolCount > 1 {
		score += 0.1
		reasons = append(reasons, "multiple tools corroborated")
	}

	// Short/direct response
	if len(resp.Text) < 200 {
		score += 0.05
		reasons = append(reasons, "concise response")
	}

	// Hedging language
	lower := strings.ToLower(resp.Text)
	hedgePenalty := 0.0
	for _, phrase := range hedgingPhrases {
		if strings.Contains(lower, phrase) {
			hedgePenalty += 0.1
			reasons = append(reasons, "hedging: \""+phrase+"\"")
		}
	}
	if hedgePenalty > 0.2 {
		hedgePenalty = 0.2
	}
	score -= hedgePenalty

	// No tools on factual question
	if toolCount == 0 {
		// Check the last user message in session history
		userMsg := lastUserMessage(session)
		lowerMsg := strings.ToLower(userMsg)
		for _, kw := range factualKeywords {
			if strings.Contains(lowerMsg, kw) {
				score -= 0.3
				reasons = append(reasons, "factual question without tool verification")
				break
			}
		}
	}

	// Clamp
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return ConfidenceScore{
		Score:      score,
		Reasons:    reasons,
		ToolBacked: toolBacked,
		Sources:    sources,
	}
}

// lastUserMessage returns the last user message from session history.
func lastUserMessage(session *Session) string {
	if session == nil {
		return ""
	}
	history := session.GetHistory()
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[i].Content
		}
	}
	return ""
}

// ── WithConfidence Option ───────────────────────────────────

// WithConfidence sets a minimum confidence threshold.
// Responses below this score trigger an automatic retry with tool instructions.
func WithConfidence(minScore float64) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.confidenceThreshold = minScore
	})
}
