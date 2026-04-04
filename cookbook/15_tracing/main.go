// Trace Intelligence — The Full Story
//
// Watch an agent run, store the trace, analyze patterns,
// detect anomalies, and replay with a different prompt.
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

	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     Trace Intelligence Demo              ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// ─────────────────────────────────────────────────────────
	// CHAPTER 1: Setup
	// ─────────────────────────────────────────────────────────
	//
	// Three lines connect tracing to persistence.
	// Every Run() is automatically captured and stored.

	store := agnogo.NewMemoryTraceStore()
	sc := agnogo.NewSpanCollector().WithTraceStore(store)

	agent := agnogo.Agent(
		"You are a helpful assistant. Use tools when asked. Be concise.",
		agnogo.Reliable(),
		agnogo.WithSpanCollector(sc),
	)

	agent.Tool("get_weather", "Get weather for a city", agnogo.Params{
		"city": {Type: "string", Desc: "City name", Required: true},
	}, func(_ context.Context, args map[string]string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		temps := []string{"22°C sunny", "18°C cloudy", "15°C rainy", "25°C clear"}
		return fmt.Sprintf("%s: %s", args["city"], temps[rand.Intn(len(temps))]), nil
	})

	agent.Tool("get_time", "Get current time", nil,
		func(_ context.Context, _ map[string]string) (string, error) {
			return time.Now().Format("15:04"), nil
		},
	)

	// ─────────────────────────────────────────────────────────
	// CHAPTER 2: Run the Agent Multiple Times
	// ─────────────────────────────────────────────────────────
	//
	// Each run is traced and auto-saved to the store.

	questions := []string{
		"What's the weather in Stockholm?",
		"What time is it right now?",
		"What's the weather in Tokyo and New York?",
	}

	session := agnogo.NewSession("demo-session")
	sc.SetSessionID(session.ID) // so traces are linked to this session

	for i, q := range questions {
		fmt.Printf("── Run %d: %s\n", i+1, q)

		resp, err := agent.Run(ctx, session, q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		trace := sc.Collect(resp)
		trace.Print()
		sc.Reset()

		fmt.Printf("  Answer: %s\n\n", truncate(resp.Text, 80))
	}

	fmt.Printf("Stored %d traces.\n\n", store.Count())

	// ─────────────────────────────────────────────────────────
	// CHAPTER 3: Query Traces
	// ─────────────────────────────────────────────────────────
	//
	// Find traces by cost, errors, session, or time window.

	fmt.Println("═══ QUERIES ═══")
	fmt.Println()

	// All traces from this session
	sessionTraces, _ := store.QueryTraces(ctx, agnogo.TraceQuery{
		SessionID: "demo-session",
	})
	fmt.Printf("  Traces in this session: %d\n", len(sessionTraces))

	// Traces that cost more than $0.0001
	expensive, _ := store.QueryTraces(ctx, agnogo.TraceQuery{MinCost: 0.0001})
	fmt.Printf("  Traces costing > $0.0001: %d\n", len(expensive))

	// Traces with errors (should be 0)
	hasErrors := true
	errorTraces, _ := store.QueryTraces(ctx, agnogo.TraceQuery{HasErrors: &hasErrors})
	fmt.Printf("  Traces with errors: %d\n", len(errorTraces))
	fmt.Println()

	// ─────────────────────────────────────────────────────────
	// CHAPTER 4: Analyze
	// ─────────────────────────────────────────────────────────
	//
	// Cost trends, anomalies, tool statistics.

	fmt.Println("═══ ANALYTICS ═══")
	fmt.Println()

	analyzer := agnogo.NewTraceAnalyzer(store)

	// Cost summary
	summary, _ := analyzer.CostSummary(ctx, time.Now().Add(-time.Hour))
	fmt.Printf("  Cost Summary (last hour):\n")
	fmt.Printf("    Runs:  %d\n", summary.RunCount)
	fmt.Printf("    Total: $%.4f\n", summary.TotalCost)
	fmt.Printf("    Avg:   $%.4f\n", summary.AvgCost)
	fmt.Printf("    Max:   $%.4f\n", summary.MaxCost)
	fmt.Printf("    Rate:  $%.4f/hour\n", summary.CostPerHour)
	fmt.Println()

	// Tool statistics
	toolStats, _ := analyzer.ToolStats(ctx, time.Now().Add(-time.Hour))
	fmt.Printf("  Tool Stats:\n")
	for name, s := range toolStats {
		fmt.Printf("    %s: %d calls, avg %s, error rate %.0f%%\n",
			name, s.CallCount, s.AvgDuration.Round(time.Millisecond), s.ErrorRate*100)
	}
	fmt.Println()

	// Anomaly detection
	anomalies, _ := analyzer.DetectAnomalies(ctx, time.Now().Add(-time.Hour))
	fmt.Printf("  Anomalies: %d found\n", len(anomalies))
	for _, a := range anomalies {
		fmt.Printf("    [%s] %s: %s\n", a.RunID, a.Type, a.Message)
	}
	fmt.Println()

	// ─────────────────────────────────────────────────────────
	// CHAPTER 5: Replay
	// ─────────────────────────────────────────────────────────
	//
	// Re-run the first trace with a different agent.
	// Compare cost, tokens, and behavior.

	fmt.Println("═══ REPLAY ═══")
	fmt.Println()

	// Load the first trace
	allTraces, _ := store.QueryTraces(ctx, agnogo.TraceQuery{Limit: 1})
	if len(allTraces) > 0 {
		original := allTraces[0]

		// Create a more concise agent for comparison
		conciseAgent := agnogo.Agent(
			"You are an extremely concise assistant. Answer in 10 words or fewer. Use tools.",
			agnogo.Reliable(),
		)
		conciseAgent.Tool("get_weather", "Get weather", agnogo.Params{
			"city": {Type: "string", Desc: "City", Required: true},
		}, func(_ context.Context, args map[string]string) (string, error) {
			return fmt.Sprintf("%s: 20°C", args["city"]), nil
		})

		result, err := agnogo.Replay(ctx, original, conciseAgent)
		if err != nil {
			fmt.Printf("  Replay failed: %v\n", err)
		} else {
			result.Print()
		}
	}

	// ─────────────────────────────────────────────────────────
	// CHAPTER 6: JSON Export
	// ─────────────────────────────────────────────────────────
	//
	// Every trace is JSON-serializable for storage and analytics.

	fmt.Println("═══ JSON EXPORT (first trace, 300 chars) ═══")
	fmt.Println()
	if len(allTraces) > 0 {
		j := allTraces[0].JSON()
		if len(j) > 300 {
			j = j[:300] + "..."
		}
		fmt.Println(j)
	}
	fmt.Println()

	fmt.Println("Done. Every agent run was traced, stored, analyzed, and replayed.")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
