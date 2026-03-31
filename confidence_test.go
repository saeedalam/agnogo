package agnogo

import (
	"testing"
)

func TestScoreConfidenceToolBacked(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What is the weather?")

	resp := &Response{
		Text:        "The weather in NYC is 72F and sunny.",
		ToolsCalled: []string{"get_weather"},
	}

	score := ScoreConfidence(resp, session, 1)

	if !score.ToolBacked {
		t.Error("expected ToolBacked=true")
	}
	// Base 0.5 + tool 0.3 + short 0.05 = 0.85
	if score.Score < 0.8 {
		t.Errorf("expected high score for tool-backed response, got %.2f", score.Score)
	}
	if len(score.Sources) != 1 || score.Sources[0] != "get_weather" {
		t.Errorf("unexpected sources: %v", score.Sources)
	}
}

func TestScoreConfidenceMultipleTools(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "Compare prices")

	resp := &Response{
		Text:        "Product A costs $10, Product B costs $15.",
		ToolsCalled: []string{"search_a", "search_b"},
	}

	score := ScoreConfidence(resp, session, 2)

	// Base 0.5 + tool 0.3 + multi 0.1 + short 0.05 = 0.95
	if score.Score < 0.9 {
		t.Errorf("expected very high score for multi-tool response, got %.2f", score.Score)
	}
}

func TestScoreConfidenceNoTools(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What is the current temperature?")

	resp := &Response{
		Text:        "The temperature is about 70 degrees.",
		ToolsCalled: nil,
	}

	score := ScoreConfidence(resp, session, 0)

	if score.ToolBacked {
		t.Error("expected ToolBacked=false")
	}
	// Base 0.5 - factual 0.3 + short 0.05 = 0.25
	if score.Score > 0.3 {
		t.Errorf("expected low score for factual question without tools, got %.2f", score.Score)
	}
}

func TestScoreConfidenceHedging(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "Tell me about Go.")

	resp := &Response{
		Text:        "I think Go is probably a good language, but I'm not sure about all its features.",
		ToolsCalled: nil,
	}

	score := ScoreConfidence(resp, session, 0)

	// Base 0.5 - hedging (i think -0.1, probably -0.1, not sure -0.1 -> capped at -0.2) + short 0.05 = 0.35
	if score.Score > 0.4 {
		t.Errorf("expected lower score due to hedging, got %.2f", score.Score)
	}

	// Check that reasons mention hedging
	hasHedge := false
	for _, r := range score.Reasons {
		if len(r) > 0 && r[:1] == "h" {
			hasHedge = true
			break
		}
	}
	if !hasHedge {
		t.Errorf("expected hedging reasons, got %v", score.Reasons)
	}
}

func TestScoreConfidenceClamped(t *testing.T) {
	// Test score never goes below 0
	session := NewSession("test")
	session.AddMessage("user", "What is the current price? How much does it cost?")

	resp := &Response{
		Text:        "I think it probably costs something, but I'm not sure. I believe it might be expensive.",
		ToolsCalled: nil,
	}

	score := ScoreConfidence(resp, session, 0)

	if score.Score < 0.0 {
		t.Errorf("score should not be below 0, got %.2f", score.Score)
	}
	if score.Score > 1.0 {
		t.Errorf("score should not be above 1, got %.2f", score.Score)
	}

	// Test score never goes above 1
	session2 := NewSession("test2")
	session2.AddMessage("user", "Do something")

	resp2 := &Response{
		Text:        "Done.",
		ToolsCalled: []string{"a", "b", "c"},
	}

	score2 := ScoreConfidence(resp2, session2, 3)
	if score2.Score > 1.0 {
		t.Errorf("score should not exceed 1.0, got %.2f", score2.Score)
	}
	if score2.Score < 0.0 {
		t.Errorf("score should not be below 0.0, got %.2f", score2.Score)
	}
}
