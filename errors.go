package agnogo

import "errors"

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrToolNotFound    = errors.New("tool not found")
	ErrMaxLoops        = errors.New("max tool call loops reached")
	ErrApprovalNeeded  = errors.New("human approval required")
	ErrModelFailed     = errors.New("model call failed")
	ErrGuardrailBlock  = errors.New("blocked by guardrail")
)
