package agnogo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// staticModel is a concurrency-safe model for benchmarks (no shared mutable state).
type staticModel struct{ text string }

func (m *staticModel) ChatCompletion(_ context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	return &ModelResponse{Text: m.text}, nil
}

// delayModel is a model that responds after a configurable delay.
type delayModel struct {
	delay time.Duration
	text  string
}

func (m *delayModel) ChatCompletion(ctx context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	select {
	case <-time.After(m.delay):
		return &ModelResponse{Text: m.text}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ── Tests ───────────────────────────────────────────────

func TestServeConcurrentRequests(t *testing.T) {
	agent := New(Config{
		Model:        &staticModel{text: "ok"},
		Instructions: "test",
	})

	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	const n = 50
	var wg sync.WaitGroup
	results := make([]int, n)
	errors := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Post(server.URL+"/ask", "application/json",
				strings.NewReader(`{"message":"hi"}`))
			if err != nil {
				errors[idx] = err
				return
			}
			defer resp.Body.Close()
			results[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}
	for i, code := range results {
		if code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, code)
		}
	}
}

func TestServeMaxConcurrent(t *testing.T) {
	agent := New(Config{
		Model:        &delayModel{delay: 50 * time.Millisecond, text: "ok"},
		Instructions: "test",
	})

	server := httptest.NewServer(agent.Handler(WithMaxConcurrent(5)))
	defer server.Close()

	const n = 20
	var wg sync.WaitGroup
	results := make([]int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Post(server.URL+"/ask", "application/json",
				strings.NewReader(`{"message":"hi"}`))
			if err != nil {
				results[idx] = -1
				return
			}
			defer resp.Body.Close()
			results[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	var ok, unavailable int
	for _, code := range results {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusServiceUnavailable:
			unavailable++
		}
	}

	t.Logf("ok=%d, 503=%d", ok, unavailable)
	if unavailable == 0 {
		t.Error("expected some 503 responses when concurrency is limited to 5 with 20 concurrent requests")
	}
	if ok == 0 {
		t.Error("expected some 200 responses")
	}
}

func TestServeMaxBodySize(t *testing.T) {
	agent := New(Config{
		Model:        &mockModel{responses: makeResponses("ok", 1)},
		Instructions: "test",
	})

	server := httptest.NewServer(agent.Handler(WithMaxBodySize(100)))
	defer server.Close()

	// Send a body larger than 100 bytes.
	largeBody := `{"message":"` + strings.Repeat("x", 200) + `"}`
	resp, err := http.Post(server.URL+"/ask", "application/json",
		strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// http.MaxBytesReader causes a 400 on read failure.
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 400 or 413", resp.StatusCode)
	}
}

func TestServeConcurrentStream(t *testing.T) {
	agent := New(Config{
		Model:        &staticModel{text: "streamed"},
		Instructions: "test",
	})

	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	const n = 20
	var wg sync.WaitGroup
	results := make([]int, n)
	gotDone := make([]bool, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := http.Post(server.URL+"/stream", "application/json",
				strings.NewReader(`{"message":"hi"}`))
			if err != nil {
				results[idx] = -1
				return
			}
			defer resp.Body.Close()
			results[idx] = resp.StatusCode

			// Check for done event in SSE response.
			var buf [4096]byte
			n, _ := resp.Body.Read(buf[:])
			body := string(buf[:n])
			if strings.Contains(body, `"done":true`) {
				gotDone[idx] = true
			}
		}(i)
	}
	wg.Wait()

	for i, code := range results {
		if code != http.StatusOK {
			t.Errorf("stream request %d: status = %d, want 200", i, code)
		}
	}

	doneCount := 0
	for _, d := range gotDone {
		if d {
			doneCount++
		}
	}
	if doneCount == 0 {
		t.Error("expected at least some SSE responses to contain done event")
	}
}

// ── Benchmarks ──────────────────────────────────────────

func BenchmarkServeAsk(b *testing.B) {
	agent := New(Config{
		Model:        &staticModel{text: "bench"},
		Instructions: "bench",
	})

	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	payload := `{"message":"hi"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post(server.URL+"/ask", "application/json",
			strings.NewReader(payload))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkServeAskConcurrent(b *testing.B) {
	agent := New(Config{
		Model:        &staticModel{text: "bench"},
		Instructions: "bench",
	})

	server := httptest.NewServer(agent.Handler())
	defer server.Close()

	payload := `{"message":"hi"}`

	b.SetParallelism(50)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Post(server.URL+"/ask", "application/json",
				strings.NewReader(payload))
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// ── Helpers ─────────────────────────────────────────────

// makeResponses creates a slice of n identical ModelResponse values.
func makeResponses(text string, n int) []ModelResponse {
	out := make([]ModelResponse, n)
	for i := range out {
		out[i] = ModelResponse{Text: text}
	}
	return out
}

