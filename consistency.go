package agnogo

import (
	"context"
	"strings"
)

// ConsistencyResult holds the result of a multi-sample consistency check.
//
// NOTE: This is an EVALUATION tool, not a production guardrail. It costs N extra
// LLM calls per check. Use it for:
//   - Testing: "does my agent hallucinate on these 100 questions?"
//   - Critical decisions: "is this medical/legal/financial answer trustworthy?"
//   - Debugging: "why did the agent give a wrong answer?"
//
// Do NOT use on every request — it's expensive. For production, use the built-in
// confidence scoring (WithConfidence) and hallucination guard (HallucinationGuard)
// which cost zero extra LLM calls.
//
// Based on: "Efficient Hallucination Detection: Adaptive Bayesian Estimation
// of Semantic Entropy" (arXiv:2506.09886). The paper shows that inconsistent
// answers at temperature > 0 indicate hallucination — real facts are stable,
// hallucinated facts vary.
type ConsistencyResult struct {
	Score      float64  `json:"score"`      // 0.0 (inconsistent) to 1.0 (consistent)
	Samples    int      `json:"samples"`    // number of samples generated
	Agreement  float64  `json:"agreement"`  // pairwise agreement ratio
	Consistent bool     `json:"consistent"` // score >= threshold
	Responses  []string `json:"responses"`  // the actual responses
}

// ConsistencyConfig configures the consistency checker.
type ConsistencyConfig struct {
	Samples   int     // how many times to ask (default 3)
	Threshold float64 // agreement threshold (default 0.7)
}

// DefaultConsistencyConfig returns sensible defaults.
func DefaultConsistencyConfig() ConsistencyConfig {
	return ConsistencyConfig{Samples: 3, Threshold: 0.7}
}

// CheckConsistency asks the model the same question multiple times and measures
// how much the answers agree. High agreement = confident. Low agreement = likely
// hallucinating.
//
// Works with any agent — uses the agent's model, instructions, and knowledge.
// Each sample gets a fresh session so responses are independent.
//
//	result := agnogo.CheckConsistency(ctx, agent, "What is the capital of France?", agnogo.DefaultConsistencyConfig())
//	if !result.Consistent {
//	    // Model gave different answers each time — don't trust it
//	}
func CheckConsistency(ctx context.Context, agent *Core, question string, cfg ConsistencyConfig) ConsistencyResult {
	if cfg.Samples < 2 {
		cfg.Samples = 3
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 0.7
	}

	// Collect responses from independent sessions
	responses := make([]string, 0, cfg.Samples)
	for i := 0; i < cfg.Samples; i++ {
		session := NewSession(generateRunID())
		resp, err := agent.Run(ctx, session, question)
		if err != nil {
			continue
		}
		if resp != nil && resp.Text != "" {
			responses = append(responses, resp.Text)
		}
	}

	if len(responses) < 2 {
		return ConsistencyResult{
			Score: 0, Samples: len(responses), Consistent: false, Responses: responses,
		}
	}

	// Compute pairwise agreement
	agreement := pairwiseAgreement(responses)

	return ConsistencyResult{
		Score:      agreement,
		Samples:    len(responses),
		Agreement:  agreement,
		Consistent: agreement >= cfg.Threshold,
		Responses:  responses,
	}
}

// pairwiseAgreement measures how similar responses are to each other.
// Uses keyword overlap (Jaccard similarity) averaged across all pairs.
//
// Algorithm:
//  1. For each response, extract significant words (>3 chars, lowercased)
//  2. For each pair of responses, compute Jaccard: |intersection| / |union|
//  3. Average all pairwise Jaccard scores
//
// This is the "low-budget" approach from the paper — no embeddings needed.
func pairwiseAgreement(responses []string) float64 {
	if len(responses) < 2 {
		return 0.0 // can't measure consistency with fewer than 2 responses
	}

	// Extract word sets for each response
	wordSets := make([]map[string]bool, len(responses))
	for i, r := range responses {
		wordSets[i] = significantWords(r)
	}

	// Compute average pairwise Jaccard similarity
	var totalSim float64
	var pairs int

	for i := 0; i < len(wordSets); i++ {
		for j := i + 1; j < len(wordSets); j++ {
			totalSim += jaccard(wordSets[i], wordSets[j])
			pairs++
		}
	}

	if pairs == 0 {
		return 1.0
	}
	return totalSim / float64(pairs)
}

// significantWords extracts meaningful words from text.
// Filters out short words and common stop words.
func significantWords(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		// Strip punctuation
		w = strings.Trim(w, ".,!?;:\"'()[]{}–—-")
		// Skip short words and stop words
		if len(w) > 3 && !stopWords[w] {
			words[w] = true
		}
	}
	return words
}

// jaccard computes the Jaccard similarity between two sets: |A∩B| / |A∪B|
func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}

	union := len(a)
	for w := range b {
		if !a[w] {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// Common English stop words to ignore in similarity comparison.
var stopWords = map[string]bool{
	"that": true, "this": true, "with": true, "from": true,
	"have": true, "been": true, "were": true, "they": true,
	"their": true, "there": true, "about": true, "would": true,
	"could": true, "should": true, "which": true, "where": true,
	"when": true, "what": true, "will": true, "your": true,
	"than": true, "then": true, "them": true, "these": true,
	"those": true, "into": true, "also": true, "some": true,
	"such": true, "each": true, "very": true, "more": true,
	"most": true, "other": true, "just": true, "only": true,
	"over": true, "after": true, "does": true, "here": true,
}
