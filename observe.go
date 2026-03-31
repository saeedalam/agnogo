package agnogo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// --------------------------------------------------------------------
// CostTracker
// --------------------------------------------------------------------

// TokenPrice holds per-million-token pricing for a model.
type TokenPrice struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// CostTracker estimates USD cost from token usage.
type CostTracker struct {
	mu     sync.RWMutex
	prices map[string]TokenPrice
}

// NewCostTracker returns a CostTracker pre-populated with known model prices.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		prices: map[string]TokenPrice{
			"gpt-4o":            {InputPerMillion: 2.50, OutputPerMillion: 10.00},
			"gpt-4.1-mini":     {InputPerMillion: 0.40, OutputPerMillion: 1.60},
			"claude-sonnet-4-5": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
			"gemini-2.0-flash": {InputPerMillion: 0.10, OutputPerMillion: 0.40},
		},
	}
}

// SetPrice sets or overrides the token price for a model.
func (ct *CostTracker) SetPrice(model string, input, output float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.prices[model] = TokenPrice{InputPerMillion: input, OutputPerMillion: output}
}

// Estimate returns the estimated USD cost for a given model and usage.
// Returns 0 if the model is unknown or usage is nil.
func (ct *CostTracker) Estimate(model string, usage *Usage) float64 {
	if usage == nil {
		return 0
	}
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	p, ok := ct.prices[model]
	if !ok {
		return 0
	}
	inCost := float64(usage.InputTokens) / 1_000_000 * p.InputPerMillion
	outCost := float64(usage.OutputTokens) / 1_000_000 * p.OutputPerMillion
	return inCost + outCost
}

// --------------------------------------------------------------------
// MetricsCollector
// --------------------------------------------------------------------

const latencyBufferSize = 1024

// MetricsCollector aggregates agent telemetry across runs.
type MetricsCollector struct {
	mu             sync.Mutex
	totalRuns      int64
	totalTokensIn  int64
	totalTokensOut int64
	totalToolCalls int64
	totalErrors    int64
	totalCost      float64
	latencies      []time.Duration // circular buffer
	latencyIdx     int
	providerCalls  map[string]int64
	toolCallCounts map[string]int64
	costTracker    *CostTracker
}

// NewMetricsCollector creates a MetricsCollector with a default CostTracker.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		latencies:      make([]time.Duration, 0, latencyBufferSize),
		providerCalls:  make(map[string]int64),
		toolCallCounts: make(map[string]int64),
		costTracker:    NewCostTracker(),
	}
}

// Trace returns a *Trace with all hooks wired to this collector.
func (mc *MetricsCollector) Trace() *Trace {
	return &Trace{
		OnModelCall: func(_ []Message, resp *ModelResponse, dur time.Duration) {
			mc.mu.Lock()
			defer mc.mu.Unlock()
			mc.totalRuns++
			// record latency in circular buffer
			if len(mc.latencies) < latencyBufferSize {
				mc.latencies = append(mc.latencies, dur)
			} else {
				mc.latencies[mc.latencyIdx%latencyBufferSize] = dur
			}
			mc.latencyIdx++
			// record tokens
			if resp != nil && resp.Usage != nil {
				mc.totalTokensIn += int64(resp.Usage.InputTokens)
				mc.totalTokensOut += int64(resp.Usage.OutputTokens)
			}
		},
		OnToolCall: func(name string, _ map[string]string, _ string, _ time.Duration, _ error) {
			mc.mu.Lock()
			defer mc.mu.Unlock()
			mc.totalToolCalls++
			mc.toolCallCounts[name]++
		},
		OnGuardrail: func(_ string, _ string, blocked bool) {
			if blocked {
				mc.mu.Lock()
				mc.totalErrors++
				mc.mu.Unlock()
			}
		},
	}
}

// MetricsSnapshot is a point-in-time view of collected metrics.
type MetricsSnapshot struct {
	TotalRuns      int64            `json:"total_runs"`
	TotalTokensIn  int64            `json:"total_tokens_in"`
	TotalTokensOut int64            `json:"total_tokens_out"`
	TotalToolCalls int64            `json:"total_tool_calls"`
	TotalErrors    int64            `json:"total_errors"`
	EstimatedCost  float64          `json:"estimated_cost_usd"`
	AvgLatency     time.Duration    `json:"avg_latency_ms"`
	P99Latency     time.Duration    `json:"p99_latency_ms"`
	ToolCallCounts map[string]int64 `json:"tool_call_counts"`
}

// Snapshot returns a point-in-time copy of the collected metrics.
func (mc *MetricsCollector) Snapshot() MetricsSnapshot {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	snap := MetricsSnapshot{
		TotalRuns:      mc.totalRuns,
		TotalTokensIn:  mc.totalTokensIn,
		TotalTokensOut: mc.totalTokensOut,
		TotalToolCalls: mc.totalToolCalls,
		TotalErrors:    mc.totalErrors,
		EstimatedCost:  mc.totalCost,
		ToolCallCounts: make(map[string]int64, len(mc.toolCallCounts)),
	}
	for k, v := range mc.toolCallCounts {
		snap.ToolCallCounts[k] = v
	}

	n := len(mc.latencies)
	if n > 0 {
		// average
		var sum time.Duration
		for _, d := range mc.latencies {
			sum += d
		}
		snap.AvgLatency = sum / time.Duration(n)

		// p99
		sorted := make([]time.Duration, n)
		copy(sorted, mc.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		idx := (n*99 + 99) / 100 // ceiling
		if idx >= n {
			idx = n - 1
		}
		snap.P99Latency = sorted[idx]
	}

	return snap
}

// Handler returns an http.Handler that serves the metrics snapshot as JSON.
func (mc *MetricsCollector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		snap := mc.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
	})
}

// --------------------------------------------------------------------
// Explain
// --------------------------------------------------------------------

// Explain prints a human-readable summary of an agent's configuration to stdout.
func Explain(agent *Core) {
	fmt.Println("=== Agent Summary ===")

	// Instructions (truncated)
	instr := agent.instructions
	if len(instr) > 100 {
		instr = instr[:100] + "..."
	}
	fmt.Printf("Instructions : %s\n", instr)

	// Model type
	if agent.model != nil {
		fmt.Printf("Model        : %T\n", agent.model)
	} else {
		fmt.Println("Model        : <nil>")
	}

	// Tools
	if agent.tools != nil {
		fmt.Printf("Tools        : %d [%s]\n", agent.tools.Count(), agent.tools.Names())
	} else {
		fmt.Println("Tools        : 0")
	}

	// Storage
	if agent.storage != nil {
		fmt.Printf("Storage      : %T\n", agent.storage)
	} else {
		fmt.Println("Storage      : <nil>")
	}

	// Guardrails
	fmt.Printf("Input guards : %d\n", len(agent.inputGuards))
	fmt.Printf("Output guards: %d\n", len(agent.outputGuards))

	// Retry
	if agent.retry != nil {
		fmt.Printf("Retry        : max=%d, initial=%s, backoff=%v, maxDelay=%s\n",
			agent.retry.MaxRetries, agent.retry.InitialDelay, agent.retry.ExponentialBackoff, agent.retry.MaxDelay)
	} else {
		fmt.Println("Retry        : <nil>")
	}

	// History
	if agent.history != nil {
		fmt.Printf("History      : maxMsgs=%d, maxTool=%d, summaryThreshold=%d\n",
			agent.history.MaxMessages, agent.history.MaxToolMessages, agent.history.SummaryThreshold)
	} else {
		fmt.Println("History      : <nil>")
	}

	// Max loops
	fmt.Printf("Max loops    : %d\n", agent.maxLoops)
}

// --------------------------------------------------------------------
// Validate
// --------------------------------------------------------------------

// ValidationError describes a configuration problem with an agent.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Level   string `json:"level"` // "error" or "warning"
}

// Validate checks an agent's configuration and returns any problems found.
func Validate(agent *Core) []ValidationError {
	var errs []ValidationError

	if agent.model == nil {
		errs = append(errs, ValidationError{
			Field:   "model",
			Message: "model provider is nil; agent cannot make LLM calls",
			Level:   "error",
		})
	}

	if agent.tools == nil || agent.tools.Count() == 0 {
		errs = append(errs, ValidationError{
			Field:   "tools",
			Message: "no tools registered; agent can only chat",
			Level:   "warning",
		})
	}

	if agent.retry == nil {
		errs = append(errs, ValidationError{
			Field:   "retry",
			Message: "no retry config; transient failures will not be retried",
			Level:   "warning",
		})
	}

	if agent.history == nil {
		errs = append(errs, ValidationError{
			Field:   "history",
			Message: "no history config; long conversations may overflow context window",
			Level:   "warning",
		})
	}

	if agent.maxLoops > 20 {
		errs = append(errs, ValidationError{
			Field:   "maxLoops",
			Message: fmt.Sprintf("maxLoops is %d; values above 20 risk runaway tool loops", agent.maxLoops),
			Level:   "warning",
		})
	}

	if agent.instructions == "" {
		errs = append(errs, ValidationError{
			Field:   "instructions",
			Message: "instructions are empty; agent has no system prompt guidance",
			Level:   "warning",
		})
	}

	return errs
}
