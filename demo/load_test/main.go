// Package main runs a standalone load test against agnogo's Serve() HTTP handler.
//
// Usage:
//
//	go run ./demo/load_test
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/saeedalam/agnogo"
)

// benchModel responds after a fixed delay, simulating real model latency.
type benchModel struct {
	delay time.Duration
}

func (m *benchModel) ChatCompletion(ctx context.Context, _ []agnogo.Message, _ []map[string]any) (*agnogo.ModelResponse, error) {
	select {
	case <-time.After(m.delay):
		return &agnogo.ModelResponse{Text: "Load test response"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type result struct {
	status  int
	latency time.Duration
	err     error
}

type stats struct {
	concurrency int
	requests    int
	duration    time.Duration
	p50         time.Duration
	p95         time.Duration
	p99         time.Duration
	errors      int
	throughput  float64
}

func main() {
	fmt.Println("=== agnogo Serve() Load Test ===")
	fmt.Println()

	agent := agnogo.New(agnogo.Config{
		Model:        &benchModel{delay: 10 * time.Millisecond},
		Instructions: "Load test agent",
	})

	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	concurrencyLevels := []int{1, 10, 50, 100}
	requestsPerLevel := 50

	fmt.Printf("%-12s| %-9s| %-9s| %-7s| %-7s| %-7s| %-7s| %s\n",
		"Concurrency", "Requests", "Duration", "p50", "p95", "p99", "Errors", "Throughput")
	fmt.Println(strings.Repeat("-", 82))

	for _, conc := range concurrencyLevels {
		s := runLoadTest(server.URL, conc, requestsPerLevel)
		fmt.Printf("%-12d| %-9d| %-9s| %-7s| %-7s| %-7s| %-7d| %d req/s\n",
			s.concurrency, s.requests, fmtDur(s.duration),
			fmtDur(s.p50), fmtDur(s.p95), fmtDur(s.p99),
			s.errors, int(s.throughput))
	}
}

func runLoadTest(baseURL string, concurrency, totalRequests int) stats {
	results := make([]result, totalRequests)
	var wg sync.WaitGroup

	sem := make(chan struct{}, concurrency)
	payload := `{"message":"load test"}`

	start := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{} // limit concurrency
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()
			resp, err := http.Post(baseURL+"/ask", "application/json",
				strings.NewReader(payload))
			latency := time.Since(reqStart)

			if err != nil {
				results[idx] = result{err: err, latency: latency}
				return
			}
			resp.Body.Close()
			results[idx] = result{status: resp.StatusCode, latency: latency}
		}(i)
	}
	wg.Wait()
	totalDuration := time.Since(start)

	// Collect latencies and errors.
	var latencies []time.Duration
	errCount := 0
	for _, r := range results {
		latencies = append(latencies, r.latency)
		if r.err != nil || (r.status != 0 && r.status != http.StatusOK) {
			errCount++
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	return stats{
		concurrency: concurrency,
		requests:    totalRequests,
		duration:    totalDuration,
		p50:         percentile(latencies, 0.50),
		p95:         percentile(latencies, 0.95),
		p99:         percentile(latencies, 0.99),
		errors:      errCount,
		throughput:  float64(totalRequests) / totalDuration.Seconds(),
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dus", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
