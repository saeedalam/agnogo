package agnogo

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSummarizeSession(t *testing.T) {
	// Session with 20 messages, verify history is compressed
	model := &mockModel{responses: []ModelResponse{
		{Text: "Summary of the conversation: user discussed topics A, B, C."},
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
