package agnogo

import (
	"context"
	"testing"
	"time"
)

func TestBenchmarkBasic(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 10; i++ {
		model.responses = append(model.responses, ModelResponse{
			Text:  "ok",
			Usage: &Usage{InputTokens: 10, OutputTokens: 5},
		})
	}

	a := New(Config{Model: model, Instructions: "bench"})

	result := Benchmark(context.Background(), a, BenchmarkConfig{
		Prompts:     []string{"one", "two", "three"},
		Concurrency: 2,
	})

	if result.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", result.TotalRequests)
	}
	if result.TotalDuration == 0 {
		t.Error("TotalDuration is zero")
	}
	if result.Throughput <= 0 {
		t.Errorf("Throughput = %f, want > 0", result.Throughput)
	}
	if result.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", result.ErrorCount)
	}
}

func TestBenchmarkString(t *testing.T) {
	result := &BenchmarkResult{
		TotalRequests:  5,
		TotalDuration:  2 * time.Second,
		AvgLatency:     400 * time.Millisecond,
		P50Latency:     350 * time.Millisecond,
		P95Latency:     800 * time.Millisecond,
		P99Latency:     950 * time.Millisecond,
		TotalTokensIn:  100,
		TotalTokensOut: 50,
		Throughput:     2.5,
	}

	s := result.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
	if len(s) < 50 {
		t.Errorf("String() output too short: %q", s)
	}
}

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	p50 := percentile(sorted, 50)
	if p50 != 60*time.Millisecond {
		t.Errorf("p50 = %v, want 60ms", p50)
	}

	p99 := percentile(sorted, 99)
	if p99 != 100*time.Millisecond {
		t.Errorf("p99 = %v, want 100ms", p99)
	}

	// Empty slice
	p0 := percentile(nil, 50)
	if p0 != 0 {
		t.Errorf("percentile of empty slice = %v, want 0", p0)
	}
}
