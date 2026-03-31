package agnogo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestAgent(text string) *Core {
	return New(Config{
		Model:        &mockModel{responses: []ModelResponse{{Text: text}}},
		Instructions: "Test agent",
	})
}

func TestServeHealth(t *testing.T) {
	a := newTestAgent("hi")
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want 'ok'", body.Status)
	}
}

func TestServeTools(t *testing.T) {
	a := newTestAgent("hi")
	a.Tool("search", "Search the web", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "result", nil
	})
	a.Tool("calc", "Calculate", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "42", nil
	})

	handler := a.Handler()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var tools []toolInfo
	if err := json.NewDecoder(rec.Body).Decode(&tools); err != nil {
		t.Fatal(err)
	}
	if len(tools) != 2 {
		t.Fatalf("tools count = %d, want 2", len(tools))
	}
	names := map[string]bool{}
	for _, ti := range tools {
		names[ti.Name] = true
	}
	if !names["search"] || !names["calc"] {
		t.Errorf("tools = %v, want search and calc", tools)
	}
}

func TestServeAsk(t *testing.T) {
	a := newTestAgent("Hello from agent!")
	handler := a.Handler()

	body, _ := json.Marshal(askRequest{Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp askResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hello from agent!" {
		t.Errorf("text = %q", resp.Text)
	}
}

func TestServeAskEmptyMessage(t *testing.T) {
	a := newTestAgent("hi")
	handler := a.Handler()

	body, _ := json.Marshal(askRequest{Message: ""})
	req := httptest.NewRequest(http.MethodPost, "/ask", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestServeAskInvalidJSON(t *testing.T) {
	a := newTestAgent("hi")
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/ask", strings.NewReader("not json{{{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestServeAuth(t *testing.T) {
	a := newTestAgent("secret")
	handler := a.Handler(WithAuth("my-token"))

	// Without token -> 401
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: status = %d, want 401", rec.Code)
	}

	// With token -> 200
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with auth: status = %d, want 200", rec.Code)
	}
}

func TestServeCORS(t *testing.T) {
	a := newTestAgent("hi")
	handler := a.Handler(WithCORS("*"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("CORS origin = %q, want '*'", origin)
	}
	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("CORS methods header missing")
	}
}

func TestServeStream(t *testing.T) {
	a := newTestAgent("streaming response here")
	handler := a.Handler()

	body, _ := json.Marshal(askRequest{Message: "stream me"})
	req := httptest.NewRequest(http.MethodPost, "/stream", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("content-type = %q, want text/event-stream", ct)
	}

	// Read SSE events
	scanner := bufio.NewScanner(rec.Body)
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if len(dataLines) == 0 {
		t.Fatal("no SSE data lines received")
	}

	// Last event should have "done":true
	lastData := dataLines[len(dataLines)-1]
	var doneEvt map[string]any
	if err := json.Unmarshal([]byte(lastData), &doneEvt); err != nil {
		t.Fatalf("cannot parse last event: %v", err)
	}
	if doneEvt["done"] != true {
		t.Errorf("last event should be done=true, got %v", doneEvt)
	}

	// At least one text event before done
	if len(dataLines) < 2 {
		t.Error("expected at least one text event before done")
	}
}
