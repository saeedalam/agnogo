package agnogo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// DebugConfig controls debug output.
// Level 1: key decisions (tool calls, model responses)
// Level 2: everything (full messages, args, results)
type DebugConfig struct {
	Enabled bool
	Level   int            // 1 or 2
	Printer func(string)   // custom output (default: fmt.Println)
	NoColor bool           // disable ANSI colors
}

// DefaultDebug returns debug config that prints to stdout.
func DefaultDebug() DebugConfig {
	return DebugConfig{Enabled: true, Level: 1, Printer: func(s string) { fmt.Println(s) }}
}

// VerboseDebug returns level 2 debug.
func VerboseDebug() DebugConfig {
	return DebugConfig{Enabled: true, Level: 2, Printer: func(s string) { fmt.Println(s) }}
}

func (d DebugConfig) color(c, s string) string {
	if d.NoColor {
		return s
	}
	return c + s + colorReset
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

func (d DebugConfig) printRunStart(runID, sessionID string) {
	bar := strings.Repeat("─", 50)
	d.print(1, "%s", "")
	d.print(1, "%s", d.color(colorCyan, "┌"+bar+"┐"))
	d.print(1, "%s %s  Run: %s", d.color(colorCyan, "│"), d.color(colorBold, "▶ Agent Run Start"), d.color(colorGray, runID))
	d.print(1, "%s   Session: %s", d.color(colorCyan, "│"), d.color(colorGray, sessionID))
	d.print(1, "%s", d.color(colorCyan, "└"+bar+"┘"))
}

func (d DebugConfig) printRunEnd(runID string, m *RunMetrics) {
	if m == nil {
		return
	}
	bar := strings.Repeat("─", 50)
	d.print(1, "%s", "")
	d.print(1, "%s", d.color(colorGreen, "┌"+bar+"┐"))
	d.print(1, "%s %s  Run: %s", d.color(colorGreen, "│"), d.color(colorBold, "■ Agent Run End"), d.color(colorGray, runID))
	d.print(1, "%s   Duration:     %s", d.color(colorGreen, "│"), d.color(colorYellow, m.Duration.Round(time.Millisecond).String()))
	d.print(1, "%s   Model calls:  %d", d.color(colorGreen, "│"), m.ModelCalls)
	if m.ToolCalls > 0 {
		d.print(1, "%s   Tool calls:   %d", d.color(colorGreen, "│"), m.ToolCalls)
	}
	if m.TotalTokens > 0 {
		d.print(1, "%s   Tokens:       %s in / %s out / %s total",
			d.color(colorGreen, "│"),
			d.color(colorCyan, fmt.Sprintf("%d", m.InputTokens)),
			d.color(colorCyan, fmt.Sprintf("%d", m.OutputTokens)),
			d.color(colorBold, fmt.Sprintf("%d", m.TotalTokens)))
	}
	d.print(1, "%s", d.color(colorGreen, "└"+bar+"┘"))
	d.print(1, "%s", "")
}

func (d DebugConfig) printModelCall(msgCount int, toolCount int, dur time.Duration) {
	d.print(1, "  %s Model call: %d messages → %d tool calls %s",
		d.color(colorBlue, "🤖"),
		msgCount, toolCount,
		d.color(colorGray, fmt.Sprintf("(%dms)", dur.Milliseconds())))
}

func (d DebugConfig) printToolCall(name string, args map[string]string, result string, dur time.Duration, err error) {
	status := d.color(colorGreen, "✅")
	if err != nil {
		status = d.color(colorRed, "❌")
	}
	d.print(1, "  %s Tool: %s %s",
		status,
		d.color(colorYellow, name),
		d.color(colorGray, fmt.Sprintf("(%dms)", dur.Milliseconds())))
	if d.Level >= 2 {
		argsJSON, _ := json.Marshal(args)
		d.print(2, "     %s %s", d.color(colorGray, "Args:"), string(argsJSON))
		d.print(2, "     %s %s", d.color(colorGray, "Result:"), truncateStr(result, 200))
	}
}

func (d DebugConfig) printKnowledge(query, result string, dur time.Duration) {
	d.print(1, "  %s Knowledge: %q → %d chars %s",
		d.color(colorCyan, "📚"),
		truncateStr(query, 50), len(result),
		d.color(colorGray, fmt.Sprintf("(%dms)", dur.Milliseconds())))
}

func (d DebugConfig) printMemory(key, value string) {
	d.print(1, "  %s Memory: %s = %q",
		d.color(colorCyan, "🧠"),
		d.color(colorBold, key), value)
}

func (d DebugConfig) printGuardrail(name, direction string, blocked bool) {
	if blocked {
		d.print(1, "  %s Guardrail %s: %s (%s)",
			d.color(colorRed, "🛡️"),
			d.color(colorRed, "BLOCKED"),
			name, direction)
	}
}

func (d DebugConfig) printResponse(text string) {
	d.print(1, "  %s Response: %s",
		d.color(colorGreen, "💬"),
		truncateStr(text, 100))
}

func (d DebugConfig) printRouting(agentName string) {
	d.print(1, "  %s Routed to: %s",
		d.color(colorBlue, "🔀"),
		d.color(colorYellow, agentName))
}

func (d DebugConfig) printApproval(toolName, reason string) {
	d.print(1, "  %s Approval needed: %s — %s",
		d.color(colorYellow, "⏸️"),
		d.color(colorBold, toolName), reason)
}

func (d DebugConfig) printRetry(attempt int, delay time.Duration) {
	d.print(1, "  %s Retry #%d %s",
		d.color(colorYellow, "🔄"),
		attempt,
		d.color(colorGray, fmt.Sprintf("(waiting %s)", delay)))
}

func (d DebugConfig) printHistory(total, kept int) {
	if total > kept {
		d.print(1, "  %s History trimmed: %d → %d messages",
			d.color(colorGray, "✂️"),
			total, kept)
	}
}

func (d DebugConfig) printMessages(messages []Message) {
	if d.Level < 2 {
		return
	}
	d.print(2, "  %s Messages (%d):", d.color(colorGray, "📝"), len(messages))
	for _, m := range messages {
		content := truncateStr(m.Content, 80)
		content = strings.ReplaceAll(content, "\n", " ")
		roleColor := colorGray
		switch m.Role {
		case "user":
			roleColor = colorCyan
		case "assistant":
			roleColor = colorGreen
		case "system":
			roleColor = colorBlue
		case "tool":
			roleColor = colorYellow
		}
		d.print(2, "     %s %s", d.color(roleColor, "["+m.Role+"]"), content)
	}
}
