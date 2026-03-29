package agnogo

import (
	"context"
	"strings"
)

// Knowledge provides RAG search for the agent.
// When set, the agent auto-searches before responding to questions.
type Knowledge interface {
	Search(ctx context.Context, query string, limit int) (string, error)
}

// KnowledgeFunc wraps a function as Knowledge.
//
//	k := agnogo.KnowledgeFunc(func(ctx, query, limit) (string, error) {
//	    return myRAGService.Search(ctx, query, limit)
//	})
type KnowledgeFunc func(ctx context.Context, query string, limit int) (string, error)

func (f KnowledgeFunc) Search(ctx context.Context, query string, limit int) (string, error) {
	return f(ctx, query, limit)
}

// injectKnowledge auto-searches for questions and prepends context.
func injectKnowledge(ctx context.Context, k Knowledge, query string, messages []Message, limit int) []Message {
	if k == nil || !looksLikeQuestion(query) {
		return messages
	}

	result, err := k.Search(ctx, query, limit)
	if err != nil || result == "" {
		return messages
	}

	// Insert knowledge before the last message (the user's question)
	knowledgeMsg := Message{
		Role:    "system",
		Content: "RELEVANT CONTEXT (use this to answer):\n" + result,
	}

	n := len(messages)
	if n == 0 {
		return []Message{knowledgeMsg}
	}

	out := make([]Message, 0, n+1)
	out = append(out, messages[:n-1]...)
	out = append(out, knowledgeMsg)
	out = append(out, messages[n-1])
	return out
}

// looksLikeQuestion is a simple heuristic — override with custom logic if needed.
func looksLikeQuestion(msg string) bool {
	lower := strings.ToLower(strings.TrimSpace(msg))
	if strings.Contains(lower, "?") {
		return true
	}
	// Common question starters (English, Swedish, and more)
	prefixes := []string{
		"what ", "how ", "where ", "when ", "who ", "which ", "can ", "do ", "does ", "is ", "are ",
		"vad ", "hur ", "var ", "när ", "vem ", "vilken ", "kan ", "har ", "finns ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}
