package agnogo

import (
	"context"
	"fmt"
	"testing"
)

func TestCheckConsistencyAgreement(t *testing.T) {
	// Model returns very similar answers → high consistency
	model := &mockModel{responses: []ModelResponse{
		{Text: "The capital of France is Paris. Paris is located in northern France along the Seine river."},
		{Text: "Paris is the capital of France. It is located in northern France on the Seine river."},
		{Text: "France has Paris as its capital city. Paris sits in northern France beside the Seine river."},
	}}
	agent := New(Config{Model: model, Instructions: "test"})

	result := CheckConsistency(context.Background(), agent, "What is the capital of France?", ConsistencyConfig{Samples: 3, Threshold: 0.4})

	if result.Samples != 3 {
		t.Errorf("samples = %d, want 3", result.Samples)
	}
	if result.Score < 0.3 {
		t.Errorf("score = %.2f, expected > 0.3 for similar answers", result.Score)
	}
	t.Logf("consistency score: %.2f", result.Score)
}

func TestCheckConsistencyDisagreement(t *testing.T) {
	// Model returns different answers → low consistency
	model := &mockModel{responses: []ModelResponse{
		{Text: "The population of Tokyo is 14 million people in the metropolitan area."},
		{Text: "Tokyo has approximately 37 million residents in the greater area."},
		{Text: "About 9.7 million people live in central Tokyo proper."},
	}}
	agent := New(Config{Model: model, Instructions: "test"})

	result := CheckConsistency(context.Background(), agent, "Population of Tokyo?", DefaultConsistencyConfig())

	if result.Score > 0.8 {
		t.Errorf("score = %.2f, expected lower for inconsistent answers", result.Score)
	}
}

func TestPairwiseAgreementIdentical(t *testing.T) {
	responses := []string{
		"The answer is 42.",
		"The answer is 42.",
		"The answer is 42.",
	}
	score := pairwiseAgreement(responses)
	if score != 1.0 {
		t.Errorf("identical responses: score = %.2f, want 1.0", score)
	}
}

func TestPairwiseAgreementCompleteDifference(t *testing.T) {
	responses := []string{
		"Apples oranges bananas grapes mangoes",
		"Quantum physics relativity entropy gravity",
		"Stockholm Sweden winter snowfall temperature",
	}
	score := pairwiseAgreement(responses)
	if score > 0.1 {
		t.Errorf("unrelated responses: score = %.2f, want < 0.1", score)
	}
}

func TestJaccardSimilarity(t *testing.T) {
	a := map[string]bool{"paris": true, "capital": true, "france": true}
	b := map[string]bool{"paris": true, "city": true, "france": true}

	sim := jaccard(a, b)
	// intersection = 2 (paris, france), union = 4 (paris, capital, france, city)
	expected := 2.0 / 4.0
	if sim != expected {
		t.Errorf("jaccard = %.2f, want %.2f", sim, expected)
	}
}

func TestJaccardEmpty(t *testing.T) {
	a := map[string]bool{}
	b := map[string]bool{}
	if jaccard(a, b) != 1.0 {
		t.Error("empty sets should have similarity 1.0")
	}
}

func TestSignificantWords(t *testing.T) {
	words := significantWords("The capital of France is Paris, which is beautiful.")
	// Should include: capital, france, paris, beautiful (>3 chars, not stop words)
	// Should exclude: the (stop), is (short), of (short), which (stop)
	if !words["capital"] {
		t.Error("should include 'capital'")
	}
	if !words["france"] {
		t.Error("should include 'france'")
	}
	if !words["paris"] {
		t.Error("should include 'paris'")
	}
	if words["which"] {
		t.Error("should exclude stop word 'which'")
	}
}

func TestCheckConsistencyTooFewSamples(t *testing.T) {
	// Model returns errors for all calls
	model := &errModel{err: fmt.Errorf("api down")}
	agent := New(Config{Model: model, Instructions: "test"})

	result := CheckConsistency(context.Background(), agent, "test", ConsistencyConfig{Samples: 3, Threshold: 0.7})

	if result.Consistent {
		t.Error("should not be consistent with failed samples")
	}
	if result.Score != 0 {
		t.Errorf("score should be 0 for failed samples, got %.2f", result.Score)
	}
}
