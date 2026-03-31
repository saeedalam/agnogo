package agnogo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── Mock Model Provider ──────────────────────────────────

type mockModel struct {
	mu        sync.Mutex
	responses []ModelResponse
	callCount int
}

func (m *mockModel) ChatCompletion(_ context.Context, messages []Message, tools []map[string]any) (*ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.callCount >= len(m.responses) {
		return &ModelResponse{Text: "No more responses"}, nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return &resp, nil
}

// ── Agent Run Tests ──────────────────────────────────────

func TestAgentSimpleReply(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "Hello! How can I help?"},
	}}

	a := New(Config{
		Model:        model,
		Instructions: "You are a helpful assistant.",
	})

	session := NewSession("test-1")
	resp, err := a.Run(context.Background(), session, "Hi!")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hello! How can I help?" {
		t.Errorf("text = %q", resp.Text)
	}
	if len(session.History) != 2 { // user + assistant
		t.Errorf("history len = %d, want 2", len(session.History))
	}
}

func TestAgentToolCall(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		// First: model wants to call a tool
		{ToolCalls: []ToolCall{{ID: "call-1", Name: "get_time", Arguments: `{}`}}},
		// Second: model responds with text
		{Text: "The current time is 14:30."},
	}}

	a := New(Config{Model: model})
	a.Tool("get_time", "Get current time", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "14:30", nil
	})

	session := NewSession("test-2")
	resp, err := a.Run(context.Background(), session, "What time is it?")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "The current time is 14:30." {
		t.Errorf("text = %q", resp.Text)
	}
	if len(resp.ToolsCalled) != 1 || resp.ToolsCalled[0] != "get_time" {
		t.Errorf("tools called = %v", resp.ToolsCalled)
	}
}

func TestAgentDuplicateToolBlocked(t *testing.T) {
	callCount := 0
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "fetch", Arguments: `{"url":"x"}`}}},
		{ToolCalls: []ToolCall{{ID: "c2", Name: "fetch", Arguments: `{"url":"x"}`}}},
		{ToolCalls: []ToolCall{{ID: "c3", Name: "fetch", Arguments: `{"url":"x"}`}}},
		{Text: "I couldn't fetch that."},
	}}

	a := New(Config{Model: model})
	a.Tool("fetch", "Fetch URL", nil, func(ctx context.Context, args map[string]string) (string, error) {
		callCount++
		return "error", fmt.Errorf("timeout")
	})

	session := NewSession("test-3")
	resp, _ := a.Run(context.Background(), session, "Fetch x")
	// Tool should be called max 2 times (3rd is blocked)
	if callCount > 2 {
		t.Errorf("tool called %d times, max should be 2", callCount)
	}
	_ = resp
}

func TestAgentMaxLoops(t *testing.T) {
	// Model always returns tool calls — should hit max loops
	model := &mockModel{}
	for i := 0; i < 20; i++ {
		model.responses = append(model.responses, ModelResponse{
			ToolCalls: []ToolCall{{ID: fmt.Sprintf("c%d", i), Name: "loop", Arguments: `{}`}},
		})
	}

	a := New(Config{Model: model, MaxLoops: 3})
	a.Tool("loop", "Loop forever", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "again", nil
	})

	session := NewSession("test-4")
	resp, _ := a.Run(context.Background(), session, "Loop")
	if !strings.Contains(resp.Text, "couldn't complete") {
		t.Errorf("expected fallback text, got %q", resp.Text)
	}
}

func TestAgentFallbackText(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 20; i++ {
		model.responses = append(model.responses, ModelResponse{
			ToolCalls: []ToolCall{{ID: fmt.Sprintf("c%d", i), Name: "x", Arguments: `{}`}},
		})
	}

	a := New(Config{Model: model, MaxLoops: 1, FallbackText: "Custom fallback!"})
	a.Tool("x", "X", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "ok", nil
	})

	session := NewSession("test-5")
	resp, _ := a.Run(context.Background(), session, "Do X")
	if resp.Text != "Custom fallback!" {
		t.Errorf("text = %q, want custom fallback", resp.Text)
	}
}

// ── Guardrail Tests ──────────────────────────────────────

func TestInputGuardrail(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "OK"}}}
	a := New(Config{Model: model})
	a.InputGuardrail("no-spam", func(ctx context.Context, s *Session, msg string) error {
		if strings.Contains(msg, "spam") {
			return errors.New("Spam detected.")
		}
		return nil
	})

	session := NewSession("test-g1")
	resp, _ := a.Run(context.Background(), session, "This is spam content")
	if resp.Text != "Spam detected." {
		t.Errorf("guardrail didn't block: %q", resp.Text)
	}
	if model.callCount != 0 {
		t.Error("model should not have been called when guardrail blocks")
	}
}

func TestOutputGuardrail(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "Here's my secret: ABC123"}}}
	a := New(Config{Model: model})
	a.OutputGuardrail("no-secrets", func(ctx context.Context, s *Session, msg string) error {
		if strings.Contains(msg, "secret") {
			return errors.New("I cannot share that information.")
		}
		return nil
	})

	session := NewSession("test-g2")
	resp, _ := a.Run(context.Background(), session, "Tell me a secret")
	if resp.Text != "I cannot share that information." {
		t.Errorf("output guardrail didn't block: %q", resp.Text)
	}
}

// ── Human Approval Tests ─────────────────────────────────

func TestHumanApproval(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "dangerous", Arguments: `{"action":"delete"}`}}},
	}}

	a := New(Config{Model: model})
	a.ToolWithApproval("dangerous", "Dangerous action", Params{
		"action": {Type: "string", Desc: "Action"},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return "Done: " + args["action"], nil
	}, "This action requires admin approval")

	session := NewSession("test-h1")
	resp, _ := a.Run(context.Background(), session, "Delete everything")

	if !resp.NeedsApproval {
		t.Error("expected NeedsApproval=true")
	}
	if resp.Approval == nil {
		t.Fatal("expected non-nil Approval")
	}
	if resp.Approval.ToolName != "dangerous" {
		t.Errorf("approval tool = %q", resp.Approval.ToolName)
	}
	if resp.Approval.Reason != "This action requires admin approval" {
		t.Errorf("approval reason = %q", resp.Approval.Reason)
	}

	// State should be saved for resume
	if session.GetStr("_pending_tool") != "dangerous" {
		t.Error("pending tool not saved in session state")
	}
}

// ── Knowledge Tests ──────────────────────────────────────

func TestKnowledgeInjection(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "What are your opening hours?"},
	}

	k := KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
		return "Opening hours: Mon-Fri 9-17", nil
	})

	result := injectKnowledge(context.Background(), k, "What are your opening hours?", messages, 3)
	if len(result) != 3 { // system + knowledge + user
		t.Errorf("expected 3 messages, got %d", len(result))
	}
	if !strings.Contains(result[1].Content, "Opening hours") {
		t.Error("knowledge not injected")
	}
}

func TestKnowledgeSkippedForNonQuestion(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Book a haircut"},
	}

	called := false
	k := KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
		called = true
		return "stuff", nil
	})

	result := injectKnowledge(context.Background(), k, "Book a haircut", messages, 3)
	if called {
		t.Error("knowledge should not be searched for non-questions")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

// ── Memory Tests ─────────────────────────────────────────

func TestPatternMemorySwedish(t *testing.T) {
	m := DefaultPatternMemory()
	s := NewSession("test")

	m.Extract(context.Background(), s, "Jag heter Anna", "Hej Anna!")
	if s.GetMemory("name") != "Anna" {
		t.Errorf("name = %q, want Anna", s.GetMemory("name"))
	}
}

func TestPatternMemoryNameWithComma(t *testing.T) {
	m := DefaultPatternMemory()
	s := NewSession("test")

	m.Extract(context.Background(), s, "My name is Erik, nice to meet you", "Hi Erik!")
	if s.GetMemory("name") != "Erik" {
		t.Errorf("name = %q, want Erik", s.GetMemory("name"))
	}
}

func TestPatternMemoryEmailInSentence(t *testing.T) {
	m := DefaultPatternMemory()
	s := NewSession("test")

	m.Extract(context.Background(), s, "You can reach me at john@company.com for details", "Got it!")
	if s.GetMemory("email") != "john@company.com" {
		t.Errorf("email = %q", s.GetMemory("email"))
	}
}

// ── Storage Tests ────────────────────────────────────────

func TestMemoryStorageRoundtrip(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	s := NewSession("roundtrip-1")
	s.SetMemory("name", "Test")
	s.Set("step", "verify")
	s.SetMeta("org", "acme")
	s.AddMessage("user", "hello")

	store.Save(ctx, s)

	loaded, err := store.Load(ctx, "roundtrip-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetMemory("name") != "Test" {
		t.Error("memory lost")
	}
	if loaded.GetStr("step") != "verify" {
		t.Error("state lost")
	}
	if loaded.GetMeta("org") != "acme" {
		t.Error("metadata lost")
	}
	if len(loaded.History) != 1 {
		t.Errorf("history len = %d", len(loaded.History))
	}
}

// ── Trace Tests ──────────────────────────────────────────

func TestTraceToolCall(t *testing.T) {
	var traced []string
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "greet", Arguments: `{"name":"World"}`}}},
		{Text: "Hello World!"},
	}}

	a := New(Config{
		Model: model,
		Trace: &Trace{
			OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) {
				traced = append(traced, name)
			},
		},
	})
	a.Tool("greet", "Greet", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "Hello " + args["name"], nil
	})

	session := NewSession("test-trace")
	a.Run(context.Background(), session, "Greet World")

	if len(traced) != 1 || traced[0] != "greet" {
		t.Errorf("traced = %v, want [greet]", traced)
	}
}

func TestTraceGuardrail(t *testing.T) {
	var blocked bool
	model := &mockModel{responses: []ModelResponse{{Text: "OK"}}}

	a := New(Config{
		Model: model,
		Trace: &Trace{
			OnGuardrail: func(name, direction string, b bool) {
				blocked = b
			},
		},
	})
	a.InputGuardrail("block-all", func(ctx context.Context, s *Session, msg string) error {
		return errors.New("blocked")
	})

	session := NewSession("test-trace-g")
	a.Run(context.Background(), session, "anything")

	if !blocked {
		t.Error("trace should have recorded guardrail block")
	}
}

// ── Auto Memory with Agent ───────────────────────────────

func TestAgentAutoMemory(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: "Nice to meet you, Erik!"}}}

	a := New(Config{
		Model:      model,
		AutoMemory: true,
	})

	session := NewSession("test-mem")
	a.Run(context.Background(), session, "My name is Erik")

	if session.GetMemory("name") != "Erik" {
		t.Errorf("auto memory: name = %q, want Erik", session.GetMemory("name"))
	}
}

// ── RunWithStorage ───────────────────────────────────────

func TestRunWithStorage(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: "Hello!"},
		{Text: "I remember you!"},
	}}
	store := NewMemoryStorage()

	a := New(Config{Model: model, Storage: store})

	// First run — creates session
	resp, err := a.RunWithStorage(context.Background(), "ws-1", "Hi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Hello!" {
		t.Errorf("text = %q", resp.Text)
	}

	// Second run — loads session from storage
	resp, err = a.RunWithStorage(context.Background(), "ws-1", "Remember me?")
	if err != nil {
		t.Fatal(err)
	}

	// Verify session was loaded (should have history from first run)
	loaded, _ := store.Load(context.Background(), "ws-1")
	if len(loaded.History) != 4 { // user1 + assistant1 + user2 + assistant2
		t.Errorf("history len = %d, want 4", len(loaded.History))
	}
}

// ── FunctionDefs Format ──────────────────────────────────

func TestFunctionDefsFormat(t *testing.T) {
	r := NewToolRegistry()
	r.Add("search", "Search things", Params{
		"query":  {Type: "string", Desc: "Search query", Required: true},
		"limit":  {Type: "number", Desc: "Max results"},
		"format": {Type: "string", Desc: "Output format", Enum: []string{"json", "text"}},
	}, nil)

	defs := r.FunctionDefs()
	if len(defs) != 1 {
		t.Fatalf("defs len = %d", len(defs))
	}

	def := defs[0]
	if def["type"] != "function" {
		t.Errorf("type = %v", def["type"])
	}
	fn := def["function"].(map[string]any)
	if fn["name"] != "search" {
		t.Errorf("name = %v", fn["name"])
	}
	params := fn["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	if len(props) != 3 {
		t.Errorf("props count = %d", len(props))
	}
	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "query" {
		t.Errorf("required = %v", required)
	}
}

// ── Concurrent Tool Registry ─────────────────────────────

func TestToolRegistryConcurrent(t *testing.T) {
	r := NewToolRegistry()
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			r.Add(fmt.Sprintf("tool-%d", n), "desc", nil,
				func(ctx context.Context, args map[string]string) (string, error) {
					return "ok", nil
				})
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(n int) {
			r.Get(fmt.Sprintf("tool-%d", n))
			r.List()
			r.FunctionDefs()
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if r.Count() != 10 {
		t.Errorf("count = %d, want 10", r.Count())
	}
}
