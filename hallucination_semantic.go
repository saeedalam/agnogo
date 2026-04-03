package agnogo

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// ── Semantic Hallucination Detection ────────────────────────────────
//
// Detects hallucinations by verifying that LLM responses are grounded
// in the evidence (tool outputs) available in the session. Uses TF-IDF
// cosine similarity — zero external dependencies.
//
// This catches a different class of hallucinations than regex patterns:
// - Regex patterns catch: fabricated dates, times, prices (no tools called)
// - Semantic grounding catches: claims that contradict or aren't supported
//   by tool outputs (tools WERE called but response diverges)
//
// Usage (standalone):
//
//	checker := &agnogo.SemanticHallucinationChecker{MinGrounding: 0.3}
//	agent := agnogo.Agent("...", agnogo.Reliable(
//	    agnogo.WithCustomHallucination(checker),
//	))
//
// Usage (hybrid — combine regex + semantic):
//
//	agent := agnogo.Agent("...", agnogo.Reliable(
//	    agnogo.WithCustomHallucination(&agnogo.HybridHallucinationChecker{
//	        MinGrounding: 0.3,
//	    }),
//	))

// SemanticHallucinationChecker verifies that responses are grounded
// in tool outputs using TF-IDF cosine similarity.
type SemanticHallucinationChecker struct {
	// MinGrounding is the minimum similarity score (0.0–1.0) between the
	// response and tool outputs. Responses below this threshold are blocked.
	// Default: 0.3 (set to 0 to use default).
	MinGrounding float64
}

func (s *SemanticHallucinationChecker) Check(_ context.Context, session *Session, response string) error {
	minGrounding := s.MinGrounding
	if minGrounding <= 0 {
		minGrounding = 0.3
	}

	// Collect tool outputs from this turn.
	toolOutputs := extractToolOutputs(session)
	if len(toolOutputs) == 0 {
		return nil // no tools called — can't check grounding
	}

	// Compute grounding score: how well does the response align with tool outputs?
	evidence := strings.Join(toolOutputs, " ")
	score := cosineSimilarity(response, evidence)

	if score < minGrounding {
		return fmt.Errorf("response may not be grounded in tool data (score: %.2f, min: %.2f) [hallucination-guard]", score, minGrounding)
	}
	return nil
}

// HybridHallucinationChecker combines regex-based detection (for when no tools
// were called) with TF-IDF grounding verification (for when tools were called).
// This is the recommended checker for production use.
type HybridHallucinationChecker struct {
	// MinGrounding is the minimum similarity score for semantic grounding.
	// Default: 0.3.
	MinGrounding float64

	// ExtraPatterns are additional regex patterns for the regex detector.
	ExtraPatterns []string
}

func (h *HybridHallucinationChecker) Check(ctx context.Context, session *Session, response string) error {
	toolOutputs := extractToolOutputs(session)

	if len(toolOutputs) == 0 {
		// No tools called → use regex detection (existing approach).
		// Compile extra patterns if provided.
		var extra []*regexp.Regexp
		for _, p := range h.ExtraPatterns {
			if re, err := regexp.Compile("(?i)" + p); err == nil {
				extra = append(extra, re)
			}
		}
		detector := &hallucinationDetector{
			tools:         NewToolRegistry(), // empty — regex detector uses registered tools for relevance matching
			patterns:      getDefaultPatterns(),
			extraPatterns: extra,
		}
		return detector.check(ctx, session, response)
	}

	// Tools were called → check grounding.
	minGrounding := h.MinGrounding
	if minGrounding <= 0 {
		minGrounding = 0.3
	}

	evidence := strings.Join(toolOutputs, " ")
	score := cosineSimilarity(response, evidence)

	if score < minGrounding {
		return fmt.Errorf("response may not be grounded in tool data (score: %.2f, min: %.2f) [hallucination-guard]", score, minGrounding)
	}
	return nil
}

// ── TF-IDF Cosine Similarity ────────────────────────────────────────

// tokenize splits text into normalized lowercase tokens, filtering stopwords.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	for _, w := range words {
		if len(w) > 1 && !isStopWord(w) {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// termFreq computes normalized term frequency for a token list.
func termFreq(tokens []string) map[string]float64 {
	tf := make(map[string]float64, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	n := float64(len(tokens))
	if n == 0 {
		return tf
	}
	for k := range tf {
		tf[k] /= n
	}
	return tf
}

// idf computes inverse document frequency from two documents.
func idf(tf1, tf2 map[string]float64) map[string]float64 {
	df := make(map[string]int)
	for k := range tf1 {
		df[k]++
	}
	for k := range tf2 {
		df[k]++
	}
	result := make(map[string]float64, len(df))
	for k, count := range df {
		result[k] = math.Log(2.0/float64(count)) + 1 // smoothed IDF
	}
	return result
}

// cosineSimilarity computes TF-IDF cosine similarity between two texts.
// Returns 0.0 (completely different) to 1.0 (identical).
func cosineSimilarity(text1, text2 string) float64 {
	tokens1 := tokenize(text1)
	tokens2 := tokenize(text2)
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0
	}

	tf1 := termFreq(tokens1)
	tf2 := termFreq(tokens2)
	idfVals := idf(tf1, tf2)

	// Build TF-IDF vectors.
	var dot, norm1, norm2 float64
	allTerms := make(map[string]bool)
	for k := range tf1 {
		allTerms[k] = true
	}
	for k := range tf2 {
		allTerms[k] = true
	}

	for term := range allTerms {
		w1 := tf1[term] * idfVals[term]
		w2 := tf2[term] * idfVals[term]
		dot += w1 * w2
		norm1 += w1 * w1
		norm2 += w2 * w2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return dot / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// ── Helpers ─────────────────────────────────────────────────────────

// extractToolOutputs collects tool result contents from the current turn.
func extractToolOutputs(session *Session) []string {
	history := session.GetHistory()
	var outputs []string

	// Walk backwards from end, collecting tool outputs until we hit the user message.
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			break
		}
		if history[i].Role == "tool" && history[i].Content != "" {
			outputs = append(outputs, history[i].Content)
		}
	}
	return outputs
}


// isStopWord filters common English stopwords to improve TF-IDF quality.
func isStopWord(w string) bool {
	switch w {
	case "the", "is", "at", "which", "on", "a", "an", "and", "or", "but",
		"in", "to", "for", "of", "with", "by", "from", "as", "into",
		"that", "this", "it", "its", "was", "are", "be", "been", "being",
		"have", "has", "had", "do", "does", "did", "will", "would", "could",
		"should", "may", "might", "can", "not", "no", "if", "then", "than",
		"so", "up", "out", "about", "also", "just", "how", "what", "when",
		"where", "who", "all", "each", "every", "both", "few", "more",
		"some", "any", "most", "other", "we", "he", "she", "they", "you",
		"me", "him", "her", "them", "my", "your", "his", "our", "their":
		return true
	}
	return false
}
