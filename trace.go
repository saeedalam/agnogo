package agnogo

import (
	"log/slog"
	"time"
)

// Trace hooks for observing every agent decision.
// Implement any or all hooks for debugging, monitoring, or auditing.
type Trace struct {
	OnModelCall     func(messages []Message, resp *ModelResponse, dur time.Duration)
	OnToolCall      func(name string, args map[string]string, result string, dur time.Duration, err error)
	OnKnowledge     func(query string, result string, dur time.Duration)
	OnMemory        func(key, value string)
	OnGuardrail     func(name string, direction string, blocked bool) // direction: "input" or "output"
	OnRouting       func(agentName string, userMessage string)
	OnApproval      func(approval HumanApproval)
	OnSessionSave   func(session *Session, err error)
	OnReasoning     func(step ReasoningStep, index int)
}

// DefaultTrace returns a trace that logs everything via slog.
func DefaultTrace() *Trace {
	return &Trace{
		OnModelCall: func(msgs []Message, resp *ModelResponse, dur time.Duration) {
			tools := len(resp.ToolCalls)
			slog.Debug("agnogo: model call", "messages", len(msgs), "tools", tools, "latency_ms", dur.Milliseconds())
		},
		OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) {
			slog.Info("agnogo: tool call", "tool", name, "latency_ms", dur.Milliseconds(), "success", err == nil, "result_len", len(result))
		},
		OnKnowledge: func(query, result string, dur time.Duration) {
			slog.Debug("agnogo: knowledge search", "query_len", len(query), "result_len", len(result), "latency_ms", dur.Milliseconds())
		},
		OnMemory: func(key, value string) {
			slog.Debug("agnogo: memory", "key", key, "value_len", len(value))
		},
		OnGuardrail: func(name, direction string, blocked bool) {
			if blocked {
				slog.Warn("agnogo: guardrail blocked", "name", name, "direction", direction)
			}
		},
		OnRouting: func(agentName, msg string) {
			slog.Info("agnogo: routed", "agent", agentName)
		},
		OnApproval: func(a HumanApproval) {
			slog.Info("agnogo: approval needed", "tool", a.ToolName, "reason", a.Reason)
		},
		OnSessionSave: func(s *Session, err error) {
			if err != nil {
				slog.Error("agnogo: session save failed", "session", s.ID, "error", err)
			}
		},
	}
}
