package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ── Reasoning Configuration ──────────────────────────────────────────
//
// Three reasoning modes:
//
//  1. Default chain-of-thought: generic multi-step prompt works with any model.
//  2. Native reasoning: O1/O3/O4, DeepSeek-R1, Claude extended thinking —
//     the model handles reasoning internally, we extract the output.
//  3. Dedicated model: use a separate, stronger model for reasoning only.
//
// Usage:
//
//	agent := agnogo.Agent("...", agnogo.Reasoning)                        // default CoT
//	agent := agnogo.Agent("...", agnogo.WithReasoning(agnogo.ReasoningConfig{
//	    Enabled: true,
//	    Mode:    agnogo.ReasoningNative,  // for O1/O3/Claude thinking
//	}))

// ReasoningConfig enables chain-of-thought reasoning before the agent responds.
type ReasoningConfig struct {
	Enabled  bool
	Model    ModelProvider // separate reasoning model (optional, uses agent model if nil)
	Mode     ReasoningMode // how to reason (default: auto-detect)
	MinSteps int           // minimum steps for CoT mode (default 2)
	MaxSteps int           // maximum steps for CoT mode (default 6)
}

// ReasoningMode controls how reasoning is performed.
type ReasoningMode int

const (
	// ReasoningAuto detects native reasoning models and uses their built-in
	// thinking. Falls back to default chain-of-thought for other models.
	ReasoningAuto ReasoningMode = iota

	// ReasoningCoT forces the default chain-of-thought prompt, even for
	// models that support native reasoning.
	ReasoningCoT

	// ReasoningNative forces native reasoning mode. Only works with models
	// that support it (O1/O3, Claude thinking, DeepSeek-R1). Falls back to
	// CoT if the model doesn't support native reasoning.
	ReasoningNative
)

// NextAction controls the flow between reasoning steps.
type NextAction string

const (
	NextContinue    NextAction = "continue"     // proceed to next step
	NextValidate    NextAction = "validate"     // cross-verify the solution
	NextFinalAnswer NextAction = "final_answer" // reasoning complete
	NextReset       NextAction = "reset"        // restart reasoning (error recovery)
)

// ReasoningStep is one step in the chain-of-thought process.
type ReasoningStep struct {
	Title      string     `json:"title"`
	Action     string     `json:"action"`     // first person: "I will..."
	Result     string     `json:"result"`     // first person: "I found..."
	Reasoning  string     `json:"reasoning"`  // rationale and considerations
	NextAction NextAction `json:"next_action"`
	Confidence float64    `json:"confidence"` // 0.0-1.0
}

// ReasoningOutput holds the complete result of a reasoning session.
type ReasoningOutput struct {
	Steps   []ReasoningStep // structured steps taken
	Context string          // refined context injected into the agent prompt
	Native  bool            // true if native model reasoning was used
}

// ── Reasoning Execution ──────────────────────────────────────────────

// runReasoning executes reasoning before the main agent run.
// Returns a ReasoningOutput with steps and context for the agent.
func runReasoning(ctx context.Context, cfg *ReasoningConfig, model ModelProvider, userMessage string, session *Session) ([]ReasoningStep, string) {
	if cfg == nil || !cfg.Enabled {
		return nil, ""
	}

	reasoningModel := model
	if cfg.Model != nil {
		reasoningModel = cfg.Model
	}

	mode := cfg.Mode

	// Auto-detect: check if model supports native reasoning
	if mode == ReasoningAuto {
		if isNativeReasoningModel(reasoningModel) {
			mode = ReasoningNative
		} else {
			mode = ReasoningCoT
		}
	}

	if mode == ReasoningNative {
		output := runNativeReasoning(ctx, reasoningModel, userMessage, session)
		if output != nil {
			return output.Steps, output.Context
		}
		// Native failed — fall back to CoT
		slog.Debug("agnogo: native reasoning unavailable, falling back to CoT")
	}

	return runCoTReasoning(ctx, cfg, reasoningModel, userMessage)
}

// ── Native Reasoning Detection ───────────────────────────────────────

// NativeReasoner is an optional interface that ModelProviders can implement
// to indicate they support native reasoning (extended thinking, <think> tags, etc.).
type NativeReasoner interface {
	// Reason performs native reasoning and returns the thinking output.
	// The thinking content should be in the returned ModelResponse.Text,
	// with any <think>...</think> tags preserved.
	Reason(ctx context.Context, messages []Message) (*ModelResponse, error)
}

// isNativeReasoningModel checks if a model supports native reasoning.
func isNativeReasoningModel(model ModelProvider) bool {
	_, ok := model.(NativeReasoner)
	return ok
}

// runNativeReasoning uses the model's built-in reasoning capability.
func runNativeReasoning(ctx context.Context, model ModelProvider, userMessage string, session *Session) *ReasoningOutput {
	native, ok := model.(NativeReasoner)
	if !ok {
		return nil
	}

	messages := []Message{
		{Role: "user", Content: userMessage},
	}

	resp, err := native.Reason(ctx, messages)
	if err != nil {
		slog.Warn("agnogo: native reasoning failed", "error", err)
		return nil
	}

	// Extract thinking from <think> tags if present
	thinking, answer := extractThinking(resp.Text)

	step := ReasoningStep{
		Title:      "Native Reasoning",
		Reasoning:  thinking,
		Result:     answer,
		NextAction: NextFinalAnswer,
		Confidence: 0.9,
	}

	context := ""
	if thinking != "" {
		context = fmt.Sprintf("REASONING (model's internal thinking):\n%s\n\nUse this reasoning to inform your response.\n", thinking)
	}

	return &ReasoningOutput{
		Steps:   []ReasoningStep{step},
		Context: context,
		Native:  true,
	}
}

// extractThinking splits response text into thinking and answer parts.
// Handles <think>...</think>, <thinking>...</thinking>, and raw text.
func extractThinking(text string) (thinking, answer string) {
	// Try <think>...</think>
	for _, tag := range []string{"think", "thinking"} {
		openTag := "<" + tag + ">"
		closeTag := "</" + tag + ">"
		if idx := strings.Index(text, openTag); idx >= 0 {
			rest := text[idx+len(openTag):]
			if endIdx := strings.Index(rest, closeTag); endIdx >= 0 {
				thinking = strings.TrimSpace(rest[:endIdx])
				answer = strings.TrimSpace(rest[endIdx+len(closeTag):])
				if answer == "" {
					answer = strings.TrimSpace(text[:idx])
				}
				return thinking, answer
			}
		}
	}
	// No tags — entire text is the answer
	return "", text
}

// ── Chain-of-Thought Reasoning ───────────────────────────────────────

// cotPrompt is the system prompt for default chain-of-thought reasoning.
// Parameterized with min/max steps.
func cotPrompt(minSteps, maxSteps int) string {
	return fmt.Sprintf(`You are a reasoning engine. Think through the problem step-by-step.

PROCESS:
1. ANALYZE: Restate the task clearly in your own words
2. STRATEGIZE: Develop at least 2 solution approaches
3. PLAN: Select the best strategy and create a detailed action plan
4. EXECUTE: Document each step with title, action, result, and confidence (0.0-1.0)
5. VALIDATE: Cross-verify your solution using an alternative approach
6. ANSWER: Deliver the final validated answer

For each step, return JSON:
{
  "title": "Step title",
  "action": "I will...",
  "result": "I found/decided...",
  "reasoning": "Why this step is needed",
  "next_action": "continue|validate|final_answer|reset",
  "confidence": 0.95
}

Rules:
- Return one step at a time as JSON
- Minimum %d steps, maximum %d steps
- Use "validate" before "final_answer" — always cross-verify
- Use "reset" if you detect an error in your reasoning
- Set "final_answer" when done (only after validation)`, minSteps, maxSteps)
}

// runCoTReasoning executes multi-step chain-of-thought reasoning.
func runCoTReasoning(ctx context.Context, cfg *ReasoningConfig, model ModelProvider, userMessage string) ([]ReasoningStep, string) {
	minSteps := cfg.MinSteps
	if minSteps <= 0 {
		minSteps = 2
	}
	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 6
	}

	var steps []ReasoningStep
	messages := []Message{
		{Role: "system", Content: cotPrompt(minSteps, maxSteps)},
		{Role: "user", Content: userMessage},
	}

	for i := 0; i < maxSteps; i++ {
		if ctx.Err() != nil {
			break
		}

		resp, err := model.ChatCompletion(ctx, messages, nil)
		if err != nil {
			slog.Warn("agnogo: reasoning step failed", "step", i, "error", err)
			break
		}

		text := strings.TrimSpace(resp.Text)
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)

		var step ReasoningStep
		if err := json.Unmarshal([]byte(text), &step); err != nil {
			// Not valid JSON — use as raw reasoning
			step = ReasoningStep{
				Title:      fmt.Sprintf("Step %d", i+1),
				Reasoning:  text,
				Confidence: 0.5,
				NextAction: NextContinue,
			}
		}

		// Normalize legacy "DONE" to NextFinalAnswer
		if strings.EqualFold(string(step.NextAction), "done") {
			step.NextAction = NextFinalAnswer
		}

		steps = append(steps, step)
		slog.Debug("agnogo: reasoning step", "step", i+1, "title", step.Title,
			"confidence", step.Confidence, "next", step.NextAction)

		// Add to conversation for next step
		messages = append(messages, Message{Role: "assistant", Content: resp.Text})

		// Handle next action
		switch step.NextAction {
		case NextFinalAnswer:
			if i+1 >= minSteps {
				goto done
			}
			// Not enough steps yet — ask to continue
			messages = append(messages, Message{Role: "user", Content: "Good progress. Continue with more analysis before finalizing."})
		case NextValidate:
			messages = append(messages, Message{Role: "user", Content: "Now validate your reasoning by cross-verifying with an alternative approach."})
		case NextReset:
			slog.Debug("agnogo: reasoning reset requested", "step", i+1)
			messages = append(messages, Message{Role: "user", Content: "Acknowledged. Please restart your reasoning from the beginning with a fresh approach."})
		default: // NextContinue
			messages = append(messages, Message{Role: "user", Content: "Continue to the next step."})
		}
	}

done:
	// Build refined context from reasoning
	var sb strings.Builder
	sb.WriteString("REASONING (think step-by-step before responding):\n")
	for _, step := range steps {
		result := step.Result
		if result == "" {
			result = step.Reasoning
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", step.Title, result))
	}
	sb.WriteString("\nUse this reasoning to inform your response.\n")

	return steps, sb.String()
}

// ── Option for smart.go ──────────────────────────────────────────────

// WithReasoningConfig sets a custom reasoning configuration.
//
//	agent := agnogo.Agent("...", agnogo.WithReasoningConfig(agnogo.ReasoningConfig{
//	    Enabled:  true,
//	    Mode:     agnogo.ReasoningNative,
//	    MaxSteps: 10,
//	}))
func WithReasoningConfig(cfg ReasoningConfig) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.Reasoning = &cfg
	})
}
