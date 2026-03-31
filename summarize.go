package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SessionSummary holds a structured summary of a conversation.
type SessionSummary struct {
	Text     string   `json:"text"`      // narrative summary
	Topics   []string `json:"topics"`    // extracted topics/tags
	KeyFacts []string `json:"key_facts"` // important facts to remember
}

// SummarizeSession replaces old history messages with a model-generated summary.
// It keeps the system message (if any) and the most recent keepRecent messages,
// replacing everything in between with a single summary message.
// The structured summary is also stored in session.State["_summary"].
//
//	agnogo.SummarizeSession(ctx, agent, session, 10)
func SummarizeSession(ctx context.Context, agent *Core, session *Session, keepRecent int) error {
	session.mu.Lock()
	history := session.History
	session.mu.Unlock()

	if keepRecent <= 0 {
		keepRecent = 10
	}
	if len(history) <= keepRecent {
		return nil // nothing to summarize
	}

	// Separate system messages from conversation
	var systemMsgs []Message
	var convMsgs []Message
	for _, m := range history {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			convMsgs = append(convMsgs, m)
		}
	}

	if len(convMsgs) <= keepRecent {
		return nil
	}

	// Split into old (to summarize) and recent (to keep)
	cutoff := len(convMsgs) - keepRecent
	oldMsgs := convMsgs[:cutoff]
	recentMsgs := convMsgs[cutoff:]

	// Format old messages for the summarization prompt
	var sb strings.Builder
	for _, m := range oldMsgs {
		if m.Role == "tool" {
			sb.WriteString(fmt.Sprintf("[tool result]: %s\n", truncateStr(m.Content, 200)))
		} else {
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
		}
	}

	prompt := []Message{
		{Role: "system", Content: "You are a conversation summarizer. You MUST respond with valid JSON only, no preamble or markdown fences."},
		{Role: "user", Content: fmt.Sprintf(`Analyze this conversation and produce JSON with:
- "text": a concise narrative summary preserving key decisions and context
- "topics": list of topic tags (3-8 tags)
- "key_facts": list of important facts mentioned (names, dates, preferences, decisions)

Conversation:
%s`, sb.String())},
	}

	resp, err := agent.model.ChatCompletion(ctx, prompt, nil)
	if err != nil {
		return fmt.Errorf("agnogo: summarize failed: %w", err)
	}

	rawText := strings.TrimSpace(resp.Text)
	if rawText == "" {
		return fmt.Errorf("agnogo: summarize returned empty text")
	}

	// Parse structured summary from JSON response
	summary := parseSummaryResponse(rawText)

	// Store structured summary in session state
	session.Set("_summary", summary)

	summaryMsg := Message{
		Role:    "system",
		Content: fmt.Sprintf("[Summary of earlier conversation]\n%s", summary.Text),
	}

	// Rebuild history: system messages + summary + recent messages
	var newHistory []Message
	newHistory = append(newHistory, systemMsgs...)
	newHistory = append(newHistory, summaryMsg)
	newHistory = append(newHistory, recentMsgs...)

	session.mu.Lock()
	session.History = newHistory
	session.mu.Unlock()

	return nil
}

// parseSummaryResponse attempts to parse a JSON SessionSummary from the model's
// response. If JSON parsing fails, it falls back to treating the entire text
// as a narrative summary with no topics or key facts.
func parseSummaryResponse(text string) *SessionSummary {
	// Strip markdown code fences if present
	cleaned := text
	if strings.HasPrefix(cleaned, "```") {
		if idx := strings.Index(cleaned[3:], "\n"); idx >= 0 {
			cleaned = cleaned[3+idx+1:]
		}
		if strings.HasSuffix(cleaned, "```") {
			cleaned = cleaned[:len(cleaned)-3]
		}
		cleaned = strings.TrimSpace(cleaned)
	}

	var summary SessionSummary
	if err := json.Unmarshal([]byte(cleaned), &summary); err != nil {
		// Fallback: use the raw text as the narrative summary
		return &SessionSummary{
			Text:     text,
			Topics:   nil,
			KeyFacts: nil,
		}
	}

	return &summary
}

// RecallFromSummary searches the session's stored summary for information relevant
// to the given query. It uses the model to answer based on the summary context.
// Returns an empty string if no summary is stored.
func RecallFromSummary(ctx context.Context, agent *Core, session *Session, query string) (string, error) {
	raw := session.Get("_summary")
	if raw == nil {
		return "", nil
	}

	var summaryText string
	switch v := raw.(type) {
	case *SessionSummary:
		summaryText = formatSummaryForRecall(v)
	case map[string]any:
		// Handle case where summary was deserialized from JSON (e.g. from storage)
		s := &SessionSummary{}
		if t, ok := v["text"].(string); ok {
			s.Text = t
		}
		if topics, ok := v["topics"].([]any); ok {
			for _, topic := range topics {
				if ts, ok := topic.(string); ok {
					s.Topics = append(s.Topics, ts)
				}
			}
		}
		if facts, ok := v["key_facts"].([]any); ok {
			for _, fact := range facts {
				if fs, ok := fact.(string); ok {
					s.KeyFacts = append(s.KeyFacts, fs)
				}
			}
		}
		summaryText = formatSummaryForRecall(s)
	default:
		return "", nil
	}

	if summaryText == "" {
		return "", nil
	}

	prompt := []Message{
		{Role: "system", Content: "You are a memory recall assistant. Answer concisely based only on the provided conversation summary. If the summary does not contain relevant information, say so."},
		{Role: "user", Content: fmt.Sprintf("Based on this conversation summary, what do you know about: %s\n\nSummary:\n%s", query, summaryText)},
	}

	resp, err := agent.model.ChatCompletion(ctx, prompt, nil)
	if err != nil {
		return "", fmt.Errorf("agnogo: recall failed: %w", err)
	}

	return strings.TrimSpace(resp.Text), nil
}

// formatSummaryForRecall formats a SessionSummary into a text block for recall queries.
func formatSummaryForRecall(s *SessionSummary) string {
	var sb strings.Builder
	sb.WriteString(s.Text)
	if len(s.Topics) > 0 {
		sb.WriteString("\n\nTopics: ")
		sb.WriteString(strings.Join(s.Topics, ", "))
	}
	if len(s.KeyFacts) > 0 {
		sb.WriteString("\n\nKey facts:\n")
		for _, f := range s.KeyFacts {
			sb.WriteString("- ")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// WithSummarize enables auto-summarization when history exceeds threshold messages.
// keepRecent is the number of recent messages to preserve (default 10).
//
//	agent := agnogo.Agent("...", agnogo.WithSummarize(30))
//	agent := agnogo.Agent("...", agnogo.WithSummarize(30, 15))
func WithSummarize(threshold int, keepRecent ...int) Option {
	kr := 10
	if len(keepRecent) > 0 && keepRecent[0] > 0 {
		kr = keepRecent[0]
	}
	return optionFunc(func(sc *smartConfig) {
		sc.summarizeThreshold = threshold
		sc.summarizeKeepRecent = kr
	})
}
