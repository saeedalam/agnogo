package agnogo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolValidator checks tool output against configurable constraints.
type ToolValidator struct {
	MaxOutputSize   int           // max bytes (default 50KB, 0 = unlimited)
	RequireNonEmpty bool          // reject empty tool results
	JSONValidate    bool          // validate JSON is well-formed if output looks like JSON
	Timeout         time.Duration // per-tool timeout (default 30s, 0 = no timeout)
}

// validateToolOutput checks a tool result against the validator config.
// Returns the (possibly modified) result and any error.
func (v *ToolValidator) validateToolOutput(toolName, result string) (string, error) {
	if v.RequireNonEmpty && strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("agnogo: tool %q returned empty result", toolName)
	}

	maxSize := v.MaxOutputSize
	if maxSize == 0 {
		maxSize = 50 * 1024 // 50KB default
	}
	if maxSize > 0 && len(result) > maxSize {
		return "", fmt.Errorf("agnogo: tool %q output exceeds max size (%d > %d bytes)", toolName, len(result), maxSize)
	}

	if v.JSONValidate && looksLikeJSON(result) {
		if !json.Valid([]byte(result)) {
			return result, fmt.Errorf("agnogo: tool %q returned invalid JSON", toolName)
		}
	}

	return result, nil
}

// looksLikeJSON returns true if the trimmed string starts with { or [.
func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

// ── Loop Detection ──────────────────────────────────────────

// LoopDetector identifies repetitive tool call patterns.
type LoopDetector struct {
	MaxRepeats    int                  // same tool+args (default 3)
	MaxCycles     int                  // A->B->A pattern (default 2)
	MaxToolErrors int                  // same tool errors N times (default 3)
	OnLoop        func(pattern string) // alert callback

	callHistory []string       // ordered tool names
	errorCounts map[string]int // tool name -> error count
}

// NewLoopDetector creates a loop detector with the given limits.
func NewLoopDetector(maxRepeats, maxCycles, maxToolErrors int) *LoopDetector {
	if maxRepeats <= 0 {
		maxRepeats = 3
	}
	if maxCycles <= 0 {
		maxCycles = 2
	}
	if maxToolErrors <= 0 {
		maxToolErrors = 3
	}
	return &LoopDetector{
		MaxRepeats:    maxRepeats,
		MaxCycles:     maxCycles,
		MaxToolErrors: maxToolErrors,
		callHistory:   []string{},
		errorCounts:   map[string]int{},
	}
}

// RecordCall records a tool call for pattern analysis.
func (ld *LoopDetector) RecordCall(toolName string) {
	ld.callHistory = append(ld.callHistory, toolName)
}

// RecordError records a tool error.
func (ld *LoopDetector) RecordError(toolName string) {
	ld.errorCounts[toolName]++
}

// DetectCycle checks for A->B->A->B cycling pattern in recent calls.
// Returns true and a description if a cycle is detected.
func (ld *LoopDetector) DetectCycle() (bool, string) {
	h := ld.callHistory
	n := len(h)
	if n < 4 {
		return false, ""
	}

	// Check for 2-element cycle: A B A B
	for cycleLen := 2; cycleLen <= 3; cycleLen++ {
		if n < cycleLen*2 {
			continue
		}
		tail := h[n-cycleLen*2:]
		pattern := tail[:cycleLen]
		matches := 0
		for i := 0; i < len(tail)-cycleLen+1; i += cycleLen {
			chunk := tail[i:]
			if len(chunk) < cycleLen {
				break
			}
			match := true
			for j := 0; j < cycleLen; j++ {
				if chunk[j] != pattern[j] {
					match = false
					break
				}
			}
			if match {
				matches++
			}
		}
		if matches >= ld.MaxCycles {
			desc := fmt.Sprintf("cycle detected: %s (repeated %d times)", strings.Join(pattern, " -> "), matches)
			if ld.OnLoop != nil {
				ld.OnLoop(desc)
			}
			return true, desc
		}
	}

	return false, ""
}

// ShouldSkipTool returns true if a tool has errored too many times.
func (ld *LoopDetector) ShouldSkipTool(toolName string) bool {
	return ld.errorCounts[toolName] >= ld.MaxToolErrors
}

// ── WithToolValidation Option ───────────────────────────────

// WithToolValidation configures tool output validation.
func WithToolValidation(v ToolValidator) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.toolValidator = &v
	})
}
