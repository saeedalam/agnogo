package agnogo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// ── LearningMachine ─────────────────────────────────────────────────

func TestLearningMachineBuildContextEmpty(t *testing.T) {
	lm := NewLearningMachine(nil)
	ctx := lm.BuildContext(context.Background(), NewSession("test"))
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}
}

func TestLearningMachineBuildContextWithProfile(t *testing.T) {
	session := NewSession("test")
	profile := UserProfile{Name: "Alice", Company: "Acme"}
	data, _ := json.Marshal(profile)
	session.Set("_learn_user_profile", string(data))

	lm := NewLearningMachine(nil)
	lm.AddStore(NewUserProfileStore())

	ctx := lm.BuildContext(context.Background(), session)
	if !strings.Contains(ctx, "Alice") {
		t.Error("context should contain user name")
	}
	if !strings.Contains(ctx, "Acme") {
		t.Error("context should contain company")
	}
	if !strings.Contains(ctx, "LEARNED CONTEXT") {
		t.Error("context should have header")
	}
}

// ── UserProfile Store ───────────────────────────────────────────────

func TestUserProfileRecallEmpty(t *testing.T) {
	store := NewUserProfileStore()
	result := store.Recall(context.Background(), NewSession("empty"))
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestUserProfileRecallWithData(t *testing.T) {
	session := NewSession("test")
	profile := UserProfile{
		Name:     "Bob",
		Email:    "bob@example.com",
		Location: "Stockholm",
		Preferences: map[string]string{
			"theme": "dark",
		},
	}
	data, _ := json.Marshal(profile)
	session.Set("_learn_user_profile", string(data))

	store := NewUserProfileStore()
	result := store.Recall(context.Background(), session)

	if !strings.Contains(result, "Bob") {
		t.Error("should contain name")
	}
	if !strings.Contains(result, "bob@example.com") {
		t.Error("should contain email")
	}
	if !strings.Contains(result, "Stockholm") {
		t.Error("should contain location")
	}
	if !strings.Contains(result, "dark") {
		t.Error("should contain preference")
	}
}

func TestUserProfileProcess(t *testing.T) {
	// Mock model returns extracted profile JSON
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"name": "Charlie", "company": "TechCo", "role": "Engineer"}`},
	}}

	session := NewSession("test")
	store := NewUserProfileStore()

	messages := []Message{
		{Role: "user", Content: "Hi, I'm Charlie from TechCo, I work as an Engineer."},
		{Role: "assistant", Content: "Nice to meet you Charlie!"},
	}

	store.Process(context.Background(), model, session, messages)

	// Verify profile was saved
	raw := session.GetStr("_learn_user_profile")
	if raw == "" {
		t.Fatal("profile should be saved")
	}

	var profile UserProfile
	json.Unmarshal([]byte(raw), &profile)
	if profile.Name != "Charlie" {
		t.Errorf("name = %q", profile.Name)
	}
	if profile.Company != "TechCo" {
		t.Errorf("company = %q", profile.Company)
	}
}

func TestUserProfileMerge(t *testing.T) {
	session := NewSession("test")

	// First extraction
	profile1 := UserProfile{Name: "Alice", Email: "alice@example.com"}
	data1, _ := json.Marshal(profile1)
	session.Set("_learn_user_profile", string(data1))

	// Second extraction adds company
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"company": "BigCorp"}`},
	}}

	store := NewUserProfileStore()
	store.Process(context.Background(), model, session, []Message{
		{Role: "user", Content: "I work at BigCorp"},
		{Role: "assistant", Content: "Got it!"},
	})

	// Verify merge: name + email preserved, company added
	var profile UserProfile
	json.Unmarshal([]byte(session.GetStr("_learn_user_profile")), &profile)
	if profile.Name != "Alice" {
		t.Errorf("name should be preserved, got %q", profile.Name)
	}
	if profile.Email != "alice@example.com" {
		t.Errorf("email should be preserved, got %q", profile.Email)
	}
	if profile.Company != "BigCorp" {
		t.Errorf("company should be added, got %q", profile.Company)
	}
}

// ── SessionContext Store ────────────────────────────────────────────

func TestSessionContextRecallEmpty(t *testing.T) {
	store := NewSessionContextStore()
	result := store.Recall(context.Background(), NewSession("empty"))
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSessionContextProcess(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"summary": "Discussed project timeline", "decisions": "Ship by Friday", "topics": "project, deadline"}`},
	}}

	session := NewSession("test")
	store := NewSessionContextStore()

	messages := []Message{
		{Role: "user", Content: "When should we ship?"},
		{Role: "assistant", Content: "Let's target Friday."},
		{Role: "user", Content: "Agreed, Friday it is."},
	}

	store.Process(context.Background(), model, session, messages)

	raw := session.GetStr("_learn_session_context")
	if raw == "" {
		t.Fatal("session context should be saved")
	}

	result := store.Recall(context.Background(), session)
	if !strings.Contains(result, "project timeline") {
		t.Error("should contain summary")
	}
	if !strings.Contains(result, "Friday") {
		t.Error("should contain decision")
	}
}

func TestSessionContextSkipsShortConversation(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"summary": "should not be called"}`},
	}}

	session := NewSession("test")
	store := NewSessionContextStore()

	// Only 2 messages — too short
	store.Process(context.Background(), model, session, []Message{
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello!"},
	})

	raw := session.GetStr("_learn_session_context")
	if raw != "" {
		t.Error("should not process short conversations")
	}
}

// ── EntityMemory Store ──────────────────────────────────────────────

func TestEntityMemoryRecallEmpty(t *testing.T) {
	store := NewEntityMemoryStore()
	result := store.Recall(context.Background(), NewSession("empty"))
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestEntityMemoryProcess(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		{Text: `[{"entity_id": "acme", "entity_type": "company", "facts": ["Founded in 2020", "Based in Stockholm"], "events": ["Raised Series A in 2023"]}]`},
	}}

	session := NewSession("test")
	store := NewEntityMemoryStore()

	messages := []Message{
		{Role: "user", Content: "Tell me about Acme Corp."},
		{Role: "assistant", Content: "Acme was founded in 2020 in Stockholm. They raised Series A in 2023."},
	}

	store.Process(context.Background(), model, session, messages)

	result := store.Recall(context.Background(), session)
	if !strings.Contains(result, "acme") {
		t.Error("should contain entity name")
	}
	if !strings.Contains(result, "Stockholm") {
		t.Error("should contain fact")
	}
	if !strings.Contains(result, "Series A") {
		t.Error("should contain event")
	}
}

func TestEntityMemoryMerge(t *testing.T) {
	session := NewSession("test")

	// Existing entity
	existing := []EntityMemory{{
		EntityID:   "acme",
		EntityType: "company",
		Facts:      []string{"Founded in 2020"},
		Events:     []string{"Raised Series A"},
	}}
	data, _ := json.Marshal(existing)
	session.Set("_learn_entities", string(data))

	// New extraction adds more facts
	model := &mockModel{responses: []ModelResponse{
		{Text: `[{"entity_id": "acme", "entity_type": "company", "facts": ["Has 50 employees", "Founded in 2020"], "events": []}]`},
	}}

	store := NewEntityMemoryStore()
	store.Process(context.Background(), model, session, []Message{
		{Role: "user", Content: "How big is Acme?"},
		{Role: "assistant", Content: "They have about 50 employees."},
	})

	var entities []EntityMemory
	json.Unmarshal([]byte(session.GetStr("_learn_entities")), &entities)

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	// Should have deduplicated facts
	if len(entities[0].Facts) != 2 {
		t.Errorf("expected 2 unique facts, got %d: %v", len(entities[0].Facts), entities[0].Facts)
	}
	// Original event should be preserved
	if len(entities[0].Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(entities[0].Events))
	}
}

// ── Dedup Helper ────────────────────────────────────────────────────

func TestDedup(t *testing.T) {
	items := []string{"a", "b", "a", "c", "b"}
	result := dedup(items)
	if len(result) != 3 {
		t.Errorf("expected 3 unique items, got %d", len(result))
	}
}

func TestDedupEmpty(t *testing.T) {
	result := dedup(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

// ── Agent Integration ───────────────────────────────────────────────

func TestLearningIntegrationContextInjection(t *testing.T) {
	// Pre-populate profile
	session := NewSession("learn-test")
	profile := UserProfile{Name: "Diana", Language: "Swedish"}
	data, _ := json.Marshal(profile)
	session.Set("_learn_user_profile", string(data))

	// Model returns a response
	model := &mockModel{responses: []ModelResponse{
		{Text: "Hej Diana! Hur kan jag hjälpa dig?"},
	}}

	lm := NewLearningMachine(model)
	lm.AddStore(NewUserProfileStore())

	agent := New(Config{Model: model})
	agent.learning = lm

	resp, err := agent.Run(context.Background(), session, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text == "" {
		t.Error("expected non-empty response")
	}
}

// ── Format Helper ───────────────────────────────────────────────────

func TestFormatMessagesForExtraction(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hi there"},
		{Role: "assistant", Content: "Hello!"},
	}

	result := formatMessagesForExtraction(messages)
	if strings.Contains(result, "system") {
		t.Error("should skip system messages")
	}
	if !strings.Contains(result, "[user]: Hi there") {
		t.Error("should contain user message")
	}
	if !strings.Contains(result, "[assistant]: Hello!") {
		t.Error("should contain assistant message")
	}
}
