package agnogo

import (
	"context"
	"testing"
)

func TestToolRegistry(t *testing.T) {
	r := NewToolRegistry()

	r.Add("greet", "Say hello", Params{
		"name": {Type: "string", Desc: "Name", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return "Hello " + args["name"], nil
	})
	r.Add("bye", "Say goodbye", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "Goodbye!", nil
	})

	if r.Count() != 2 {
		t.Errorf("Count() = %d, want 2", r.Count())
	}
	if r.Get("greet") == nil {
		t.Error("Get('greet') = nil")
	}
	if r.Get("missing") != nil {
		t.Error("Get('missing') should be nil")
	}

	result, err := r.Invoke(context.Background(), "greet", map[string]string{"name": "World"})
	if err != nil || result != "Hello World" {
		t.Errorf("Invoke = %q, %v", result, err)
	}

	_, err = r.Invoke(context.Background(), "missing", nil)
	if err == nil {
		t.Error("expected error for missing tool")
	}

	if r.Names() != "greet, bye" {
		t.Errorf("Names() = %q", r.Names())
	}

	defs := r.FunctionDefs()
	if len(defs) != 2 {
		t.Errorf("FunctionDefs() len = %d", len(defs))
	}

	// Replace tool
	r.Add("greet", "Updated", nil, func(ctx context.Context, args map[string]string) (string, error) {
		return "Hi!", nil
	})
	result, _ = r.Invoke(context.Background(), "greet", nil)
	if result != "Hi!" {
		t.Errorf("after replace = %q", result)
	}
	if r.Count() != 2 {
		t.Errorf("Count after replace = %d", r.Count())
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		key   string
		want  string
	}{
		{"string", `{"name":"Erik"}`, "name", "Erik"},
		{"number", `{"count":5}`, "count", "5"},
		{"empty", `{}`, "x", ""},
		{"invalid", `not json`, "x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseArgs(tt.input)
			if got[tt.key] != tt.want {
				t.Errorf("ParseArgs(%q)[%q] = %q, want %q", tt.input, tt.key, got[tt.key], tt.want)
			}
		})
	}
}

func TestSession(t *testing.T) {
	s := NewSession("test-1")

	s.SetMemory("name", "Erik")
	if s.GetMemory("name") != "Erik" {
		t.Error("memory not stored")
	}

	s.Set("step", "verify")
	if s.GetStr("step") != "verify" {
		t.Error("state not stored")
	}

	c := s.Increment("attempts")
	if c != 1 {
		t.Errorf("first increment = %d", c)
	}
	c = s.Increment("attempts")
	if c != 2 {
		t.Errorf("second increment = %d", c)
	}

	s.SetMeta("business_id", "abc-123")
	if s.GetMeta("business_id") != "abc-123" {
		t.Error("metadata not stored")
	}

	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi")
	if len(s.History) != 2 {
		t.Errorf("history len = %d", len(s.History))
	}
}

func TestMemoryStorage(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	s := NewSession("s1")
	s.SetMemory("name", "Test")

	if err := store.Save(ctx, s); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetMemory("name") != "Test" {
		t.Error("memory not persisted")
	}

	_, err = store.Load(ctx, "missing")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestPatternMemory(t *testing.T) {
	m := DefaultPatternMemory()
	s := NewSession("test")

	m.Extract(context.Background(), s, "My name is Erik", "Nice to meet you!")
	if s.GetMemory("name") != "Erik" {
		t.Errorf("name = %q, want Erik", s.GetMemory("name"))
	}

	m.Extract(context.Background(), s, "My email is test@example.com", "Got it!")
	if s.GetMemory("email") != "test@example.com" {
		t.Errorf("email = %q", s.GetMemory("email"))
	}
}

func TestTeamRouting(t *testing.T) {
	team := NewTeam(TeamConfig{
		RouterFunc: func(ctx context.Context, msg string, agents []string) (string, error) {
			if containsFold(msg, "book") {
				return "booking", nil
			}
			return "support", nil
		},
	})

	// We can't fully test Run without a model, but we can test routing
	if team.fallback != "" {
		t.Error("fallback should be empty before agents registered")
	}

	// Verify agent registration
	dummyAgent := New(Config{Model: &mockModel{}})
	team.Agent("booking", dummyAgent)
	team.Agent("support", dummyAgent)

	if team.fallback != "booking" {
		t.Errorf("fallback = %q, want booking", team.fallback)
	}
}

func TestCleanAgentName(t *testing.T) {
	valid := []string{"booking", "support", "complaint"}
	tests := []struct {
		raw  string
		want string
	}{
		{"booking", "booking"},
		{"BOOKING", "booking"},
		{`"booking"`, "booking"},
		{"support agent", "support"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := cleanAgentName(tt.raw, valid)
		if got != tt.want {
			t.Errorf("cleanAgentName(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestLooksLikeQuestion(t *testing.T) {
	yes := []string{"What time?", "how much", "Where is it?", "Vad kostar det?", "Hur lång tid?"}
	no := []string{"Book a haircut", "Cancel please", "OK", "yes"}

	for _, q := range yes {
		if !looksLikeQuestion(q) {
			t.Errorf("expected question: %q", q)
		}
	}
	for _, q := range no {
		if looksLikeQuestion(q) {
			t.Errorf("not a question: %q", q)
		}
	}
}
