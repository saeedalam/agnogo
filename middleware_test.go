package agnogo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentMiddleware(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	a := New(Config{Model: model, Instructions: "test"})

	var found *Core
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		found = AgentFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := AgentMiddleware(a)(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if found == nil {
		t.Fatal("AgentFromContext returned nil inside middleware chain")
	}
	if found != a {
		t.Error("AgentFromContext returned a different agent")
	}
}

func TestAgentFromContextNil(t *testing.T) {
	ctx := context.Background()
	a := AgentFromContext(ctx)
	if a != nil {
		t.Errorf("expected nil, got %v", a)
	}
}

func TestAgentHandler(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "Hello from agent!"}}}
	a := New(Config{Model: model, Instructions: "test"})

	handler := AgentHandler(a)
	body := `{"message":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp agentResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Text != "Hello from agent!" {
		t.Errorf("text = %q, want 'Hello from agent!'", resp.Text)
	}
	if resp.SessionID == "" {
		t.Error("expected non-empty session_id")
	}
}

func TestAgentHandlerBadMethod(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	a := New(Config{Model: model, Instructions: "test"})

	handler := AgentHandler(a)
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestAgentHandlerEmptyMessage(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "ok"}}}
	a := New(Config{Model: model, Instructions: "test"})

	handler := AgentHandler(a)
	body := `{"message":""}`
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
