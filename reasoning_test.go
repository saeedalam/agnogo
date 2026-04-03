package agnogo

import (
	"context"
	"strings"
	"testing"
)

// ── Extract Thinking ────────────────────────────────────────────────

func TestExtractThinkingWithThinkTags(t *testing.T) {
	text := "<think>step by step analysis</think>The answer is 42."
	thinking, answer := extractThinking(text)
	if thinking != "step by step analysis" {
		t.Errorf("thinking = %q", thinking)
	}
	if answer != "The answer is 42." {
		t.Errorf("answer = %q", answer)
	}
}

func TestExtractThinkingWithThinkingTags(t *testing.T) {
	text := "<thinking>deep thought</thinking>Final result."
	thinking, answer := extractThinking(text)
	if thinking != "deep thought" {
		t.Errorf("thinking = %q", thinking)
	}
	if answer != "Final result." {
		t.Errorf("answer = %q", answer)
	}
}

func TestExtractThinkingNoTags(t *testing.T) {
	text := "Just a plain answer."
	thinking, answer := extractThinking(text)
	if thinking != "" {
		t.Errorf("thinking should be empty, got %q", thinking)
	}
	if answer != "Just a plain answer." {
		t.Errorf("answer = %q", answer)
	}
}

func TestExtractThinkingEmptyAnswer(t *testing.T) {
	text := "Before thinking<think>the reasoning</think>"
	thinking, answer := extractThinking(text)
	if thinking != "the reasoning" {
		t.Errorf("thinking = %q", thinking)
	}
	if answer != "Before thinking" {
		t.Errorf("answer = %q, want %q", answer, "Before thinking")
	}
}

// ── CoT Reasoning ───────────────────────────────────────────────────

func TestCoTReasoningBasic(t *testing.T) {
	// Mock model returns structured reasoning steps
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"title":"Analyze","action":"I will analyze","result":"Problem understood","reasoning":"Need to understand first","next_action":"continue","confidence":0.8}`},
		{Text: `{"title":"Validate","action":"I will verify","result":"Verified correct","reasoning":"Cross-checking","next_action":"validate","confidence":0.9}`},
		{Text: `{"title":"Answer","action":"I will answer","result":"The answer is 42","reasoning":"Final step","next_action":"final_answer","confidence":0.95}`},
	}}

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 2, MaxSteps: 6}
	steps, ctx_str := runReasoning(context.Background(), cfg, model, "What is 6*7?", nil)

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].Title != "Analyze" {
		t.Errorf("step 0 title = %q", steps[0].Title)
	}
	if steps[1].NextAction != NextValidate {
		t.Errorf("step 1 next_action = %q, want validate", steps[1].NextAction)
	}
	if steps[2].NextAction != NextFinalAnswer {
		t.Errorf("step 2 next_action = %q, want final_answer", steps[2].NextAction)
	}
	if ctx_str == "" {
		t.Error("context string should not be empty")
	}
	if !strings.Contains(ctx_str, "Analyze") {
		t.Error("context should contain step titles")
	}
}

func TestCoTReasoningMinSteps(t *testing.T) {
	// Model tries to finish in 1 step, but minSteps=3
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"title":"Quick","result":"Done fast","next_action":"final_answer","confidence":0.9}`},
		{Text: `{"title":"More","result":"Extra analysis","next_action":"continue","confidence":0.85}`},
		{Text: `{"title":"Final","result":"Real answer","next_action":"final_answer","confidence":0.95}`},
	}}

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 3, MaxSteps: 6}
	steps, _ := runReasoning(context.Background(), cfg, model, "test", nil)

	if len(steps) < 3 {
		t.Errorf("expected at least 3 steps (minSteps), got %d", len(steps))
	}
}

func TestCoTReasoningMaxSteps(t *testing.T) {
	// Model never says final_answer — should stop at maxSteps
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"title":"Step 1","result":"continuing","next_action":"continue","confidence":0.5}`},
		{Text: `{"title":"Step 2","result":"still going","next_action":"continue","confidence":0.5}`},
		{Text: `{"title":"Step 3","result":"more","next_action":"continue","confidence":0.5}`},
	}}

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 1, MaxSteps: 3}
	steps, _ := runReasoning(context.Background(), cfg, model, "test", nil)

	if len(steps) != 3 {
		t.Errorf("expected 3 steps (maxSteps), got %d", len(steps))
	}
}

func TestCoTReasoningFallbackRawText(t *testing.T) {
	// Model returns non-JSON — should fallback to raw reasoning
	model := &mockModel{responses: []ModelResponse{
		{Text: "Let me think about this... The answer is 42."},
		{Text: `{"title":"Done","result":"42","next_action":"final_answer","confidence":0.9}`},
	}}

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 1, MaxSteps: 3}
	steps, _ := runReasoning(context.Background(), cfg, model, "test", nil)

	if len(steps) < 1 {
		t.Fatal("expected at least 1 step")
	}
	// First step should be raw text fallback
	if steps[0].Title != "Step 1" {
		t.Errorf("fallback title = %q, want 'Step 1'", steps[0].Title)
	}
	if !strings.Contains(steps[0].Reasoning, "think about this") {
		t.Error("fallback should contain raw text as reasoning")
	}
}

func TestCoTReasoningLegacyDONE(t *testing.T) {
	// Legacy "DONE" next_step should be normalized to NextFinalAnswer
	model := &mockModel{responses: []ModelResponse{
		{Text: `{"title":"Step 1","result":"ok","next_action":"continue","confidence":0.8}`},
		{Text: `{"title":"Done","result":"finished","next_action":"DONE","confidence":0.9}`},
	}}

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 1, MaxSteps: 5}
	steps, _ := runReasoning(context.Background(), cfg, model, "test", nil)

	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[1].NextAction != NextFinalAnswer {
		t.Errorf("DONE should be normalized to final_answer, got %q", steps[1].NextAction)
	}
}

// ── Native Reasoning ────────────────────────────────────────────────

type mockNativeReasoner struct {
	response string
}

func (m *mockNativeReasoner) ChatCompletion(_ context.Context, _ []Message, _ []map[string]any) (*ModelResponse, error) {
	return &ModelResponse{Text: m.response}, nil
}

func (m *mockNativeReasoner) Reason(_ context.Context, _ []Message) (*ModelResponse, error) {
	return &ModelResponse{Text: m.response}, nil
}

func TestNativeReasoningDetection(t *testing.T) {
	native := &mockNativeReasoner{response: "<think>deep thought</think>Answer."}
	if !isNativeReasoningModel(native) {
		t.Error("should detect NativeReasoner interface")
	}

	regular := &mockModel{responses: []ModelResponse{{Text: "hi"}}}
	if isNativeReasoningModel(regular) {
		t.Error("mockModel should NOT be detected as native reasoner")
	}
}

func TestNativeReasoningExecution(t *testing.T) {
	native := &mockNativeReasoner{response: "<think>I analyzed the problem carefully</think>The answer is 42."}

	cfg := &ReasoningConfig{Enabled: true, Mode: ReasoningAuto}
	steps, ctx_str := runReasoning(context.Background(), cfg, native, "What is 6*7?", nil)

	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Reasoning != "I analyzed the problem carefully" {
		t.Errorf("reasoning = %q", steps[0].Reasoning)
	}
	if !strings.Contains(ctx_str, "analyzed the problem") {
		t.Error("context should contain thinking content")
	}
}

func TestReasoningModeCoTForced(t *testing.T) {
	// Force CoT mode even for native reasoner
	native := &mockNativeReasoner{response: `{"title":"CoT Step","result":"forced CoT","next_action":"final_answer","confidence":0.9}`}

	cfg := &ReasoningConfig{Enabled: true, Mode: ReasoningCoT, MinSteps: 1, MaxSteps: 2}
	steps, _ := runReasoning(context.Background(), cfg, native, "test", nil)

	if len(steps) == 0 {
		t.Fatal("expected CoT steps")
	}
	// Should use CoT path (structured JSON), not native path (<think> tags)
	if steps[0].Title != "CoT Step" {
		t.Errorf("title = %q, expected CoT result", steps[0].Title)
	}
}

// ── Response Integration ────────────────────────────────────────────

func TestReasoningStepsInResponse(t *testing.T) {
	model := &mockModel{responses: []ModelResponse{
		// Reasoning step
		{Text: `{"title":"Think","result":"thought","next_action":"final_answer","confidence":0.9}`},
		// Main response
		{Text: "The answer is 42."},
	}}

	agent := New(Config{
		Model:     model,
		Reasoning: &ReasoningConfig{Enabled: true, MinSteps: 1, MaxSteps: 2},
	})

	session := NewSession("reasoning-test")
	resp, err := agent.Run(context.Background(), session, "What is 6*7?")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "The answer is 42." {
		t.Errorf("text = %q", resp.Text)
	}
	if len(resp.ReasoningSteps) == 0 {
		t.Error("expected reasoning steps in response")
	}
}

// ── Multi-Turn Context ──────────────────────────────────────────────

func TestCoTReasoningIncludesSessionHistory(t *testing.T) {
	var capturedMessages []Message

	// Mock model that captures messages it receives
	model := &mockModelCapture{
		capturedMessages: &capturedMessages,
		response:         `{"title":"Done","result":"answer","next_action":"final_answer","confidence":0.9}`,
	}

	session := NewSession("multi-turn")
	session.AddMessage("user", "What is Go?")
	session.AddMessage("assistant", "Go is a programming language.")

	cfg := &ReasoningConfig{Enabled: true, MinSteps: 1, MaxSteps: 2}
	runCoTReasoning(context.Background(), cfg, model, "Tell me more about Go", session)

	// Messages should include session history
	if len(capturedMessages) < 4 {
		t.Fatalf("expected >= 4 messages (system + history + user), got %d", len(capturedMessages))
	}
	// First should be system prompt
	if capturedMessages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", capturedMessages[0].Role)
	}
	// History should be present
	foundHistory := false
	for _, m := range capturedMessages {
		if m.Content == "Go is a programming language." {
			foundHistory = true
			break
		}
	}
	if !foundHistory {
		t.Error("session history not included in reasoning messages")
	}
}

type mockModelCapture struct {
	capturedMessages *[]Message
	response         string
}

func (m *mockModelCapture) ChatCompletion(_ context.Context, messages []Message, _ []map[string]any) (*ModelResponse, error) {
	*m.capturedMessages = make([]Message, len(messages))
	copy(*m.capturedMessages, messages)
	return &ModelResponse{Text: m.response}, nil
}

// ── Extract Thinking Edge Cases ─────────────────────────────────────

func TestExtractThinkingPreferLongerTag(t *testing.T) {
	// Should match <thinking> not <think> when both tag patterns are in text
	text := "<thinking>deep analysis</thinking>The result."
	thinking, answer := extractThinking(text)
	if thinking != "deep analysis" {
		t.Errorf("thinking = %q", thinking)
	}
	if answer != "The result." {
		t.Errorf("answer = %q", answer)
	}
}

// ── Disabled Reasoning ──────────────────────────────────────────────

func TestReasoningDisabled(t *testing.T) {
	cfg := &ReasoningConfig{Enabled: false}
	steps, ctx_str := runReasoning(context.Background(), cfg, nil, "test", nil)
	if steps != nil {
		t.Error("disabled reasoning should return nil steps")
	}
	if ctx_str != "" {
		t.Error("disabled reasoning should return empty context")
	}
}

func TestReasoningNilConfig(t *testing.T) {
	steps, ctx_str := runReasoning(context.Background(), nil, nil, "test", nil)
	if steps != nil || ctx_str != "" {
		t.Error("nil config should return empty results")
	}
}
