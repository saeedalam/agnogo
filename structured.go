package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RunStructured runs the agent and parses the response into a Go struct.
// Forces the model to return JSON matching the provided schema.
//
//	type BookingResult struct {
//	    Service string `json:"service"`
//	    Date    string `json:"date"`
//	    Time    string `json:"time"`
//	}
//	var result BookingResult
//	err := agnogo.RunStructured(ctx, agent, session, "Book a haircut tomorrow at 14:00", &result)
//	// result.Service == "Herrklippning", result.Date == "2026-04-01", etc.
func RunStructured[T any](ctx context.Context, agent *Core, session *Session, message string, out *T) error {
	// Build JSON instruction as a separate user message prefix (no agent mutation)
	schema, _ := json.Marshal(out)
	instruction := fmt.Sprintf("RESPONSE FORMAT: You MUST respond with valid JSON matching this structure: %s\nReturn ONLY the JSON, no other text.", string(schema))

	// Inject as a system-level message in the session history temporarily
	// We prepend the instruction to the user message to avoid mutating the agent
	augmented := instruction + "\n\n" + message

	resp, err := agent.Run(ctx, session, augmented)
	if err != nil {
		return err
	}

	// Parse JSON from response
	text := extractJSON(resp.Text)

	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("failed to parse structured response: %w (raw: %s)", err, truncateStr(text, 200))
	}
	return nil
}

// extractJSON strips markdown code blocks and finds JSON content.
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// Strip markdown code blocks
	if strings.HasPrefix(text, "```") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 1 {
			rest := lines[1]
			if idx := strings.LastIndex(rest, "```"); idx > 0 {
				text = strings.TrimSpace(rest[:idx])
			}
		}
	}

	// Try to find JSON object or array
	if start := strings.IndexAny(text, "{["); start >= 0 {
		text = text[start:]
	}

	return text
}
