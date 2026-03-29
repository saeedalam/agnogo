package agnogo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ── Verification Tests ──────────────────────────────────
// These tests verify that every major feature of agnogo works correctly.
// Run with: go test -v -run TestVerify

func TestVerifyToolRegistration(t *testing.T) {
	a := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "ok"}}}})

	// Tool()
	a.Tool("t1", "Tool 1", nil, func(ctx context.Context, args map[string]string) (string, error) { return "1", nil })
	if a.Tools().Count() != 1 { t.Error("Tool() failed") }

	// AddTools()
	a.AddTools(ToolDef{Name: "t2", Desc: "Tool 2", Fn: func(ctx context.Context, args map[string]string) (string, error) { return "2", nil }})
	if a.Tools().Count() != 2 { t.Error("AddTools() failed") }

	// ToolWithApproval()
	a.ToolWithApproval("t3", "Tool 3", nil, func(ctx context.Context, args map[string]string) (string, error) { return "3", nil }, "reason")
	if !a.Tools().Get("t3").RequireApproval { t.Error("ToolWithApproval() failed") }

	// SetTools()
	a.SetTools(ToolDef{Name: "only", Desc: "Only tool", Fn: func(ctx context.Context, args map[string]string) (string, error) { return "only", nil }})
	if a.Tools().Count() != 1 || a.Tools().Get("only") == nil { t.Error("SetTools() failed") }

	// ClearTools()
	a.ClearTools()
	if a.Tools().Count() != 0 { t.Error("ClearTools() failed") }
}

func TestVerifyInputGuardrail(t *testing.T) {
	a := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "ok"}}}})
	a.InputGuardrail("block", func(ctx context.Context, s *Session, msg string) error {
		if msg == "bad" { return errors.New("blocked") }
		return nil
	})
	s := NewSession("test")
	resp, _ := a.Run(context.Background(), s, "bad")
	if resp.Text != "blocked" { t.Error("input guardrail didn't block") }
	resp, _ = a.Run(context.Background(), s, "good")
	if resp.Text != "ok" { t.Error("input guardrail blocked good input") }
}

func TestVerifyOutputGuardrail(t *testing.T) {
	a := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "secret 12345"}}}})
	a.OutputGuardrail("redact", func(ctx context.Context, s *Session, msg string) error {
		if strings.Contains(msg, "secret") { return errors.New("redacted") }
		return nil
	})
	s := NewSession("test")
	resp, _ := a.Run(context.Background(), s, "tell me")
	if resp.Text != "redacted" { t.Error("output guardrail didn't block") }
}

func TestVerifyAutoMemory(t *testing.T) {
	a := New(Config{
		Model:      &mockModel{responses: []ModelResponse{{Text: "Nice to meet you!"}}},
		AutoMemory: true,
	})
	s := NewSession("test")
	a.Run(context.Background(), s, "My name is Anna")
	if s.GetMemory("name") != "Anna" { t.Errorf("auto memory: name=%q", s.GetMemory("name")) }

	a.Run(context.Background(), s, "Contact me at anna@test.com please")
	if s.GetMemory("email") != "anna@test.com" { t.Errorf("auto memory: email=%q", s.GetMemory("email")) }
}

func TestVerifyKnowledge(t *testing.T) {
	a := New(Config{
		Model: &mockModel{responses: []ModelResponse{{Text: "We are open 9-17."}}},
		Knowledge: KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
			return "Opening hours: Monday-Friday 9:00-17:00", nil
		}),
	})
	s := NewSession("test")
	resp, _ := a.Run(context.Background(), s, "What are your opening hours?")
	if resp.Text == "" { t.Error("knowledge agent returned empty") }
}

func TestVerifyStorage(t *testing.T) {
	store := NewMemoryStorage()
	a := New(Config{
		Model:   &mockModel{responses: []ModelResponse{{Text: "hi"}, {Text: "remembered"}}},
		Storage: store,
	})

	// RunWithStorage creates + saves
	resp, _ := a.RunWithStorage(context.Background(), "s1", "hello")
	if resp.Text != "hi" { t.Error("first run failed") }

	// Session persisted
	s, err := store.Load(context.Background(), "s1")
	if err != nil { t.Fatal("session not saved") }
	if len(s.History) < 2 { t.Error("history not saved") }

	// Delete
	store.Delete(context.Background(), "s1")
	_, err = store.Load(context.Background(), "s1")
	if err != ErrSessionNotFound { t.Error("delete didn't work") }

	// List
	store.Save(context.Background(), NewSession("a"))
	store.Save(context.Background(), NewSession("b"))
	list, _ := store.List(context.Background(), 10)
	if len(list) != 2 { t.Errorf("list: %d", len(list)) }
}

func TestVerifyKnowledgeStore(t *testing.T) {
	store := NewMemoryStorage()
	store.AddKnowledge(context.Background(), "hours", "9-17 Mon-Fri")
	store.AddKnowledge(context.Background(), "parking", "Free parking available")

	entries, _ := store.ListKnowledge(context.Background())
	if len(entries) != 2 { t.Errorf("knowledge entries: %d", len(entries)) }

	store.DeleteKnowledge(context.Background(), "parking")
	entries, _ = store.ListKnowledge(context.Background())
	if len(entries) != 1 { t.Error("delete knowledge failed") }
}

func TestVerifyTeam(t *testing.T) {
	booking := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "Booking handled."}}}})
	support := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "Support handled."}}}})

	team := NewTeam(TeamConfig{
		RouterFunc: func(ctx context.Context, msg string, agents []string) (string, error) {
			if strings.Contains(strings.ToLower(msg), "book") { return "booking", nil }
			return "support", nil
		},
	})
	team.Agent("booking", booking)
	team.Agent("support", support)

	s := NewSession("test")
	resp, _ := team.Run(context.Background(), s, "I want to book")
	if resp.Text != "Booking handled." { t.Errorf("team routing: %q", resp.Text) }

	resp, _ = team.Run(context.Background(), s, "I need help")
	if resp.Text != "Support handled." { t.Errorf("team routing: %q", resp.Text) }
}

func TestVerifySequentialWorkflow(t *testing.T) {
	s1 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "extracted data"}}}})
	s2 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "validated + processed"}}}})
	wf := Sequential(Step("extract", s1), Step("validate", s2))
	session := NewSession("test")
	resp, _ := wf.Run(context.Background(), session, "input")
	if resp.Text != "validated + processed" { t.Errorf("sequential: %q", resp.Text) }
}

func TestVerifyParallelWorkflow(t *testing.T) {
	a1 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "result A"}}}})
	a2 := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "result B"}}}})
	wf := Parallel(Step("A", a1), Step("B", a2))
	session := NewSession("test")
	resp, _ := wf.Run(context.Background(), session, "input")
	if !strings.Contains(resp.Text, "result") { t.Errorf("parallel: %q", resp.Text) }
}

func TestVerifyConditionWorkflow(t *testing.T) {
	urgent := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "URGENT handled"}}}})
	normal := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "Normal handled"}}}})

	wf := Condition(
		func(ctx context.Context, input string) bool { return strings.Contains(input, "urgent") },
		&SequentialWorkflow{steps: []WorkflowStep{Step("urgent", urgent)}},
		&SequentialWorkflow{steps: []WorkflowStep{Step("normal", normal)}},
	)
	session := NewSession("test")
	resp, _ := wf.Run(context.Background(), session, "this is urgent")
	if resp.Text != "URGENT handled" { t.Errorf("condition true: %q", resp.Text) }

	resp, _ = wf.Run(context.Background(), session, "this is normal")
	if resp.Text != "Normal handled" { t.Errorf("condition false: %q", resp.Text) }
}

func TestVerifyRouterWorkflow(t *testing.T) {
	refund := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "Refund processed"}}}})
	general := New(Config{Model: &mockModel{responses: []ModelResponse{{Text: "General reply"}}}})

	wf := Route(
		func(ctx context.Context, input string) string {
			if strings.Contains(input, "refund") { return "refund" }
			return "general"
		},
		map[string]Workflow{
			"refund":  &SequentialWorkflow{steps: []WorkflowStep{Step("r", refund)}},
			"general": &SequentialWorkflow{steps: []WorkflowStep{Step("g", general)}},
		},
	)
	session := NewSession("test")
	resp, _ := wf.Run(context.Background(), session, "I want a refund")
	if resp.Text != "Refund processed" { t.Errorf("router refund: %q", resp.Text) }

	resp, _ = wf.Run(context.Background(), session, "Hello")
	if resp.Text != "General reply" { t.Errorf("router general: %q", resp.Text) }
}

func TestVerifyLoopWorkflow(t *testing.T) {
	model := &mockModel{}
	for i := 0; i < 5; i++ {
		model.responses = append(model.responses, ModelResponse{Text: fmt.Sprintf("iter %d", i)})
	}
	agent := New(Config{Model: model})
	wf := Loop(agent, func(resp *Response, i int) bool { return i >= 2 }).WithMaxIterations(10)
	session := NewSession("test")
	resp, _ := wf.Run(context.Background(), session, "start")
	if !strings.Contains(resp.Text, "iter 2") { t.Errorf("loop: %q", resp.Text) }
}

func TestVerifyStructuredOutput(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{{Text: `{"city": "Stockholm", "temp": 22}`}}}
	agent := New(Config{Model: model})
	session := NewSession("test")

	type Weather struct {
		City string `json:"city"`
		Temp int    `json:"temp"`
	}
	var w Weather
	err := RunStructured(context.Background(), agent, session, "Weather?", &w)
	if err != nil { t.Fatal(err) }
	if w.City != "Stockholm" || w.Temp != 22 { t.Errorf("structured: %+v", w) }
}

func TestVerifyHumanApproval(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "danger", Arguments: `{"x":"y"}`}}},
	}}
	a := New(Config{Model: model})
	a.ToolWithApproval("danger", "Dangerous", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "executed", nil
	}, "Requires approval")

	s := NewSession("test")
	resp, _ := a.Run(context.Background(), s, "Do danger")
	if !resp.NeedsApproval { t.Error("should need approval") }
	if resp.Approval.ToolName != "danger" { t.Error("wrong tool name") }
	if s.GetStr("_pending_tool") != "danger" { t.Error("pending state not saved") }
}

func TestVerifyRetry(t *testing.T) {
	attempts := 0
	cfg := RetryConfig{MaxRetries: 2, InitialDelay: 0}
	resp, err := retryModelCall(context.Background(), cfg, func() (*ModelResponse, error) {
		attempts++
		if attempts < 3 { return nil, errors.New("fail") }
		return &ModelResponse{Text: "success"}, nil
	})
	if err != nil { t.Fatal(err) }
	if resp.Text != "success" { t.Error("retry didn't succeed") }
	if attempts != 3 { t.Errorf("attempts: %d", attempts) }
}

func TestVerifyHistoryTrimming(t *testing.T) {
	msgs := []Message{{Role: "system", Content: "sys"}}
	for i := 0; i < 100; i++ {
		msgs = append(msgs, Message{Role: "user", Content: fmt.Sprintf("msg%d", i)})
	}
	cfg := HistoryConfig{MaxMessages: 10}
	trimmed := trimHistory(msgs, cfg)
	if len(trimmed) > 12 { t.Errorf("trimmed: %d", len(trimmed)) }
	if trimmed[0].Role != "system" { t.Error("system msg lost") }
}

func TestVerifyDebugMode(t *testing.T) {
	var output []string
	dbg := DebugConfig{Enabled: true, Level: 1, Printer: func(s string) { output = append(output, s) }}
	dbg.printResponse("test")
	if len(output) != 1 { t.Error("debug didn't print") }

	dbg.Enabled = false
	dbg.printResponse("hidden")
	if len(output) != 1 { t.Error("disabled debug printed") }
}

func TestVerifyCancelRun(t *testing.T) {
	ctx, id := RegisterRun(context.Background(), "run-1")
	if ActiveRunCount() != 1 { t.Error("not registered") }
	CancelRun(id)
	if ctx.Err() == nil { t.Error("context not cancelled") }
	if ActiveRunCount() != 0 { t.Error("not unregistered") }
}

func TestVerifySerialization(t *testing.T) {
	a := New(Config{Model: &mockModel{}, Instructions: "You are helpful.", MaxLoops: 5})
	a.Tool("t1", "Tool 1", nil, nil)
	a.Tool("t2", "Tool 2", nil, nil)

	d := a.ToDict()
	if d["instructions"] != "You are helpful." { t.Error("instructions") }
	if d["max_loops"] != 5 { t.Error("max_loops") }

	j, err := a.ToJSON()
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(j), "t1") { t.Error("JSON missing tool") }

	s := a.String()
	if !strings.Contains(s, "t1, t2") { t.Errorf("String: %q", s) }
}

func TestVerifySessionOperations(t *testing.T) {
	s := NewSession("s1")

	// Memory
	s.SetMemory("k", "v")
	if s.GetMemory("k") != "v" { t.Error("memory") }

	// State
	s.Set("step", "verify")
	if s.GetStr("step") != "verify" { t.Error("state") }
	if s.Increment("n") != 1 { t.Error("increment 1") }
	if s.Increment("n") != 2 { t.Error("increment 2") }

	// Metadata
	s.SetMeta("org", "acme")
	if s.GetMeta("org") != "acme" { t.Error("metadata") }

	// History
	s.AddMessage("user", "hi")
	s.AddMessage("assistant", "hello")
	s.AddToolResult("call-1", "result")
	if len(s.History) != 3 { t.Errorf("history: %d", len(s.History)) }
}

func TestVerifyFunctionDefs(t *testing.T) {
	r := NewToolRegistry()
	r.Add("search", "Search", Params{
		"q": {Type: "string", Desc: "Query", Required: true},
		"n": {Type: "number", Desc: "Limit"},
	}, nil)

	defs := r.FunctionDefs()
	if len(defs) != 1 { t.Fatal("defs count") }
	fn := defs[0]["function"].(map[string]any)
	if fn["name"] != "search" { t.Error("name") }
	params := fn["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	if len(props) != 2 { t.Error("props count") }
	req := params["required"].([]string)
	if len(req) != 1 || req[0] != "q" { t.Error("required") }
}
