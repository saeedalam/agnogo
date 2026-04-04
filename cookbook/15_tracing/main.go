// Structured Agent Tracing — See Inside Every Run()
//
// This example shows how to trace an agent's execution:
// every model call, tool call, and guardrail check —
// with timing, tokens, and cost.
//
// Run:
//   export OPENAI_API_KEY=sk-...
//   go run main.go

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/saeedalam/agnogo"
)

func main() {
	ctx := context.Background()

	// ── Step 1: Create a SpanCollector ──────────────────────────
	// This is the only setup needed. One line.

	sc := agnogo.NewSpanCollector()

	// ── Step 2: Create an agent with tracing + reliability ─────
	// Reliable() adds guardrails (hallucination, PII, confidence).
	// WithSpanCollector hooks the collector into every trace point.

	agent := agnogo.Agent(
		"You are a helpful assistant. Use your tools to answer questions accurately.",
		agnogo.Reliable(),
		agnogo.WithSpanCollector(sc),
	)

	// ── Step 3: Give the agent some tools ───────────────────────
	// These will show up as [tool] spans in the trace.

	agent.Tool("get_weather", "Get weather for a city", agnogo.Params{
		"city": {Type: "string", Desc: "City name", Required: true},
	}, func(_ context.Context, args map[string]string) (string, error) {
		time.Sleep(50 * time.Millisecond) // simulate API latency
		temps := []string{"22°C sunny", "18°C cloudy", "15°C rainy", "25°C clear"}
		return fmt.Sprintf("%s: %s", args["city"], temps[rand.Intn(len(temps))]), nil
	})

	agent.Tool("get_time", "Get current time in a timezone", agnogo.Params{
		"timezone": {Type: "string", Desc: "Timezone", Required: true},
	}, func(_ context.Context, args map[string]string) (string, error) {
		time.Sleep(30 * time.Millisecond) // simulate API latency
		return time.Now().Format("15:04 MST"), nil
	})

	// ── Step 4: Run the agent ──────────────────────────────────

	session := agnogo.NewSession("trace-demo")

	fmt.Println("Asking: What's the weather in Stockholm and what time is it?")
	fmt.Println()

	resp, err := agent.Run(ctx, session, "What's the weather in Stockholm and what time is it there?")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// ── Step 5: See the trace ──────────────────────────────────
	// This is the magic. One call shows everything that happened.

	trace := sc.Collect(resp)

	fmt.Println("═══ TRACE ═══")
	fmt.Println()
	trace.Print()

	fmt.Println("═══ RESPONSE ═══")
	fmt.Println()
	fmt.Println(resp.Text)

	// ── Step 6: Use trace data programmatically ────────────────
	// Cost alerts, quality monitoring, debugging.

	fmt.Println()
	fmt.Println("═══ INSIGHTS ═══")
	fmt.Println()
	fmt.Printf("  Total cost:    $%.4f\n", trace.TotalCost)
	fmt.Printf("  Total tokens:  %d\n", trace.TotalTokens)
	fmt.Printf("  Model calls:   %d\n", trace.ModelCalls)
	fmt.Printf("  Tool calls:    %d\n", trace.ToolCalls)
	fmt.Printf("  Duration:      %s\n", trace.Duration.Round(time.Millisecond))

	if trace.TotalCost > 0.01 {
		fmt.Println("  ⚠ Cost alert: conversation exceeded $0.01")
	} else {
		fmt.Println("  ✓ Cost within budget")
	}

	// ── Step 7: Export as JSON ─────────────────────────────────
	// Store traces for analytics, debugging, compliance.

	fmt.Println()
	fmt.Println("═══ JSON (first 500 chars) ═══")
	fmt.Println()
	jsonStr := trace.JSON()
	if len(jsonStr) > 500 {
		jsonStr = jsonStr[:500] + "..."
	}
	fmt.Println(jsonStr)
}
