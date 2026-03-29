package agnogo

import (
	"context"
	"encoding/json"
	"strings"
)

// MemoryExtractor extracts facts from conversation.
// Called after each turn if AutoMemory is enabled.
type MemoryExtractor interface {
	Extract(ctx context.Context, session *Session, userMessage, assistantReply string)
}

// PatternMemory extracts facts using string pattern matching (no LLM call).
// Fast and free — handles common patterns like "My name is X" and email addresses.
type PatternMemory struct {
	// NamePrefixes to detect name statements. Defaults to English + Swedish.
	NamePrefixes []string
}

// DefaultPatternMemory returns a memory extractor with common patterns.
func DefaultPatternMemory() *PatternMemory {
	return &PatternMemory{
		NamePrefixes: []string{
			"my name is ", "i'm ", "i am ", "this is ",
			"jag heter ", "mitt namn är ", "hej jag är ", "det är ",
		},
	}
}

func (m *PatternMemory) Extract(_ context.Context, session *Session, userMessage, _ string) {
	msg := strings.ToLower(userMessage)

	// Name extraction
	for _, prefix := range m.NamePrefixes {
		if strings.HasPrefix(msg, prefix) {
			name := strings.TrimSpace(userMessage[len(prefix):])
			for _, stop := range []string{",", ".", "!", " and ", " och "} {
				if idx := strings.Index(strings.ToLower(name), stop); idx > 0 {
					name = name[:idx]
				}
			}
			name = strings.TrimSpace(name)
			if name != "" && len(name) < 50 {
				session.SetMemory("name", name)
			}
			break
		}
	}

	// Email extraction
	if strings.Contains(msg, "@") && strings.Contains(msg, ".") {
		for _, w := range strings.Fields(userMessage) {
			if strings.Contains(w, "@") && strings.Contains(w, ".") {
				session.SetMemory("email", strings.Trim(w, ".,!?;:"))
				break
			}
		}
	}
}

// LLMMemory uses the model to extract structured facts. More accurate, costs tokens.
type LLMMemory struct {
	Model  ModelProvider
	Fields []string // fields to extract, e.g. ["name", "email", "preference"]
}

func (m *LLMMemory) Extract(ctx context.Context, session *Session, _, _ string) {
	if len(session.History) < 2 || m.Model == nil {
		return
	}

	fields := m.Fields
	if len(fields) == 0 {
		fields = []string{"name", "email", "phone", "preference", "language"}
	}

	// Use last 4 messages for extraction
	start := len(session.History) - 4
	if start < 0 {
		start = 0
	}

	msgs := []Message{
		{Role: "system", Content: "Extract key facts about the user. Return JSON: {" + strings.Join(fields, ", ") + "}. Only include found fields. Return {} if none."},
	}
	msgs = append(msgs, session.History[start:]...)

	resp, err := m.Model.ChatCompletion(ctx, msgs, nil)
	if err != nil || resp.Text == "" {
		return
	}

	text := strings.TrimSpace(resp.Text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	var facts map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &facts); err != nil {
		return
	}
	for k, v := range facts {
		if v != "" {
			session.SetMemory(k, v)
		}
	}
}
