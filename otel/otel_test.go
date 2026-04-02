package otel

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saeedalam/agnogo"
)

func TestExporterTrace(t *testing.T) {
	exp := NewExporter("http://localhost:4318/v1/metrics")

	trace := exp.Trace()
	if trace.OnModelCall == nil {
		t.Fatal("OnModelCall should be set")
	}
	if trace.OnToolCall == nil {
		t.Fatal("OnToolCall should be set")
	}
	if trace.OnGuardrail == nil {
		t.Fatal("OnGuardrail should be set")
	}

	// Simulate model calls.
	trace.OnModelCall(nil, &agnogo.ModelResponse{
		Usage: &agnogo.Usage{InputTokens: 100, OutputTokens: 50},
	}, 200*time.Millisecond)

	trace.OnModelCall(nil, &agnogo.ModelResponse{
		Usage: &agnogo.Usage{InputTokens: 200, OutputTokens: 100},
	}, 300*time.Millisecond)

	// Simulate tool calls.
	trace.OnToolCall("search", nil, "ok", 50*time.Millisecond, nil)
	trace.OnToolCall("search", nil, "ok", 60*time.Millisecond, nil)
	trace.OnToolCall("read_file", nil, "ok", 10*time.Millisecond, nil)

	// Simulate guardrail block.
	trace.OnGuardrail("pii", "output", true)

	// Take snapshot.
	exp.mu.Lock()
	snap := exp.snapshot()
	exp.mu.Unlock()

	if snap.runs != 2 {
		t.Errorf("runs = %d, want 2", snap.runs)
	}
	if snap.modelCalls != 2 {
		t.Errorf("modelCalls = %d, want 2", snap.modelCalls)
	}
	if snap.toolCalls != 3 {
		t.Errorf("toolCalls = %d, want 3", snap.toolCalls)
	}
	if snap.tokensIn != 300 {
		t.Errorf("tokensIn = %d, want 300", snap.tokensIn)
	}
	if snap.tokensOut != 150 {
		t.Errorf("tokensOut = %d, want 150", snap.tokensOut)
	}
	if snap.errors != 1 {
		t.Errorf("errors = %d, want 1", snap.errors)
	}
	if snap.guardBlocks != 1 {
		t.Errorf("guardBlocks = %d, want 1", snap.guardBlocks)
	}
	if snap.toolCounts["search"] != 2 {
		t.Errorf("toolCounts[search] = %d, want 2", snap.toolCounts["search"])
	}
	if snap.toolCounts["read_file"] != 1 {
		t.Errorf("toolCounts[read_file] = %d, want 1", snap.toolCounts["read_file"])
	}
}

func TestOTLPPayloadFormat(t *testing.T) {
	exp := NewExporter("http://localhost:4318/v1/metrics", WithServiceName("test-agent"))

	exp.mu.Lock()
	exp.runs = 5
	exp.modelCalls = 10
	exp.toolCalls = 3
	exp.tokensIn = 1000
	exp.tokensOut = 500
	snap := exp.snapshot()
	exp.mu.Unlock()

	payload := exp.buildOTLP(snap)

	// Validate structure.
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Should contain resourceMetrics.
	var raw map[string]any
	json.Unmarshal(data, &raw)

	rm, ok := raw["resourceMetrics"].([]any)
	if !ok || len(rm) == 0 {
		t.Fatal("missing resourceMetrics")
	}

	rmObj := rm[0].(map[string]any)
	resource := rmObj["resource"].(map[string]any)
	attrs := resource["attributes"].([]any)
	if len(attrs) < 1 {
		t.Fatal("missing resource attributes")
	}

	// Check service name.
	attr0 := attrs[0].(map[string]any)
	if attr0["key"] != "service.name" {
		t.Errorf("first attribute key = %v", attr0["key"])
	}
	val := attr0["value"].(map[string]any)
	if val["stringValue"] != "test-agent" {
		t.Errorf("service.name = %v", val["stringValue"])
	}

	// Check scope metrics exist.
	sm := rmObj["scopeMetrics"].([]any)
	if len(sm) == 0 {
		t.Fatal("missing scopeMetrics")
	}
	scope := sm[0].(map[string]any)
	metrics := scope["metrics"].([]any)
	if len(metrics) < 8 {
		t.Errorf("expected at least 8 metrics, got %d", len(metrics))
	}
}

func TestExporterFlush(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewExporter(server.URL, WithServiceName("flush-test"))
	trace := exp.Trace()
	trace.OnModelCall(nil, &agnogo.ModelResponse{
		Usage: &agnogo.Usage{InputTokens: 50, OutputTokens: 25},
	}, 100*time.Millisecond)

	err := exp.Flush()
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if len(received) == 0 {
		t.Fatal("expected data to be sent to endpoint")
	}

	// Verify it's valid JSON.
	var payload map[string]any
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("received invalid JSON: %v", err)
	}
	if payload["resourceMetrics"] == nil {
		t.Error("payload missing resourceMetrics")
	}
}

func TestExporterFlushError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	exp := NewExporter(server.URL)
	err := exp.Flush()
	if err == nil {
		t.Error("expected error on 500 response")
	}
}

func TestExporterStop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := NewExporter(server.URL, WithInterval(50*time.Millisecond))
	time.Sleep(10 * time.Millisecond) // let pusher start

	err := exp.Stop()
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Should not panic on double stop.
	_ = exp.Stop()
}

func TestNilUsage(t *testing.T) {
	exp := NewExporter("http://localhost:4318/v1/metrics")
	trace := exp.Trace()

	// Nil usage should not panic.
	trace.OnModelCall(nil, &agnogo.ModelResponse{Usage: nil}, 100*time.Millisecond)
	trace.OnModelCall(nil, nil, 50*time.Millisecond)

	exp.mu.Lock()
	if exp.runs != 2 {
		t.Errorf("runs = %d, want 2", exp.runs)
	}
	if exp.tokensIn != 0 {
		t.Errorf("tokensIn = %d, want 0", exp.tokensIn)
	}
	exp.mu.Unlock()
}
