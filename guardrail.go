package agnogo

import "context"

// Guardrail checks input or output. Return error to block, nil to allow.
type Guardrail struct {
	Name  string
	Check func(ctx context.Context, session *Session, msg string) error
}

func runGuardrails(ctx context.Context, guards []Guardrail, session *Session, msg string) error {
	for _, g := range guards {
		if err := g.Check(ctx, session, msg); err != nil {
			return err
		}
	}
	return nil
}
