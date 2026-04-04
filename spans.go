package agnogo

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ── Structured Agent Tracing ─────────────────────────────────────────
//
// Every Run() is a black box. Structured tracing opens it up.
//
// Instead of just getting Response{Text: "..."}, you get a full trace
// of what happened inside: every model call, tool call, guardrail check,
// reasoning step — with timing, tokens, cost, and status.
//
// Usage:
//
//	sc := agnogo.NewSpanCollector()
//	agent := agnogo.Agent("...", agnogo.WithTrace(sc.Trace()))
//
//	resp, _ := agent.Run(ctx, session, "Book Thursday 2pm")
//	trace := sc.Collect(resp)
//	trace.Print()   // human-readable tree
//	trace.JSON()    // machine-readable
//
// Output:
//
//	[run r_abc] 2.1s | $0.003 | 600 tok | 2 model | 2 tool
//	  ├─ [guard]  pii-input           ✓       0ms
//	  ├─ [model]  call                1.2s   420 tok  $0.002
//	  ├─ [tool]   check_availability  0.4s   → "3 slots"
//	  ├─ [tool]   book_slot           0.2s   → "confirmed"
//	  ├─ [model]  call                0.3s   180 tok  $0.001
//	  └─ [guard]  hallucination       ✓       0ms

// ── Span Types ───────────────────────────────────────────────────────

// SpanKind identifies what type of event a span represents.
type SpanKind int

const (
	SpanModel     SpanKind = iota // LLM model call
	SpanTool                      // tool execution
	SpanGuardrail                 // input/output guardrail check
	SpanKnowledge                 // knowledge/RAG search
	SpanReasoning                 // reasoning step (chain-of-thought)
	SpanApproval                  // human approval request
	SpanSession                   // session save/load
)

// SpanStatus indicates the outcome of a span.
type SpanStatus int

const (
	SpanOK      SpanStatus = iota // completed successfully
	SpanError                      // failed with error
	SpanBlocked                    // blocked by guardrail or approval
)

// Span is one event in an agent run.
type Span struct {
	Name      string        `json:"name"`
	Kind      SpanKind      `json:"kind"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration_ms"`
	Status    SpanStatus    `json:"status"`

	// Model-specific
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`

	// Tool-specific
	ToolArgs   map[string]string `json:"tool_args,omitempty"`
	ToolResult string            `json:"tool_result,omitempty"`

	// Guardrail-specific
	Direction string `json:"direction,omitempty"` // "input" or "output"
	Blocked   bool   `json:"blocked,omitempty"`

	// Reasoning-specific
	Confidence float64 `json:"confidence,omitempty"`

	// Error
	Error error `json:"-"`

	// Children (for reasoning steps)
	Children []*Span `json:"children,omitempty"`

	// Free-form metadata
	Meta map[string]any `json:"meta,omitempty"`
}

func (s SpanKind) String() string {
	switch s {
	case SpanModel:
		return "model"
	case SpanTool:
		return "tool"
	case SpanGuardrail:
		return "guard"
	case SpanKnowledge:
		return "knowledge"
	case SpanReasoning:
		return "reasoning"
	case SpanApproval:
		return "approval"
	case SpanSession:
		return "session"
	default:
		return "unknown"
	}
}

func (s SpanStatus) String() string {
	switch s {
	case SpanOK:
		return "ok"
	case SpanError:
		return "error"
	case SpanBlocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// ── RunTrace ─────────────────────────────────────────────────────────

// RunTrace captures the complete structured trace of an agent Run().
type RunTrace struct {
	RunID     string        `json:"run_id"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration_ms"`
	Spans     []*Span       `json:"spans"`

	// Aggregates (computed from spans)
	TotalTokens  int     `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	ModelCalls   int     `json:"model_calls"`
	ToolCalls    int     `json:"tool_calls"`
}

// Print outputs a human-readable trace tree to stdout.
func (rt *RunTrace) Print() {
	dur := rt.Duration.Round(time.Millisecond)
	fmt.Printf("[run %s] %s | $%.4f | %d tok | %d model | %d tool\n",
		rt.RunID, dur, rt.TotalCost, rt.TotalTokens, rt.ModelCalls, rt.ToolCalls)

	for i, span := range rt.Spans {
		last := i == len(rt.Spans)-1
		printSpan(span, last, "")
	}
	fmt.Println()
}

func printSpan(s *Span, last bool, indent string) {
	connector := "├─"
	if last {
		connector = "└─"
	}

	kind := fmt.Sprintf("[%s]", s.Kind)
	var detail string

	switch s.Kind {
	case SpanModel:
		detail = fmt.Sprintf("%-8s %s  %d tok  $%.4f",
			s.Name, fmtDur(s.Duration), s.InputTokens+s.OutputTokens, s.Cost)
	case SpanTool:
		result := s.ToolResult
		if len(result) > 60 {
			result = result[:60] + "..."
		}
		detail = fmt.Sprintf("%-20s %s  -> %q", s.Name, fmtDur(s.Duration), result)
	case SpanGuardrail:
		status := "✓"
		if s.Blocked {
			status = "✗ BLOCKED"
		}
		detail = fmt.Sprintf("%-20s %s  %s", s.Name, s.Direction, status)
	case SpanReasoning:
		detail = fmt.Sprintf("%-20s confidence: %.0f%%", s.Name, s.Confidence*100)
	case SpanKnowledge:
		detail = fmt.Sprintf("%-20s %s", s.Name, fmtDur(s.Duration))
	case SpanApproval:
		reason := ""
		if s.Meta != nil {
			if r, ok := s.Meta["reason"].(string); ok {
				reason = r
			}
		}
		detail = fmt.Sprintf("%-20s %s", s.Name, reason)
	case SpanSession:
		status := "saved"
		if s.Status == SpanError {
			status = "FAILED"
		}
		detail = fmt.Sprintf("%-20s %s", s.Name, status)
	default:
		detail = s.Name
	}

	fmt.Printf("  %s%s %s  %s\n", indent, connector, kind, detail)

	// Print children
	childIndent := indent + "  "
	if !last {
		childIndent = indent + "│ "
	}
	for j, child := range s.Children {
		childLast := j == len(s.Children)-1
		printSpan(child, childLast, childIndent)
	}
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return "<1ms"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// JSON returns the trace as formatted JSON.
func (rt *RunTrace) JSON() string {
	data, err := json.MarshalIndent(rt, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

// ── SpanCollector ────────────────────────────────────────────────────

// SpanCollector captures structured spans from Trace hooks.
// Thread-safe — handles concurrent tool calls from goroutines.
type SpanCollector struct {
	mu        sync.Mutex
	spans     []*Span
	startTime time.Time
	prices    *CostTracker

	// Current reasoning parent (for nesting reasoning steps)
	reasoningSpan *Span
}

// NewSpanCollector creates a span collector with default cost pricing.
func NewSpanCollector() *SpanCollector {
	return &SpanCollector{
		startTime: time.Now(),
		prices:    NewCostTracker(),
	}
}

// Trace returns a *Trace that captures structured spans from all hook points.
// Pass this to WithTrace() when creating an agent.
func (sc *SpanCollector) Trace() *Trace {
	return &Trace{
		OnModelCall: func(_ []Message, resp *ModelResponse, dur time.Duration) {
			span := &Span{
				Name:      "call",
				Kind:      SpanModel,
				StartTime: time.Now().Add(-dur),
				Duration:  dur,
				Status:    SpanOK,
			}
			if resp != nil && resp.Usage != nil {
				span.InputTokens = resp.Usage.InputTokens
				span.OutputTokens = resp.Usage.OutputTokens
				span.Cost = sc.prices.Estimate("gpt-4.1-mini", resp.Usage)
			}
			sc.add(span)
		},

		OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) {
			status := SpanOK
			if err != nil {
				status = SpanError
			}
			sc.add(&Span{
				Name:       name,
				Kind:       SpanTool,
				StartTime:  time.Now().Add(-dur),
				Duration:   dur,
				Status:     status,
				ToolArgs:   args,
				ToolResult: truncateStr(result, 200),
				Error:      err,
			})
		},

		OnGuardrail: func(name, direction string, blocked bool) {
			status := SpanOK
			if blocked {
				status = SpanBlocked
			}
			sc.add(&Span{
				Name:      name,
				Kind:      SpanGuardrail,
				StartTime: time.Now(),
				Status:    status,
				Direction: direction,
				Blocked:   blocked,
			})
		},

		OnKnowledge: func(query, _ string, dur time.Duration) {
			sc.add(&Span{
				Name:      "search",
				Kind:      SpanKnowledge,
				StartTime: time.Now().Add(-dur),
				Duration:  dur,
				Status:    SpanOK,
				Meta:      map[string]any{"query": query},
			})
		},

		OnApproval: func(a HumanApproval) {
			sc.add(&Span{
				Name:      a.ToolName,
				Kind:      SpanApproval,
				StartTime: time.Now(),
				Status:    SpanBlocked,
				Meta:      map[string]any{"reason": a.Reason, "session": a.SessionID},
			})
		},

		OnSessionSave: func(_ *Session, err error) {
			status := SpanOK
			if err != nil {
				status = SpanError
			}
			sc.add(&Span{
				Name:      "save",
				Kind:      SpanSession,
				StartTime: time.Now(),
				Status:    status,
				Error:     err,
			})
		},

		OnReasoning: func(step ReasoningStep, index int) {
			span := &Span{
				Name:       step.Title,
				Kind:       SpanReasoning,
				StartTime:  time.Now(),
				Status:     SpanOK,
				Confidence: step.Confidence,
				Meta:       map[string]any{"action": step.Action, "result": step.Result},
			}
			sc.addReasoning(span)
		},
	}
}

// add appends a span (thread-safe for concurrent tool calls).
func (sc *SpanCollector) add(span *Span) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.spans = append(sc.spans, span)
}

// addReasoning groups reasoning steps under a parent span.
func (sc *SpanCollector) addReasoning(span *Span) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.reasoningSpan == nil {
		sc.reasoningSpan = &Span{
			Name:      "reasoning",
			Kind:      SpanReasoning,
			StartTime: span.StartTime,
			Status:    SpanOK,
		}
		sc.spans = append(sc.spans, sc.reasoningSpan)
	}
	sc.reasoningSpan.Children = append(sc.reasoningSpan.Children, span)
	sc.reasoningSpan.Duration = time.Since(sc.reasoningSpan.StartTime)
}

// Collect finalizes the trace after a Run() completes.
// Call this after agent.Run() returns.
func (sc *SpanCollector) Collect(resp *Response) *RunTrace {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	rt := &RunTrace{
		StartTime: sc.startTime,
		Duration:  time.Since(sc.startTime),
		Spans:     sc.spans,
	}

	if resp != nil && resp.Metrics != nil {
		rt.RunID = resp.Metrics.RunID
	}

	// Compute aggregates from spans
	for _, span := range sc.spans {
		sc.aggregate(rt, span)
	}

	return rt
}

func (sc *SpanCollector) aggregate(rt *RunTrace, span *Span) {
	switch span.Kind {
	case SpanModel:
		rt.ModelCalls++
		rt.TotalTokens += span.InputTokens + span.OutputTokens
		rt.TotalCost += span.Cost
	case SpanTool:
		rt.ToolCalls++
	}
	for _, child := range span.Children {
		sc.aggregate(rt, child)
	}
}

// Reset clears the collector for reuse across multiple runs.
func (sc *SpanCollector) Reset() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.spans = nil
	sc.reasoningSpan = nil
	sc.startTime = time.Now()
}

// ── Convenience Option ───────────────────────────────────────────────

// WithSpanCollector creates a SpanCollector and sets it as the agent's trace.
// Returns the collector for later use with Collect().
//
//	sc := agnogo.NewSpanCollector()
//	agent := agnogo.Agent("...", agnogo.WithSpanCollector(sc))
func WithSpanCollector(sc *SpanCollector) Option {
	return WithTrace(sc.Trace())
}

// spanKindNames for JSON marshaling
var spanKindNames = [...]string{"model", "tool", "guardrail", "knowledge", "reasoning", "approval", "session"}

func (k SpanKind) MarshalJSON() ([]byte, error) {
	if int(k) < len(spanKindNames) {
		return json.Marshal(spanKindNames[k])
	}
	return json.Marshal("unknown")
}

var spanStatusNames = [...]string{"ok", "error", "blocked"}

func (s SpanStatus) MarshalJSON() ([]byte, error) {
	if int(s) < len(spanStatusNames) {
		return json.Marshal(spanStatusNames[s])
	}
	return json.Marshal("unknown")
}

// DurationMS is used for JSON marshaling of Duration fields.
func (s *Span) MarshalJSON() ([]byte, error) {
	type spanAlias Span
	return json.Marshal(&struct {
		*spanAlias
		DurationMS int64  `json:"duration_ms"`
		ErrorStr   string `json:"error,omitempty"`
	}{
		spanAlias:  (*spanAlias)(s),
		DurationMS: s.Duration.Milliseconds(),
		ErrorStr: func() string {
			if s.Error != nil {
				return s.Error.Error()
			}
			return ""
		}(),
	})
}

