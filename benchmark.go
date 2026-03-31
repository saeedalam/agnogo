package agnogo

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// BenchmarkConfig configures a benchmark run.
type BenchmarkConfig struct {
	Prompts     []string // messages to send to the agent
	Concurrency int      // number of parallel workers (default 1)
	Warmup      int      // warmup rounds before measuring (default 0)
}

// BenchmarkResult holds the results of a benchmark run.
type BenchmarkResult struct {
	TotalRequests  int           `json:"total_requests"`
	TotalDuration  time.Duration `json:"total_duration"`
	AvgLatency     time.Duration `json:"avg_latency"`
	P50Latency     time.Duration `json:"p50_latency"`
	P95Latency     time.Duration `json:"p95_latency"`
	P99Latency     time.Duration `json:"p99_latency"`
	TotalTokensIn  int           `json:"total_tokens_in"`
	TotalTokensOut int           `json:"total_tokens_out"`
	ErrorCount     int           `json:"error_count"`
	Throughput     float64       `json:"throughput_rps"` // requests per second
}

// String returns a human-readable benchmark summary.
func (r *BenchmarkResult) String() string {
	return fmt.Sprintf(
		"Benchmark: %d requests, %d errors\n"+
			"  Duration:   %s\n"+
			"  Throughput: %.2f req/s\n"+
			"  Latency:    avg=%s p50=%s p95=%s p99=%s\n"+
			"  Tokens:     in=%d out=%d",
		r.TotalRequests, r.ErrorCount,
		r.TotalDuration.Round(time.Millisecond),
		r.Throughput,
		r.AvgLatency.Round(time.Millisecond),
		r.P50Latency.Round(time.Millisecond),
		r.P95Latency.Round(time.Millisecond),
		r.P99Latency.Round(time.Millisecond),
		r.TotalTokensIn, r.TotalTokensOut,
	)
}

// Benchmark runs a set of prompts against an agent and measures performance.
// Each prompt is sent as an independent request with its own session.
func Benchmark(ctx context.Context, agent *Core, cfg BenchmarkConfig) *BenchmarkResult {
	concurrency := cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	// Warmup rounds (results discarded).
	for i := 0; i < cfg.Warmup; i++ {
		for _, prompt := range cfg.Prompts {
			session := NewSession(generateRunID())
			agent.Run(ctx, session, prompt)
		}
	}

	// Build tasks.
	tasks := make([]WorkerTask, len(cfg.Prompts))
	for i, prompt := range cfg.Prompts {
		tasks[i] = WorkerTask{
			ID:      fmt.Sprintf("bench-%d", i),
			Message: prompt,
		}
	}

	start := time.Now()
	results := Batch(ctx, agent, tasks, concurrency)
	totalDuration := time.Since(start)

	// Compute statistics.
	var (
		latencies  []time.Duration
		tokensIn   int
		tokensOut  int
		errorCount int
	)
	for _, r := range results {
		if r.Err != nil {
			errorCount++
			continue
		}
		latencies = append(latencies, r.Duration)
		if r.Response != nil && r.Response.Metrics != nil {
			tokensIn += r.Response.Metrics.InputTokens
			tokensOut += r.Response.Metrics.OutputTokens
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	br := &BenchmarkResult{
		TotalRequests:  len(cfg.Prompts),
		TotalDuration:  totalDuration,
		TotalTokensIn:  tokensIn,
		TotalTokensOut: tokensOut,
		ErrorCount:     errorCount,
	}

	if totalDuration > 0 {
		br.Throughput = float64(len(cfg.Prompts)) / totalDuration.Seconds()
	}

	if len(latencies) > 0 {
		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		br.AvgLatency = total / time.Duration(len(latencies))
		br.P50Latency = percentile(latencies, 50)
		br.P95Latency = percentile(latencies, 95)
		br.P99Latency = percentile(latencies, 99)
	}

	return br
}

// percentile returns the p-th percentile from a sorted slice of durations.
func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
