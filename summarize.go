package agnogo

import (
	"context"
	"fmt"
	"strings"
)

// SummarizeSession replaces old history messages with a model-generated summary.
// It keeps the system message (if any) and the most recent keepRecent messages,
// replacing everything in between with a single summary message.
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
		{Role: "system", Content: "You are a conversation summarizer. Produce a concise summary that preserves key facts, decisions, user preferences, and important context. Output only the summary, no preamble."},
		{Role: "user", Content: fmt.Sprintf("Summarize this conversation:\n\n%s", sb.String())},
	}

	resp, err := agent.model.ChatCompletion(ctx, prompt, nil)
	if err != nil {
		return fmt.Errorf("agnogo: summarize failed: %w", err)
	}

	summaryText := strings.TrimSpace(resp.Text)
	if summaryText == "" {
		return fmt.Errorf("agnogo: summarize returned empty text")
	}

	summaryMsg := Message{
		Role:    "system",
		Content: fmt.Sprintf("[Summary of earlier conversation]\n%s", summaryText),
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
