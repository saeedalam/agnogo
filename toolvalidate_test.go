package agnogo

import (
	"strings"
	"testing"
)

func TestToolValidatorMaxSize(t *testing.T) {
	v := &ToolValidator{
		MaxOutputSize: 100,
	}

	// Small output passes
	result, err := v.validateToolOutput("test", "hello")
	if err != nil {
		t.Fatalf("small output should pass: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}

	// Large output rejected
	big := strings.Repeat("x", 200)
	_, err = v.validateToolOutput("test", big)
	if err == nil {
		t.Fatal("expected error for oversized output")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestToolValidatorNonEmpty(t *testing.T) {
	v := &ToolValidator{
		RequireNonEmpty: true,
	}

	// Non-empty passes
	result, err := v.validateToolOutput("test", "data")
	if err != nil {
		t.Fatalf("non-empty should pass: %v", err)
	}
	if result != "data" {
		t.Errorf("expected 'data', got %q", result)
	}

	// Empty rejected
	_, err = v.validateToolOutput("test", "")
	if err == nil {
		t.Fatal("expected error for empty output")
	}
	if !strings.Contains(err.Error(), "empty result") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Whitespace-only rejected
	_, err = v.validateToolOutput("test", "   \n  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only output")
	}
}

func TestToolValidatorJSONInvalid(t *testing.T) {
	v := &ToolValidator{
		JSONValidate: true,
	}

	// Invalid JSON flagged
	_, err := v.validateToolOutput("test", `{"key": broken}`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestToolValidatorJSONValid(t *testing.T) {
	v := &ToolValidator{
		JSONValidate: true,
	}

	// Valid JSON passes
	result, err := v.validateToolOutput("test", `{"key": "value", "num": 42}`)
	if err != nil {
		t.Fatalf("valid JSON should pass: %v", err)
	}
	if result != `{"key": "value", "num": 42}` {
		t.Errorf("unexpected result: %q", result)
	}

	// Non-JSON content is not validated
	result, err = v.validateToolOutput("test", "just plain text")
	if err != nil {
		t.Fatalf("plain text should pass: %v", err)
	}
	if result != "just plain text" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestLoopDetectorCycle(t *testing.T) {
	ld := NewLoopDetector(3, 2, 3)

	// Record A -> B -> A -> B pattern
	ld.RecordCall("toolA")
	ld.RecordCall("toolB")
	ld.RecordCall("toolA")
	ld.RecordCall("toolB")

	detected, desc := ld.DetectCycle()
	if !detected {
		t.Fatal("expected cycle to be detected")
	}
	if !strings.Contains(desc, "toolA") || !strings.Contains(desc, "toolB") {
		t.Errorf("description should mention tool names: %q", desc)
	}

	// No cycle with varied calls
	ld2 := NewLoopDetector(3, 2, 3)
	ld2.RecordCall("toolA")
	ld2.RecordCall("toolB")
	ld2.RecordCall("toolC")
	ld2.RecordCall("toolD")

	detected, _ = ld2.DetectCycle()
	if detected {
		t.Fatal("no cycle should be detected for varied calls")
	}
}

func TestLoopDetectorToolErrors(t *testing.T) {
	ld := NewLoopDetector(3, 2, 3)

	// Not enough errors yet
	ld.RecordError("badTool")
	ld.RecordError("badTool")
	if ld.ShouldSkipTool("badTool") {
		t.Fatal("should not skip after 2 errors (threshold is 3)")
	}

	// Third error triggers skip
	ld.RecordError("badTool")
	if !ld.ShouldSkipTool("badTool") {
		t.Fatal("should skip after 3 errors")
	}

	// Other tools unaffected
	if ld.ShouldSkipTool("goodTool") {
		t.Fatal("unrelated tool should not be skipped")
	}
}

func TestLoopDetectorDefaults(t *testing.T) {
	ld := NewLoopDetector(0, 0, 0)
	if ld.MaxRepeats != 3 {
		t.Errorf("expected default MaxRepeats=3, got %d", ld.MaxRepeats)
	}
	if ld.MaxCycles != 2 {
		t.Errorf("expected default MaxCycles=2, got %d", ld.MaxCycles)
	}
	if ld.MaxToolErrors != 3 {
		t.Errorf("expected default MaxToolErrors=3, got %d", ld.MaxToolErrors)
	}
}
