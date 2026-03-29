package agnogo

// HistoryConfig controls conversation history management.
// Prevents context window overflow for long conversations.
type HistoryConfig struct {
	MaxMessages      int // max messages to keep (0 = unlimited)
	MaxToolMessages  int // max tool result messages (default 20)
	SummaryThreshold int // summarize when history exceeds this (0 = never)
}

// DefaultHistoryConfig returns defaults that prevent context overflow.
func DefaultHistoryConfig() HistoryConfig {
	return HistoryConfig{
		MaxMessages:     50,
		MaxToolMessages: 20,
	}
}

// trimHistory applies history limits to a message list.
// Keeps the system message, trims middle messages, preserves recent ones.
func trimHistory(messages []Message, cfg HistoryConfig) []Message {
	if cfg.MaxMessages <= 0 || len(messages) <= cfg.MaxMessages {
		return messages
	}

	// Always keep system message (first) + most recent messages
	system := messages[0]
	recent := messages[len(messages)-cfg.MaxMessages+1:]

	result := make([]Message, 0, cfg.MaxMessages)
	result = append(result, system)

	// Add a summary marker if we trimmed
	if len(messages) > cfg.MaxMessages {
		result = append(result, Message{
			Role:    "system",
			Content: "[Earlier conversation trimmed for context. Recent messages follow.]",
		})
	}

	result = append(result, recent...)
	return result
}

// trimToolMessages removes old tool result messages to save context space.
// Tool results are the largest messages — trimming them helps most.
func trimToolMessages(messages []Message, maxTool int) []Message {
	if maxTool <= 0 {
		return messages
	}

	// Count tool messages from the end
	toolCount := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			toolCount++
		}
	}
	if toolCount <= maxTool {
		return messages
	}

	// Remove oldest tool messages
	toRemove := toolCount - maxTool
	removed := 0
	result := make([]Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" && removed < toRemove {
			removed++
			continue
		}
		result = append(result, m)
	}
	return result
}
