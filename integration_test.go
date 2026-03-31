//go:build integration

// Integration tests that hit real LLM APIs.
// Run with: source .env && go test -tags integration -v -run TestIntegration -timeout 120s
//
// Requires: OPENAI_API_KEY (and optionally GEMINI_API_KEY)
package agnogo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func skipIfNoKey(t *testing.T, envVar string) string {
	t.Helper()
	key := os.Getenv(envVar)
	if key == "" {
		t.Skipf("skipping: %s not set", envVar)
	}
	return key
}

// ── 1. OpenAI basic chat ────────────────────────────────

func TestIntegrationOpenAIChat(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	agent := Agent("You are a helpful assistant. Answer in one sentence.", WithOpenAI("gpt-4.1-mini"))
	answer, err := agent.Ask(context.Background(), "What is 2+2?")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}
	if answer == "" {
		t.Fatal("empty response")
	}
	if !strings.Contains(strings.ToLower(answer), "4") {
		t.Errorf("expected '4' in response, got: %s", answer)
	}
	t.Logf("OpenAI response: %s", answer)
}

// ── 2. OpenAI with tools ────────────────────────────────

func TestIntegrationOpenAITools(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	type CityInput struct {
		City string `json:"city" desc:"City name" required:"true"`
	}

	weather := TypedTool("get_weather", "Get current weather for a city",
		func(ctx context.Context, in CityInput) (string, error) {
			return fmt.Sprintf(`{"city":"%s","temp":18,"condition":"sunny"}`, in.City), nil
		})

	agent := Agent("You are a weather assistant. Always use the get_weather tool.", Tools(weather), WithOpenAI("gpt-4.1-mini"))

	session := NewSession("integration-tools")
	resp, err := agent.Run(context.Background(), session, "What's the weather in Stockholm?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(resp.ToolsCalled) == 0 {
		t.Error("expected tool to be called, but no tools were called")
	}
	if !strings.Contains(strings.ToLower(resp.Text), "stockholm") && !strings.Contains(strings.ToLower(resp.Text), "18") {
		t.Errorf("response doesn't mention Stockholm or 18: %s", resp.Text)
	}
	t.Logf("Tools called: %v", resp.ToolsCalled)
	t.Logf("Response: %s", resp.Text)
}

// ── 3. OpenAI structured output ─────────────────────────

func TestIntegrationOpenAIStructured(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	type CityInfo struct {
		Name       string `json:"name"`
		Country    string `json:"country"`
		Population int    `json:"population"`
	}

	agent := Agent("You provide city data. Respond with valid JSON only.", WithOpenAI("gpt-4.1-mini"))

	var info CityInfo
	err := AskStructured(context.Background(), agent, "Tell me about Tokyo", &info)
	if err != nil {
		t.Fatalf("AskStructured failed: %v", err)
	}
	if info.Name == "" {
		t.Error("name is empty")
	}
	if info.Population == 0 {
		t.Error("population is 0")
	}
	t.Logf("Structured: %+v", info)
}

// ── 4. OpenAI streaming ─────────────────────────────────

func TestIntegrationOpenAIStream(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	agent := Agent("You are helpful. Be very concise.", WithOpenAI("gpt-4.1-mini"))

	ch := agent.AskStream(context.Background(), "Say hello in 3 words")
	var chunks []string
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("stream error: %v", chunk.Error)
		}
		if chunk.Done {
			break
		}
		chunks = append(chunks, chunk.Text)
	}
	full := strings.Join(chunks, "")
	if full == "" {
		t.Fatal("empty stream")
	}
	t.Logf("Stream (%d chunks): %s", len(chunks), full)
}

// ── 5. Gemini basic chat ────────────────────────────────

func TestIntegrationGeminiChat(t *testing.T) {
	skipIfNoKey(t, "GEMINI_API_KEY")

	agent := Agent("You are helpful. Answer in one sentence.", WithGemini("gemini-2.0-flash"))
	answer, err := agent.Ask(context.Background(), "What is the capital of Japan?")
	if err != nil {
		// Gemini free tier may be rate limited
		if strings.Contains(err.Error(), "429") {
			t.Skipf("Gemini rate limited: %v", err)
		}
		t.Fatalf("Ask failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(answer), "tokyo") {
		t.Errorf("expected 'tokyo', got: %s", answer)
	}
	t.Logf("Gemini response: %s", answer)
}

// ── 6. Gemini with tools ────────────────────────────────

func TestIntegrationGeminiTools(t *testing.T) {
	skipIfNoKey(t, "GEMINI_API_KEY")

	type CalcInput struct {
		A int `json:"a" desc:"First number" required:"true"`
		B int `json:"b" desc:"Second number" required:"true"`
	}

	add := TypedTool("add_numbers", "Add two numbers",
		func(ctx context.Context, in CalcInput) (string, error) {
			return fmt.Sprintf(`{"result":%d}`, in.A+in.B), nil
		})

	agent := Agent("You are a calculator. Use the add_numbers tool.", Tools(add), WithGemini("gemini-2.0-flash"))

	session := NewSession("gemini-tools")
	resp, err := agent.Run(context.Background(), session, "What is 7 + 13?")
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skipf("Gemini rate limited: %v", err)
		}
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(resp.Text, "20") {
		t.Errorf("expected '20' in response, got: %s", resp.Text)
	}
	t.Logf("Gemini tools: %v, response: %s", resp.ToolsCalled, resp.Text)
}

// ── 7. Auto-detect provider ─────────────────────────────

func TestIntegrationAutoDetect(t *testing.T) {
	// Should pick OpenAI or Gemini based on what's in env
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("no API key set")
	}

	agent := Agent("You are helpful. Answer in one word.")
	answer, err := agent.Ask(context.Background(), "Is the sky blue? Answer yes or no.")
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skipf("rate limited: %v", err)
		}
		t.Fatalf("auto-detect failed: %v", err)
	}
	t.Logf("Auto-detect response: %s", answer)
}

// ── 8. Pipeline with real LLM ───────────────────────────

func TestIntegrationPipeline(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	summarizer := Agent("Summarize in one sentence.", WithOpenAI("gpt-4.1-mini"))
	translator := Agent("Translate to French.", WithOpenAI("gpt-4.1-mini"))

	pipeline := summarizer.Then(translator)
	session := NewSession("pipeline-test")
	resp, err := pipeline.Run(context.Background(), session, "Go is a statically typed language designed at Google. It has goroutines for concurrency.")
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if resp.Text == "" {
		t.Fatal("empty pipeline result")
	}
	t.Logf("Pipeline result: %s", resp.Text)
}

// ── 9. Graph workflow with real LLM ─────────────────────

func TestIntegrationGraph(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	classify := Agent("Classify the input. If it's about code, respond with exactly 'CODE'. Otherwise respond with exactly 'OTHER'.", WithOpenAI("gpt-4.1-mini"))
	codeAgent := Agent("You are a coding assistant. Be concise.", WithOpenAI("gpt-4.1-mini"))
	otherAgent := Agent("You are a general assistant. Be concise.", WithOpenAI("gpt-4.1-mini"))

	g := NewGraph()
	g.AddNode("classify", classify)
	g.AddNode("code", codeAgent)
	g.AddNode("other", otherAgent)
	g.SetEntry("classify").SetEnd("code", "other")
	g.AddEdge("classify", "code", func(ctx context.Context, state *GraphState) bool {
		return strings.Contains(strings.ToUpper(state.GetStr("last_response")), "CODE")
	})
	g.AddEdge("classify", "other", nil)

	session := NewSession("graph-test")
	resp, err := g.Run(context.Background(), session, "How do I write a for loop in Go?")
	if err != nil {
		t.Fatalf("graph failed: %v", err)
	}
	if resp.Text == "" {
		t.Fatal("empty graph result")
	}
	t.Logf("Graph routed to code agent: %s", resp.Text[:min(len(resp.Text), 100)])
}

// ── 10. Session summarization with real LLM ─────────────

func TestIntegrationSummarize(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	agent := Agent("You are helpful. Be concise.", WithOpenAI("gpt-4.1-mini"))
	session := NewSession("summarize-test")
	ctx := context.Background()

	// Build up history
	questions := []string{
		"What is Go?",
		"Who created it?",
		"When was it released?",
		"What is its mascot?",
		"What are goroutines?",
		"What is a channel in Go?",
	}
	for _, q := range questions {
		_, err := agent.Run(ctx, session, q)
		if err != nil {
			t.Fatalf("Run failed on %q: %v", q, err)
		}
	}

	historyBefore := len(session.History)
	err := SummarizeSession(ctx, agent, session, 4)
	if err != nil {
		t.Fatalf("SummarizeSession failed: %v", err)
	}
	historyAfter := len(session.History)

	if historyAfter >= historyBefore {
		t.Errorf("history not compressed: before=%d, after=%d", historyBefore, historyAfter)
	}

	// Check summary was stored
	summary := session.Get("_summary")
	if summary == nil {
		t.Error("no summary stored in session state")
	}
	t.Logf("History: %d -> %d messages, summary stored: %v", historyBefore, historyAfter, summary != nil)
}

// ── 11. Metrics and cost tracking ───────────────────────

func TestIntegrationMetrics(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	mc := NewMetricsCollector()
	agent := Agent("Be concise.", WithOpenAI("gpt-4.1-mini"), WithTrace(mc.Trace()))

	_, err := agent.Ask(context.Background(), "Say hi")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	snap := mc.Snapshot()
	if snap.TotalRuns == 0 {
		t.Error("no runs recorded")
	}
	if snap.TotalTokensIn == 0 {
		t.Error("no input tokens recorded")
	}
	if snap.TotalTokensOut == 0 {
		t.Error("no output tokens recorded")
	}
	if snap.AvgLatency == 0 {
		t.Error("no latency recorded")
	}
	t.Logf("Metrics: runs=%d, tokens_in=%d, tokens_out=%d, avg_latency=%s",
		snap.TotalRuns, snap.TotalTokensIn, snap.TotalTokensOut, snap.AvgLatency)
}

// ── 12. Event bus with real run ─────────────────────────

func TestIntegrationEventBus(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	bus := NewEventBus()
	var events []EventType
	bus.OnAll(func(e Event) {
		events = append(events, e.Type)
	})

	agent := Agent("Be concise.", WithOpenAI("gpt-4.1-mini"), WithEvents(bus))
	_, err := agent.Ask(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("no events emitted")
	}

	hasRunStart := false
	hasModelDone := false
	hasRunEnd := false
	for _, e := range events {
		switch e {
		case EventRunStart:
			hasRunStart = true
		case EventModelDone:
			hasModelDone = true
		case EventRunEnd:
			hasRunEnd = true
		}
	}

	if !hasRunStart {
		t.Error("missing EventRunStart")
	}
	if !hasModelDone {
		t.Error("missing EventModelDone")
	}
	if !hasRunEnd {
		t.Error("missing EventRunEnd")
	}
	t.Logf("Events: %v", events)
}

// ── 13. Middleware hooks with real run ───────────────────

func TestIntegrationHooks(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	var hookLog []string

	timer := func(ctx context.Context, a *Core, s *Session, msg string, next NextFunc) (*Response, error) {
		start := time.Now()
		resp, err := next(ctx, a, s, msg)
		hookLog = append(hookLog, fmt.Sprintf("took %s", time.Since(start).Round(time.Millisecond)))
		return resp, err
	}

	agent := Agent("Be concise.", WithOpenAI("gpt-4.1-mini"), WithHooks(timer))
	answer, err := agent.Ask(context.Background(), "Say hi")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}
	if len(hookLog) == 0 {
		t.Error("hook didn't run")
	}
	t.Logf("Hook log: %v, answer: %s", hookLog, answer)
}

// ── 14. RunContext dependency injection ──────────────────

func TestIntegrationRunContext(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	type LookupInput struct {
		Key string `json:"key" desc:"Which key to look up" required:"true"`
	}

	lookup := TypedTool("lookup", "Look up a value from context",
		func(ctx context.Context, in LookupInput) (string, error) {
			rc := RunCtx(ctx)
			if rc == nil {
				return "no context", nil
			}
			return rc.GetStr(in.Key), nil
		})

	agent := Agent("You have a lookup tool. You MUST use the lookup tool to get any user info. NEVER guess. Call lookup with key='user_name' and key='user_plan'.",
		Tools(lookup), WithOpenAI("gpt-4.1-mini"))

	rctx := NewRunContext()
	rctx.Set("user_name", "Erik Svensson")
	rctx.Set("user_plan", "Premium")
	ctx := rctx.WithContext(context.Background())

	session := NewSession("runctx-test")
	resp, err := agent.Run(ctx, session, "What is my name and plan?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(resp.ToolsCalled) == 0 {
		t.Errorf("expected lookup tool to be called, got no tools. Response: %s", resp.Text)
	}
	t.Logf("RunContext tools: %v, response: %s", resp.ToolsCalled, resp.Text)
}

// ── 15. HackerNews tool (free, no auth) ─────────────────

func TestIntegrationHackerNews(t *testing.T) {
	// Test the contrib tool directly against the real API
	// HackerNews API is free, no auth needed
	ctx := context.Background()

	// Import is tools/contrib, but since we're in package agnogo, we test via HTTP directly
	// Actually — let's just verify the HTTP call works
	resp, err := doGet(ctx, "https://hacker-news.firebaseio.com/v0/topstories.json", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("HackerNews API failed: %v", err)
	}
	if len(resp) < 10 {
		t.Errorf("response too short: %s", string(resp))
	}
	t.Logf("HackerNews API works, got %d bytes", len(resp))
}

// ── 16. ArXiv search (free, no auth) ────────────────────

func TestIntegrationArXiv(t *testing.T) {
	ctx := context.Background()
	resp, err := doGet(ctx, "http://export.arxiv.org/api/query?search_query=all:transformer&max_results=1", nil, 10*time.Second)
	if err != nil {
		t.Fatalf("ArXiv API failed: %v", err)
	}
	if !strings.Contains(string(resp), "<entry>") {
		t.Errorf("no entries in ArXiv response")
	}
	t.Logf("ArXiv API works, got %d bytes", len(resp))
}

// ── 17. Hallucination guard with real LLM ───────────────

func TestIntegrationHallucinationGuard(t *testing.T) {
	skipIfNoKey(t, "OPENAI_API_KEY")

	type TimeInput struct {
		Tz string `json:"timezone" desc:"Timezone" required:"true"`
	}

	getTime := TypedTool("get_time", "Get current time",
		func(ctx context.Context, in TimeInput) (string, error) {
			loc, _ := time.LoadLocation(in.Tz)
			if loc == nil {
				loc = time.UTC
			}
			return time.Now().In(loc).Format("2006-01-02 15:04:05"), nil
		})

	agent := Agent("You are a time assistant. ALWAYS use the get_time tool.",
		Tools(getTime), WithOpenAI("gpt-4.1-mini"))

	session := NewSession("hallucination-test")
	resp, err := agent.Run(context.Background(), session, "What time is it in UTC?")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(resp.ToolsCalled) == 0 {
		t.Error("hallucination guard should have forced tool usage, but no tools were called")
	}
	t.Logf("Tools: %v, Response: %s", resp.ToolsCalled, resp.Text[:min(len(resp.Text), 100)])
}

// ── Helper ──────────────────────────────────────────────

func doGet(ctx context.Context, rawURL string, headers map[string]string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
