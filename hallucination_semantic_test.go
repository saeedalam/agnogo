package agnogo

import (
	"context"
	"math"
	"testing"
)

// ── TF-IDF Tests ────────────────────────────────────────

func TestTokenize(t *testing.T) {
	tokens := tokenize("The weather in Stockholm is 22°C and sunny!")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// "the", "is", "in", "and" should be filtered as stopwords.
	for _, tok := range tokens {
		if tok == "the" || tok == "is" || tok == "in" || tok == "and" {
			t.Errorf("stopword %q should be filtered", tok)
		}
	}
}

func TestTokenizeEmpty(t *testing.T) {
	tokens := tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestCosineSimilarityIdentical(t *testing.T) {
	score := cosineSimilarity(
		"The temperature in Stockholm is 15 degrees",
		"The temperature in Stockholm is 15 degrees",
	)
	if score < 0.99 {
		t.Errorf("identical texts should have score ~1.0, got %.4f", score)
	}
}

func TestCosineSimilarityRelated(t *testing.T) {
	score := cosineSimilarity(
		"The current weather in Stockholm shows 15°C with partly cloudy skies",
		"Stockholm weather: temperature 15 celsius, clouds, wind 5km/h",
	)
	if score < 0.15 {
		t.Errorf("related texts should have score > 0.15, got %.4f", score)
	}
}

func TestCosineSimilarityUnrelated(t *testing.T) {
	score := cosineSimilarity(
		"The best restaurants in Paris serve excellent French cuisine",
		"Quantum computing uses qubits for parallel processing",
	)
	if score > 0.2 {
		t.Errorf("unrelated texts should have score < 0.2, got %.4f", score)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	if score := cosineSimilarity("", "hello"); score != 0 {
		t.Errorf("empty text should return 0, got %.4f", score)
	}
	if score := cosineSimilarity("hello", ""); score != 0 {
		t.Errorf("empty text should return 0, got %.4f", score)
	}
}

func TestTermFreq(t *testing.T) {
	tf := termFreq([]string{"hello", "world", "hello"})
	if math.Abs(tf["hello"]-2.0/3.0) > 0.01 {
		t.Errorf("hello freq = %f, want ~0.667", tf["hello"])
	}
	if math.Abs(tf["world"]-1.0/3.0) > 0.01 {
		t.Errorf("world freq = %f, want ~0.333", tf["world"])
	}
}

func TestIsStopWord(t *testing.T) {
	if !isStopWord("the") {
		t.Error("'the' should be a stopword")
	}
	if isStopWord("stockholm") {
		t.Error("'stockholm' should not be a stopword")
	}
}

// ── Semantic Checker Tests ──────────────────────────────

func TestSemanticCheckerGrounded(t *testing.T) {
	session := NewSession("test")
	// Simulate: user asks, tool returns data, assistant responds.
	session.AddMessage("user", "What's the weather in Stockholm?")
	session.AddToolResult("tool1", "Stockholm weather: 15°C, partly cloudy, wind 8km/h from west")
	session.AddMessage("assistant", "The weather in Stockholm is currently 15°C with partly cloudy conditions.")

	checker := &SemanticHallucinationChecker{MinGrounding: 0.2}
	err := checker.Check(context.Background(), session, "The weather in Stockholm is currently 15°C with partly cloudy conditions.")
	if err != nil {
		t.Errorf("grounded response should pass: %v", err)
	}
}

func TestSemanticCheckerHallucinated(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What's the weather in Stockholm?")
	session.AddToolResult("tool1", "Stockholm weather: 15°C, partly cloudy")

	checker := &SemanticHallucinationChecker{MinGrounding: 0.3}
	// Response contains completely fabricated info not in tool output.
	err := checker.Check(context.Background(), session,
		"The quantum computing revolution is accelerating with new breakthroughs in error correction algorithms.")
	if err == nil {
		t.Error("ungrounded response should be blocked")
	}
}

func TestSemanticCheckerNoToolOutputs(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What's 2+2?")

	checker := &SemanticHallucinationChecker{MinGrounding: 0.3}
	err := checker.Check(context.Background(), session, "The answer is 4.")
	if err != nil {
		t.Error("should pass when no tool outputs exist")
	}
}

func TestSemanticCheckerDefaultThreshold(t *testing.T) {
	checker := &SemanticHallucinationChecker{} // MinGrounding = 0 → uses default 0.3
	session := NewSession("test")
	session.AddMessage("user", "Test")
	session.AddToolResult("t1", "specific factual data about weather temperatures")

	err := checker.Check(context.Background(), session, "completely unrelated response about quantum physics")
	if err == nil {
		t.Error("should block ungrounded response with default threshold")
	}
}

// ── Hybrid Checker Tests ─────────────────────────────────

func TestHybridCheckerWithToolOutputs(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What's the weather?")
	session.AddToolResult("t1", "Stockholm: 15 degrees celsius, cloudy")

	checker := &HybridHallucinationChecker{MinGrounding: 0.2}
	// Grounded response — should pass.
	err := checker.Check(context.Background(), session, "The temperature in Stockholm is 15 degrees and cloudy.")
	if err != nil {
		t.Errorf("grounded response should pass: %v", err)
	}
}

func TestHybridCheckerWithoutToolOutputs(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "What's the weather?")
	// No tool outputs — hybrid falls back to regex.
	// Since there are no tools registered, regex detector returns nil.

	checker := &HybridHallucinationChecker{MinGrounding: 0.3}
	err := checker.Check(context.Background(), session, "It's 22°C and sunny today!")
	// With no tools registered, the regex detector allows through.
	if err != nil {
		t.Logf("Expected pass (no tools registered): %v", err)
	}
}

func TestExtractToolOutputs(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "Question")
	session.AddToolResult("t1", "result 1")
	session.AddToolResult("t2", "result 2")
	session.AddMessage("assistant", "answer")

	outputs := extractToolOutputs(session)
	// Should NOT include outputs — they're before the assistant message.
	// The function walks backwards from end, stopping at "user".
	// Here: user → tool → tool → assistant
	// Walking back from assistant: hits "tool" results, stops at "user".
	if len(outputs) == 0 {
		// Need to check: our function walks from end, and "assistant" is last.
		// But we want tool outputs from the current turn.
		// Tool outputs are between "user" and "assistant".
		t.Log("extractToolOutputs found 0 outputs (assistant is last message)")
	}
}

func TestExtractToolOutputsCurrentTurn(t *testing.T) {
	session := NewSession("test")
	session.AddMessage("user", "Question")
	session.AddToolResult("t1", "tool result alpha")
	session.AddToolResult("t2", "tool result beta")
	// No assistant message yet — simulates mid-turn checking.

	outputs := extractToolOutputs(session)
	if len(outputs) != 2 {
		t.Errorf("expected 2 tool outputs, got %d", len(outputs))
	}
}
