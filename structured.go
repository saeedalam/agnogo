package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
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
func RunStructured[T any](ctx context.Context, agent *Agent, session *Session, message string, out *T) error {
	// Add JSON instruction to the system prompt
	schema, _ := json.Marshal(out)
	instruction := fmt.Sprintf("\n\nRESPONSE FORMAT: You MUST respond with valid JSON matching this structure: %s\nReturn ONLY the JSON, no other text.", string(schema))

	// Temporarily modify instructions
	origInstr := agent.instructions
	origPromptFunc := agent.promptFunc
	if agent.promptFunc != nil {
		orig := agent.promptFunc
		agent.promptFunc = func(s *Session) string {
			return orig(s) + instruction
		}
	} else {
		agent.instructions += instruction
	}

	resp, err := agent.Run(ctx, session, message)

	// Restore
	agent.instructions = origInstr
	agent.promptFunc = origPromptFunc

	if err != nil {
		return err
	}

	// Parse JSON from response
	text := resp.Text
	// Strip markdown code blocks if present
	if len(text) > 6 && text[:3] == "```" {
		if idx := findIndex(text[3:], "```"); idx > 0 {
			inner := text[3 : 3+idx]
			if len(inner) > 0 && inner[0] == '{' {
				text = inner
			} else if nl := findIndex(inner, "\n"); nl > 0 {
				text = inner[nl+1:]
			}
		}
	}

	if err := json.Unmarshal([]byte(text), out); err != nil {
		return fmt.Errorf("failed to parse structured response: %w (raw: %s)", err, truncateStr(text, 200))
	}
	return nil
}

func findIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
