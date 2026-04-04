package agnogo

import (
	"context"
	"fmt"
	"time"
)

// ── Trace Replay ─────────────────────────────────────────────────────
//
// Layer 3 of Trace Intelligence. Re-run a stored trace with a different
// agent to compare outputs, costs, and behavior.
//
// Usage:
//
//	original, _ := store.LoadTrace(ctx, "run_abc123")
//	betterAgent := agnogo.Agent("Improved prompt...", agnogo.Reliable())
//	result, _ := agnogo.Replay(ctx, original, betterAgent)
//
//	fmt.Printf("Cost: $%.4f → $%.4f (%+.4f)\n",
//	    result.Original.TotalCost, result.Replayed.TotalCost, result.Diff.CostDelta)
//	fmt.Printf("Response changed: %v\n", result.Diff.ResponseChanged)

// ReplayResult holds the original trace, the replayed trace, and the diff.
type ReplayResult struct {
	Original *RunTrace  `json:"original"`
	Replayed *RunTrace  `json:"replayed"`
	Diff     *TraceDiff `json:"diff"`
}

// TraceDiff shows what changed between two traces.
type TraceDiff struct {
	CostDelta       float64       `json:"cost_delta"`        // replayed - original
	TokenDelta      int           `json:"token_delta"`       // replayed - original
	DurationDelta   time.Duration `json:"duration_delta"`    // replayed - original
	ModelCallDelta  int           `json:"model_call_delta"`  // replayed - original
	ToolCallDelta   int           `json:"tool_call_delta"`   // replayed - original
	ResponseChanged bool          `json:"response_changed"`  // did output text change?
	OriginalResponse string       `json:"original_response,omitempty"` // for comparison
	ReplayedResponse string       `json:"replayed_response,omitempty"` // for comparison
}

// Replay re-runs a stored trace's input with a different agent.
// The original trace must have UserMessage set (captured by SpanCollector).
// Returns the comparison between the original and replayed execution.
func Replay(ctx context.Context, original *RunTrace, agent *Core) (*ReplayResult, error) {
	if original == nil {
		return nil, fmt.Errorf("agnogo: original trace is nil")
	}
	if original.UserMessage == "" {
		return nil, fmt.Errorf("agnogo: original trace has no UserMessage (needed for replay)")
	}
	if agent == nil {
		return nil, fmt.Errorf("agnogo: replay agent is nil")
	}

	// Create fresh session and span collector
	sessionID := "replay-" + original.RunID
	session := NewSession(sessionID)
	sc := NewSpanCollector()

	// Temporarily set trace on agent, restore original after
	origTrace := agent.trace
	agent.trace = sc.Trace()
	defer func() { agent.trace = origTrace }()

	// Run the same input through the new agent
	resp, err := agent.Run(ctx, session, original.UserMessage)
	if err != nil {
		return nil, fmt.Errorf("agnogo: replay failed: %w", err)
	}

	// Collect the replay trace
	replayed := sc.Collect(resp)
	replayed.UserMessage = original.UserMessage
	replayed.SessionID = sessionID

	// Compute diff
	diff := &TraceDiff{
		CostDelta:      replayed.TotalCost - original.TotalCost,
		TokenDelta:     replayed.TotalTokens - original.TotalTokens,
		DurationDelta:  replayed.Duration - original.Duration,
		ModelCallDelta: replayed.ModelCalls - original.ModelCalls,
		ToolCallDelta:  replayed.ToolCalls - original.ToolCalls,
	}

	// Compare responses
	diff.OriginalResponse = original.ResponseText
	if resp != nil {
		diff.ReplayedResponse = resp.Text
	}
	diff.ResponseChanged = original.ResponseText != diff.ReplayedResponse

	return &ReplayResult{
		Original: original,
		Replayed: replayed,
		Diff:     diff,
	}, nil
}

// Print outputs a human-readable replay comparison.
func (r *ReplayResult) Print() {
	fmt.Println("═══ REPLAY COMPARISON ═══")
	fmt.Println()
	fmt.Printf("  Input:    %q\n", truncateStr(r.Original.UserMessage, 80))
	fmt.Printf("  Original: %s | $%.4f | %d tok | %d model | %d tool\n",
		r.Original.Duration.Round(time.Millisecond),
		r.Original.TotalCost, r.Original.TotalTokens,
		r.Original.ModelCalls, r.Original.ToolCalls)
	fmt.Printf("  Replayed: %s | $%.4f | %d tok | %d model | %d tool\n",
		r.Replayed.Duration.Round(time.Millisecond),
		r.Replayed.TotalCost, r.Replayed.TotalTokens,
		r.Replayed.ModelCalls, r.Replayed.ToolCalls)
	fmt.Println()
	fmt.Printf("  Cost:     %+.4f\n", r.Diff.CostDelta)
	fmt.Printf("  Tokens:   %+d\n", r.Diff.TokenDelta)
	fmt.Printf("  Duration: %+v\n", r.Diff.DurationDelta.Round(time.Millisecond))
	fmt.Printf("  Models:   %+d\n", r.Diff.ModelCallDelta)
	fmt.Printf("  Tools:    %+d\n", r.Diff.ToolCallDelta)
	fmt.Println()
}
