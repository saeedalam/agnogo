package agnogo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DebugConfig controls debug output.
// Level 1: key decisions (tool calls, model responses)
// Level 2: everything (full messages, args, results)
type DebugConfig struct {
	Enabled bool
	Level   int    // 1 or 2
	Printer func(string) // custom output (default: fmt.Println)
}

// DefaultDebug returns debug config that prints to stdout.
func DefaultDebug() DebugConfig {
	return DebugConfig{Enabled: true, Level: 1, Printer: func(s string) { fmt.Println(s) }}
}

// VerboseDebug returns level 2 debug.
func VerboseDebug() DebugConfig {
	return DebugConfig{Enabled: true, Level: 2, Printer: func(s string) { fmt.Println(s) }}
}

func (d DebugConfig) print(level int, format string, args ...any) {
	if !d.Enabled || level > d.Level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if d.Printer != nil {
		d.Printer(msg)
	}
}

func (d DebugConfig) printModelCall(msgCount int, toolCount int, dur time.Duration) {
	d.print(1, "🤖 Model call: %d messages → %d tool calls (%dms)", msgCount, toolCount, dur.Milliseconds())
}

func (d DebugConfig) printToolCall(name string, args map[string]string, result string, dur time.Duration, err error) {
	status := "✅"
	if err != nil {
		status = "❌"
	}
	d.print(1, "🔧 %s Tool: %s (%dms)", status, name, dur.Milliseconds())
	if d.Level >= 2 {
		argsJSON, _ := json.Marshal(args)
		d.print(2, "   Args: %s", string(argsJSON))
		d.print(2, "   Result: %s", truncateStr(result, 200))
	}
}

func (d DebugConfig) printKnowledge(query, result string, dur time.Duration) {
	d.print(1, "📚 Knowledge search: %q → %d chars (%dms)", truncateStr(query, 50), len(result), dur.Milliseconds())
}

func (d DebugConfig) printMemory(key, value string) {
	d.print(1, "🧠 Memory: %s = %q", key, value)
}

func (d DebugConfig) printGuardrail(name, direction string, blocked bool) {
	if blocked {
		d.print(1, "🛡️ Guardrail BLOCKED: %s (%s)", name, direction)
	}
}

func (d DebugConfig) printResponse(text string) {
	d.print(1, "💬 Response: %s", truncateStr(text, 100))
}

func (d DebugConfig) printRouting(agentName string) {
	d.print(1, "🔀 Routed to: %s", agentName)
}

func (d DebugConfig) printApproval(toolName, reason string) {
	d.print(1, "⏸️ Approval needed: %s — %s", toolName, reason)
}

func (d DebugConfig) printRetry(attempt int, delay time.Duration) {
	d.print(1, "🔄 Retry #%d (waiting %s)", attempt, delay)
}

func (d DebugConfig) printHistory(total, kept int) {
	if total > kept {
		d.print(1, "✂️ History trimmed: %d → %d messages", total, kept)
	}
}

func (d DebugConfig) printMessages(messages []Message) {
	if d.Level < 2 {
		return
	}
	d.print(2, "📝 Messages (%d):", len(messages))
	for _, m := range messages {
		content := truncateStr(m.Content, 80)
		content = strings.ReplaceAll(content, "\n", " ")
		d.print(2, "   [%s] %s", m.Role, content)
	}
}
