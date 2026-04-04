package agnogo

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ── Trace Intelligence ───────────────────────────────────────────────
//
// Layer 2 of Trace Intelligence. Analyze stored traces for insights.
//
// Usage:
//
//	analyzer := agnogo.NewTraceAnalyzer(store)
//	summary, _ := analyzer.CostSummary(ctx, time.Now().Add(-24*time.Hour))
//	anomalies, _ := analyzer.DetectAnomalies(ctx, time.Now().Add(-24*time.Hour))
//	for _, a := range anomalies {
//	    alert(a.Message)
//	}

// TraceAnalyzer computes insights from stored traces.
type TraceAnalyzer struct {
	store TraceStore
}

// NewTraceAnalyzer creates an analyzer backed by a trace store.
func NewTraceAnalyzer(store TraceStore) *TraceAnalyzer {
	return &TraceAnalyzer{store: store}
}

// ── Cost Summary ─────────────────────────────────────────────────────

// CostSummary holds cost statistics for a time window.
type CostSummary struct {
	TotalCost   float64       `json:"total_cost"`
	AvgCost     float64       `json:"avg_cost"`
	MaxCost     float64       `json:"max_cost"`
	RunCount    int           `json:"run_count"`
	TokenCount  int           `json:"token_count"`
	TotalTime   time.Duration `json:"total_time"`
	CostPerHour float64       `json:"cost_per_hour"`
}

// CostSummary returns cost statistics for traces since the given time.
func (ta *TraceAnalyzer) CostSummary(ctx context.Context, since time.Time) (*CostSummary, error) {
	traces, err := ta.store.QueryTraces(ctx, TraceQuery{Since: since})
	if err != nil {
		return nil, err
	}
	if len(traces) == 0 {
		return &CostSummary{}, nil
	}

	s := &CostSummary{RunCount: len(traces)}
	for _, t := range traces {
		s.TotalCost += t.TotalCost
		s.TokenCount += t.TotalTokens
		s.TotalTime += t.Duration
		if t.TotalCost > s.MaxCost {
			s.MaxCost = t.TotalCost
		}
	}
	s.AvgCost = s.TotalCost / float64(s.RunCount)

	elapsed := time.Since(since)
	if elapsed > 0 {
		s.CostPerHour = s.TotalCost / elapsed.Hours()
	}

	return s, nil
}

// ── Anomaly Detection ────────────────────────────────────────────────

// Anomaly describes a trace that deviates significantly from normal.
type Anomaly struct {
	RunID    string  `json:"run_id"`
	Type     string  `json:"type"`     // "high_cost", "slow", "error", "chatty"
	Message  string  `json:"message"`  // human-readable
	Value    float64 `json:"value"`    // the anomalous value
	Expected float64 `json:"expected"` // rolling average
}

// DetectAnomalies finds runs that deviate from normal patterns.
// Uses mean + 2*stddev as the threshold (catches ~5% outliers).
func (ta *TraceAnalyzer) DetectAnomalies(ctx context.Context, since time.Time) ([]Anomaly, error) {
	traces, err := ta.store.QueryTraces(ctx, TraceQuery{Since: since})
	if err != nil {
		return nil, err
	}
	if len(traces) < 3 {
		return nil, nil // need at least 3 traces for meaningful stats
	}

	// Compute means
	var costs, durations, modelCounts []float64
	for _, t := range traces {
		costs = append(costs, t.TotalCost)
		durations = append(durations, t.Duration.Seconds())
		modelCounts = append(modelCounts, float64(t.ModelCalls))
	}

	meanCost, stdCost := meanStdDev(costs)
	meanDur, stdDur := meanStdDev(durations)
	meanModels, stdModels := meanStdDev(modelCounts)

	var anomalies []Anomaly
	for _, t := range traces {
		// High cost
		if t.TotalCost > meanCost+2*stdCost && stdCost > 0 {
			anomalies = append(anomalies, Anomaly{
				RunID:    t.RunID,
				Type:     "high_cost",
				Message:  fmt.Sprintf("cost $%.4f is %.1fx the average $%.4f", t.TotalCost, t.TotalCost/meanCost, meanCost),
				Value:    t.TotalCost,
				Expected: meanCost,
			})
		}

		// Slow
		durSec := t.Duration.Seconds()
		if durSec > meanDur+2*stdDur && stdDur > 0 {
			anomalies = append(anomalies, Anomaly{
				RunID:    t.RunID,
				Type:     "slow",
				Message:  fmt.Sprintf("%.1fs is %.1fx the average %.1fs", durSec, durSec/meanDur, meanDur),
				Value:    durSec,
				Expected: meanDur,
			})
		}

		// Chatty (too many model calls)
		mc := float64(t.ModelCalls)
		if mc > meanModels+2*stdModels && stdModels > 0 {
			anomalies = append(anomalies, Anomaly{
				RunID:    t.RunID,
				Type:     "chatty",
				Message:  fmt.Sprintf("%d model calls vs average %.0f", t.ModelCalls, meanModels),
				Value:    mc,
				Expected: meanModels,
			})
		}

		// Errors
		if t.HasErrors {
			anomalies = append(anomalies, Anomaly{
				RunID:   t.RunID,
				Type:    "error",
				Message: "run had errors",
				Value:   1,
			})
		}
	}

	return anomalies, nil
}

// ── Tool Statistics ──────────────────────────────────────────────────

// ToolStat holds usage statistics for a single tool.
type ToolStat struct {
	Name        string        `json:"name"`
	CallCount   int           `json:"call_count"`
	TotalTime   time.Duration `json:"total_time"`
	AvgDuration time.Duration `json:"avg_duration"`
	ErrorCount  int           `json:"error_count"`
	ErrorRate   float64       `json:"error_rate"` // 0.0-1.0
}

// ToolStats returns per-tool usage statistics.
func (ta *TraceAnalyzer) ToolStats(ctx context.Context, since time.Time) (map[string]*ToolStat, error) {
	traces, err := ta.store.QueryTraces(ctx, TraceQuery{Since: since})
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*ToolStat)
	for _, t := range traces {
		for _, span := range t.Spans {
			collectToolStats(span, stats)
		}
	}

	// Compute averages
	for _, s := range stats {
		if s.CallCount > 0 {
			s.AvgDuration = s.TotalTime / time.Duration(s.CallCount)
			s.ErrorRate = float64(s.ErrorCount) / float64(s.CallCount)
		}
	}

	return stats, nil
}

func collectToolStats(span *Span, stats map[string]*ToolStat) {
	if span.Kind == SpanTool {
		s, ok := stats[span.Name]
		if !ok {
			s = &ToolStat{Name: span.Name}
			stats[span.Name] = s
		}
		s.CallCount++
		s.TotalTime += span.Duration
		if span.Status == SpanError {
			s.ErrorCount++
		}
	}
	for _, child := range span.Children {
		collectToolStats(child, stats)
	}
}

// ── Error Report ─────────────────────────────────────────────────────

// ErrorReport returns all traces with errors in the time window.
func (ta *TraceAnalyzer) ErrorReport(ctx context.Context, since time.Time) ([]*RunTrace, error) {
	hasErrors := true
	return ta.store.QueryTraces(ctx, TraceQuery{Since: since, HasErrors: &hasErrors})
}

// ── Math Helpers ─────────────────────────────────────────────────────

func meanStdDev(vals []float64) (mean, stddev float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	n := float64(len(vals))
	for _, v := range vals {
		mean += v
	}
	mean /= n

	var variance float64
	for _, v := range vals {
		diff := v - mean
		variance += diff * diff
	}
	variance /= n
	stddev = math.Sqrt(variance)
	return mean, stddev
}
