package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ReasoningConfig enables chain-of-thought reasoning before the agent responds.
// Matches Agno's reasoning mode: structured multi-step thinking.
//
//	agent := agnogo.New(agnogo.Config{
//	    Reasoning: &agnogo.ReasoningConfig{
//	        Enabled:  true,
//	        MinSteps: 2,
//	        MaxSteps: 6,
//	    },
//	})
type ReasoningConfig struct {
	Enabled  bool
	Model    ModelProvider // separate model for reasoning (optional, uses agent model if nil)
	MinSteps int          // minimum reasoning steps (default 2)
	MaxSteps int          // maximum reasoning steps (default 6)
}

// ReasoningStep is one step in the chain-of-thought process.
type ReasoningStep struct {
	Title      string  `json:"title"`
	Reasoning  string  `json:"reasoning"`
	Action     string  `json:"action"`
	Result     string  `json:"result"`
	Confidence float64 `json:"confidence"` // 0.0-1.0
	NextStep   string  `json:"next_step"`
}

// reasoningPrompt is the system prompt that guides the reasoning agent.
// Based on Agno's 6-step framework.
const reasoningPrompt = `You are a reasoning engine. Think through the problem step-by-step.

PROCESS:
1. ANALYZE: Restate the task clearly in your own words
2. STRATEGIZE: Develop at least 2 solution approaches
3. PLAN: Select the best strategy and create a detailed action plan
4. EXECUTE: Document each step with title, action, result, and confidence (0.0-1.0)
5. VALIDATE: Cross-verify your solution
6. ANSWER: Deliver the final validated answer

For each step, return JSON:
{
  "title": "Step title",
  "reasoning": "Why this step is needed",
  "action": "What to do",
  "result": "What was found/decided",
  "confidence": 0.95,
  "next_step": "What comes next (or DONE if complete)"
}

Return one step at a time. When done, set next_step to "DONE".`

// runReasoning executes chain-of-thought reasoning before the main agent run.
// Returns the reasoning steps and a refined prompt for the agent.
func runReasoning(ctx context.Context, cfg *ReasoningConfig, model ModelProvider, userMessage string, session *Session) ([]ReasoningStep, string) {
	if cfg == nil || !cfg.Enabled {
		return nil, ""
	}

	reasoningModel := model
	if cfg.Model != nil {
		reasoningModel = cfg.Model
	}

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
		{Role: "system", Content: reasoningPrompt},
		{Role: "user", Content: userMessage},
	}

	for i := 0; i < maxSteps; i++ {
		resp, err := reasoningModel.ChatCompletion(ctx, messages, nil)
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
				Title:     fmt.Sprintf("Step %d", i+1),
				Reasoning: text,
				Confidence: 0.5,
			}
		}
		steps = append(steps, step)

		slog.Debug("agnogo: reasoning step", "step", i+1, "title", step.Title, "confidence", step.Confidence)

		// Add to conversation for next step
		messages = append(messages, Message{Role: "assistant", Content: resp.Text})
		messages = append(messages, Message{Role: "user", Content: "Continue to the next step."})

		// Stop if done and minimum steps reached
		if strings.EqualFold(step.NextStep, "DONE") && i+1 >= minSteps {
			break
		}
	}

	// Build refined context from reasoning
	var sb strings.Builder
	sb.WriteString("REASONING (think step-by-step before responding):\n")
	for _, step := range steps {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", step.Title, step.Result))
	}
	sb.WriteString("\nUse this reasoning to inform your response.\n")

	return steps, sb.String()
}
