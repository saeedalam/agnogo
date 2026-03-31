package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSummarizeSession(t *testing.T) {
	// Session with 20 messages, verify history is compressed
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"text":"Summary of the conversation: user discussed topics A, B, C.","topics":["A","B","C"],"key_facts":["User mentioned A"]}`},
	}}
	a := New(Config{Model: model})

	session := NewSession("sum-1")
	// Add 20 conversation messages (no system)
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		session.History = append(session.History, Message{
			Role:    role,
			Content: fmt.Sprintf("message %d", i),
		})
	}

	err := SummarizeSession(context.Background(), a, session, 10)
	if err != nil {
		t.Fatal(err)
	}

	// Should have: 1 summary + 10 recent = 11 messages
	if len(session.History) != 11 {
		t.Errorf("history len = %d, want 11", len(session.History))
	}

	// First message should be the summary (system role)
	if session.History[0].Role != "system" {
		t.Errorf("first message role = %q, want system", session.History[0].Role)
	}
	if !strings.Contains(session.History[0].Content, "Summary") {
		t.Errorf("summary content = %q", session.History[0].Content)
	}

	// Verify structured summary is stored in session state
	raw := session.Get("_summary")
	if raw == nil {
		t.Fatal("expected _summary in session state")
	}
	summary, ok := raw.(*SessionSummary)
	if !ok {
		t.Fatalf("expected *SessionSummary, got %T", raw)
	}
	if summary.Text == "" {
		t.Error("summary text is empty")
	}
}

func TestSummarizeSessionShort(t *testing.T) {
	// Session with 5 messages, no summarization (below threshold)
	model := &mockModel{responses: []ModelResponse{
		{Text: "should not be called"},
	}}
	a := New(Config{Model: model})

	session := NewSession("sum-short")
	for i := 0; i < 5; i++ {
		session.History = append(session.History, Message{
			Role:    "user",
			Content: fmt.Sprintf("msg %d", i),
		})
	}

	err := SummarizeSession(context.Background(), a, session, 10)
	if err != nil {
		t.Fatal(err)
	}

	// History should be unchanged
	if len(session.History) != 5 {
		t.Errorf("history len = %d, want 5", len(session.History))
	}
	if model.callCount != 0 {
		t.Errorf("model called %d times, want 0", model.callCount)
	}
}

func TestWithSummarizeOption(t *testing.T) {
	// Verify the option sets correct config
	opt := WithSummarize(30, 15)
	sc := &smartConfig{}
	opt.applyOption(sc)

	if sc.summarizeThreshold != 30 {
		t.Errorf("threshold = %d, want 30", sc.summarizeThreshold)
	}
	if sc.summarizeKeepRecent != 15 {
		t.Errorf("keepRecent = %d, want 15", sc.summarizeKeepRecent)
	}

	// Test default keepRecent
	opt2 := WithSummarize(20)
	sc2 := &smartConfig{}
	opt2.applyOption(sc2)

	if sc2.summarizeKeepRecent != 10 {
		t.Errorf("default keepRecent = %d, want 10", sc2.summarizeKeepRecent)
	}
}

func TestSummarizeSessionTopics(t *testing.T) {
	// Verify topics are extracted from the structured summary.
	summaryJSON := `{
		"text": "User discussed booking a flight to Tokyo and preferred window seats.",
		"topics": ["travel", "flights", "tokyo", "preferences"],
		"key_facts": ["User prefers window seats", "Destination: Tokyo", "Travel date: March 2026"]
	}`

	model := &mockModel{responses: []ModelResponse{
		{Text: summaryJSON},
	}}
	a := New(Config{Model: model})

	session := NewSession("sum-topics")
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		session.History = append(session.History, Message{
			Role:    role,
			Content: fmt.Sprintf("message about travel %d", i),
		})
	}

	err := SummarizeSession(context.Background(), a, session, 5)
	if err != nil {
		t.Fatal(err)
	}

	raw := session.Get("_summary")
	if raw == nil {
		t.Fatal("expected _summary in session state")
	}
	summary, ok := raw.(*SessionSummary)
	if !ok {
		t.Fatalf("expected *SessionSummary, got %T", raw)
	}

	// Verify topics
	if len(summary.Topics) != 4 {
		t.Errorf("topics = %v, want 4 items", summary.Topics)
	}
	found := false
	for _, topic := range summary.Topics {
		if topic == "tokyo" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'tokyo' in topics: %v", summary.Topics)
	}

	// Verify key facts
	if len(summary.KeyFacts) != 3 {
		t.Errorf("key_facts = %v, want 3 items", summary.KeyFacts)
	}

	// Verify narrative text
	if !strings.Contains(summary.Text, "Tokyo") {
		t.Errorf("summary text should mention Tokyo: %q", summary.Text)
	}
}

func TestRecallFromSummary(t *testing.T) {
	// First, store a summary in the session.
	session := NewSession("recall-test")
	session.Set("_summary", &SessionSummary{
		Text:     "User booked a flight to Tokyo for March 15. They prefer window seats and vegetarian meals.",
		Topics:   []string{"travel", "flights", "tokyo", "preferences"},
		KeyFacts: []string{"Flight to Tokyo", "Date: March 15", "Window seat preference", "Vegetarian meals"},
	})

	// Model responds to recall query based on summary.
	model := &mockModel{responses: []ModelResponse{
		{Text: "The user prefers window seats and vegetarian meals."},
	}}
	a := New(Config{Model: model})

	result, err := RecallFromSummary(context.Background(), a, session, "seat preference")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty recall result")
	}
	if !strings.Contains(result, "window") {
		t.Errorf("recall should mention window seats: %q", result)
	}

	// Verify the model was called with the right prompt
	if model.callCount != 1 {
		t.Errorf("model called %d times, want 1", model.callCount)
	}
}

func TestRecallFromSummaryEmpty(t *testing.T) {
	// No summary stored -- should return empty string, no error.
	session := NewSession("recall-empty")
	model := &mockModel{responses: []ModelResponse{
		{Text: "should not be called"},
	}}
	a := New(Config{Model: model})

	result, err := RecallFromSummary(context.Background(), a, session, "anything")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
	if model.callCount != 0 {
		t.Error("model should not be called when no summary exists")
	}
}

func TestRecallFromSummaryMapType(t *testing.T) {
	// Simulate a summary that was deserialized from JSON storage (map[string]any).
	session := NewSession("recall-map")
	session.Set("_summary", map[string]any{
		"text":      "User likes pizza and lives in Stockholm.",
		"topics":    []any{"food", "location"},
		"key_facts": []any{"Likes pizza", "Lives in Stockholm"},
	})

	model := &mockModel{responses: []ModelResponse{
		{Text: "The user lives in Stockholm."},
	}}
	a := New(Config{Model: model})

	result, err := RecallFromSummary(context.Background(), a, session, "where does the user live")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Stockholm") {
		t.Errorf("recall should mention Stockholm: %q", result)
	}
}

func TestParseSummaryResponseFallback(t *testing.T) {
	// Non-JSON response should fall back to narrative-only summary.
	summary := parseSummaryResponse("Just a plain text summary with no JSON.")
	if summary.Text != "Just a plain text summary with no JSON." {
		t.Errorf("text = %q", summary.Text)
	}
	if len(summary.Topics) != 0 {
		t.Errorf("topics should be empty for non-JSON response: %v", summary.Topics)
	}
}

func TestParseSummaryResponseMarkdownFences(t *testing.T) {
	// JSON wrapped in markdown code fences.
	input := "```json\n{\"text\":\"summary\",\"topics\":[\"a\"],\"key_facts\":[\"b\"]}\n```"
	summary := parseSummaryResponse(input)
	if summary.Text != "summary" {
		t.Errorf("text = %q, want 'summary'", summary.Text)
	}
	if len(summary.Topics) != 1 || summary.Topics[0] != "a" {
		t.Errorf("topics = %v", summary.Topics)
	}
}

func TestSessionSummaryJSON(t *testing.T) {
	// Verify SessionSummary serializes/deserializes correctly.
	s := &SessionSummary{
		Text:     "A conversation about weather.",
		Topics:   []string{"weather", "forecast"},
		KeyFacts: []string{"User is in Stockholm", "Prefers Celsius"},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var s2 SessionSummary
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatal(err)
	}

	if s2.Text != s.Text {
		t.Errorf("text = %q", s2.Text)
	}
	if len(s2.Topics) != 2 {
		t.Errorf("topics len = %d", len(s2.Topics))
	}
	if len(s2.KeyFacts) != 2 {
		t.Errorf("key_facts len = %d", len(s2.KeyFacts))
	}
}
