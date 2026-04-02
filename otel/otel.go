package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saeedalam/agnogo"
)

// Exporter collects agent metrics and exports them in OTLP JSON format.
type Exporter struct {
	endpoint    string
	serviceName string
	interval    time.Duration
	client      *http.Client

	mu          sync.Mutex
	runs        int64
	modelCalls  int64
	toolCalls   int64
	errors      int64
	tokensIn    int64
	tokensOut   int64
	latencies   []time.Duration
	toolCounts  map[string]int64
	guardBlocks int64
	stopCh      chan struct{}
	running     atomic.Bool
}

// ExporterOption configures the Exporter.
type ExporterOption func(*Exporter)

// WithInterval sets how often metrics are pushed to the OTLP endpoint.
// Default: manual flush only (no periodic push).
func WithInterval(d time.Duration) ExporterOption {
	return func(e *Exporter) { e.interval = d }
}

// WithServiceName sets the service.name resource attribute.
// Default: "agnogo".
func WithServiceName(name string) ExporterOption {
	return func(e *Exporter) { e.serviceName = name }
}

// WithHTTPClient sets a custom HTTP client for exporting.
func WithHTTPClient(c *http.Client) ExporterOption {
	return func(e *Exporter) { e.client = c }
}

// NewExporter creates an OTLP metrics exporter.
// The endpoint should be an OTLP HTTP receiver, e.g.:
//
//	"http://localhost:4318/v1/metrics"      // OTel Collector
//	"https://api.datadoghq.com/v1/metrics"  // Datadog
func NewExporter(endpoint string, opts ...ExporterOption) *Exporter {
	e := &Exporter{
		endpoint:    endpoint,
		serviceName: "agnogo",
		client:      &http.Client{Timeout: 10 * time.Second},
		toolCounts:  make(map[string]int64),
		stopCh:      make(chan struct{}),
	}
	for _, opt := range opts {
		opt(e)
	}
	if e.interval > 0 {
		e.startPusher()
	}
	return e
}

// Trace returns a *Trace with all hooks wired to this exporter.
// Use with agnogo.WithTrace():
//
//	agent := agnogo.Agent("...", agnogo.WithTrace(exporter.Trace()))
func (e *Exporter) Trace() *agnogo.Trace {
	return &agnogo.Trace{
		OnModelCall: func(_ []agnogo.Message, resp *agnogo.ModelResponse, dur time.Duration) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.runs++
			e.modelCalls++
			e.latencies = append(e.latencies, dur)
			if resp != nil && resp.Usage != nil {
				e.tokensIn += int64(resp.Usage.InputTokens)
				e.tokensOut += int64(resp.Usage.OutputTokens)
			}
		},
		OnToolCall: func(name string, _ map[string]string, _ string, _ time.Duration, err error) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.toolCalls++
			e.toolCounts[name]++
			if err != nil {
				e.errors++
			}
		},
		OnGuardrail: func(_ string, _ string, blocked bool) {
			if blocked {
				e.mu.Lock()
				e.guardBlocks++
				e.errors++
				e.mu.Unlock()
			}
		},
	}
}

// Flush exports the current metrics to the OTLP endpoint immediately.
func (e *Exporter) Flush() error {
	return e.export()
}

// Stop halts periodic pushing and flushes remaining metrics.
func (e *Exporter) Stop() error {
	if e.running.CompareAndSwap(true, false) {
		close(e.stopCh)
	}
	return e.Flush()
}

// ── OTLP JSON format ────────────────────────────────────────────────

func (e *Exporter) export() error {
	e.mu.Lock()
	snap := e.snapshot()
	e.mu.Unlock()

	payload := e.buildOTLP(snap)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("otel: marshal: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("otel: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("otel: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("otel: endpoint returned %d", resp.StatusCode)
	}
	return nil
}

type metricsSnapshot struct {
	runs        int64
	modelCalls  int64
	toolCalls   int64
	errors      int64
	tokensIn    int64
	tokensOut   int64
	guardBlocks int64
	avgLatency  time.Duration
	p99Latency  time.Duration
	toolCounts  map[string]int64
}

func (e *Exporter) snapshot() metricsSnapshot {
	snap := metricsSnapshot{
		runs:        e.runs,
		modelCalls:  e.modelCalls,
		toolCalls:   e.toolCalls,
		errors:      e.errors,
		tokensIn:    e.tokensIn,
		tokensOut:   e.tokensOut,
		guardBlocks: e.guardBlocks,
		toolCounts:  make(map[string]int64, len(e.toolCounts)),
	}
	for k, v := range e.toolCounts {
		snap.toolCounts[k] = v
	}
	if n := len(e.latencies); n > 0 {
		var sum time.Duration
		for _, d := range e.latencies {
			sum += d
		}
		snap.avgLatency = sum / time.Duration(n)
	}
	return snap
}

// ── OTLP JSON payload builders ──────────────────────────────────────

func (e *Exporter) buildOTLP(snap metricsSnapshot) map[string]any {
	now := time.Now().UnixNano()

	gauges := []map[string]any{
		e.otlpGauge("agnogo.runs.total", "Total agent runs", snap.runs, now),
		e.otlpGauge("agnogo.model_calls.total", "Total LLM model calls", snap.modelCalls, now),
		e.otlpGauge("agnogo.tool_calls.total", "Total tool invocations", snap.toolCalls, now),
		e.otlpGauge("agnogo.errors.total", "Total errors (guardrail blocks + tool failures)", snap.errors, now),
		e.otlpGauge("agnogo.tokens.input", "Total input tokens consumed", snap.tokensIn, now),
		e.otlpGauge("agnogo.tokens.output", "Total output tokens generated", snap.tokensOut, now),
		e.otlpGauge("agnogo.guardrail.blocks", "Total guardrail blocks", snap.guardBlocks, now),
		e.otlpGauge("agnogo.latency.avg_ms", "Average model call latency (ms)", snap.avgLatency.Milliseconds(), now),
	}

	// Per-tool call counts.
	for tool, count := range snap.toolCounts {
		gauges = append(gauges, e.otlpGaugeWithAttrs(
			"agnogo.tool.calls", "Tool call count",
			count, now,
			map[string]string{"tool.name": tool},
		))
	}

	return map[string]any{
		"resourceMetrics": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []map[string]any{
						{"key": "service.name", "value": map[string]any{"stringValue": e.serviceName}},
						{"key": "service.version", "value": map[string]any{"stringValue": "0.4.0"}},
					},
				},
				"scopeMetrics": []map[string]any{
					{
						"scope": map[string]any{
							"name":    "agnogo.otel",
							"version": "0.1.0",
						},
						"metrics": gauges,
					},
				},
			},
		},
	}
}

func (e *Exporter) otlpGauge(name, desc string, value int64, timeNano int64) map[string]any {
	return map[string]any{
		"name":        name,
		"description": desc,
		"unit":        "1",
		"gauge": map[string]any{
			"dataPoints": []map[string]any{
				{
					"timeUnixNano": fmt.Sprintf("%d", timeNano),
					"asInt":        fmt.Sprintf("%d", value),
				},
			},
		},
	}
}

func (e *Exporter) otlpGaugeWithAttrs(name, desc string, value int64, timeNano int64, attrs map[string]string) map[string]any {
	attrList := make([]map[string]any, 0, len(attrs))
	for k, v := range attrs {
		attrList = append(attrList, map[string]any{
			"key":   k,
			"value": map[string]any{"stringValue": v},
		})
	}
	return map[string]any{
		"name":        name,
		"description": desc,
		"unit":        "1",
		"gauge": map[string]any{
			"dataPoints": []map[string]any{
				{
					"timeUnixNano": fmt.Sprintf("%d", timeNano),
					"asInt":        fmt.Sprintf("%d", value),
					"attributes":   attrList,
				},
			},
		},
	}
}

// ── Periodic push ───────────────────────────────────────────────────

func (e *Exporter) startPusher() {
	e.running.Store(true)
	go func() {
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = e.export()
			case <-e.stopCh:
				return
			}
		}
	}()
}
