# agnogo

Go agent framework. Build, serve, and scale AI agents.

243 tests | 15 core tools + 37 contrib integrations | 10 LLM providers | zero external dependencies

```bash
go get github.com/saeedalam/agnogo
```

**New here?** Start with the [5-Minute Quickstart](QUICKSTART.md). Full API reference in [GUIDE.md](GUIDE.md). See what's coming in [ROADMAP.md](ROADMAP.md).

## Quick Start

```go
// One-liner: auto-detects provider from env vars
agent := agnogo.Agent("You are a helpful assistant.")
answer, _ := agent.Ask(context.Background(), "What is the capital of France?")
```

```go
// Explicit provider
agent := agnogo.Agent("You are helpful.", agnogo.WithAnthropic("claude-sonnet-4-5-20250514"))
```

```go
// Full control
agent := agnogo.New(agnogo.Config{
    Model:        openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini"),
    Instructions: "You are a helpful assistant.",
})
session := agnogo.NewSession("user-1")
resp, _ := agent.Run(context.Background(), session, "Hello")
fmt.Println(resp.Text)
```

Both `Agent()` and `New()` return `*agnogo.Core`.

## Why Go

| | Python (Agno) | Go (agnogo) |
|---|---|---|
| Concurrency | asyncio (single thread) | goroutines (real parallelism) |
| Type safety | Runtime errors | Compile-time with `TypedTool[In, Out]` |
| Dependencies | pip install pulls the world | Zero external dependencies |
| HTTP server | Bring your own FastAPI | `agent.Serve(":8080")` built in |
| Pipelines | Manual orchestration | `agent.Then(next).Then(final)` |
| Resilience | DIY | `CircuitBreaker`, `RateLimiter`, `Fallback` |
| Deployment | Container + ASGI server | Single static binary |

## API Overview

| Function | Description |
|----------|-------------|
| `Agent(instructions, opts...)` | Smart constructor with auto-detection |
| `New(Config)` | Full-control constructor |
| `Ask(ctx, msg)` | One-shot question |
| `AskStream(ctx, msg)` | One-shot streaming |
| `AskStructured[T](ctx, agent, msg, &out)` | Parse response into struct |
| `Run(ctx, session, msg)` | Full run with session |
| `RunWithStorage(ctx, sessionID, msg)` | Run with persistent storage |
| `Serve(addr, opts...)` | HTTP server |
| `Handler(opts...)` | `http.Handler` for embedding |
| `Then(next)` | Sequential pipeline |
| `All(agents...)` | Parallel fan-out |
| `Race(agents...)` | First response wins |
| `Map(ctx, agent, inputs, n)` | Parallel map |
| `Batch(ctx, agent, tasks, n)` | Batch processing |
| `Benchmark(ctx, agent, cfg)` | Latency/throughput benchmark |
| `Explain(agent)` | Print config summary |
| `Validate(agent)` | Check config for issues |
| `NewGraph()` | Graph workflow with conditional edges |
| `NewRunContext()` | Dependency injection via context |
| `NewEventBus()` | Pub/sub event system |
| `WithHooks(h)` | Middleware hook chain |
| `WithSummarize(n)` | Auto-summarize old messages |

## Provider Selection

Auto-detection picks the first available key from environment:

```go
agent := agnogo.Agent("instructions")                          // auto-detect
agent := agnogo.Agent("instructions", agnogo.WithOpenAI())     // default model
agent := agnogo.Agent("instructions", agnogo.WithAnthropic("claude-sonnet-4-5-20250514"))
```

| Option | Env Var | Default Model |
|--------|---------|---------------|
| `WithOpenAI()` | `OPENAI_API_KEY` | gpt-4.1-mini |
| `WithAnthropic()` | `ANTHROPIC_API_KEY` | claude-sonnet-4-5-20250514 |
| `WithGemini()` | `GEMINI_API_KEY` | gemini-2.5-flash |
| `WithGroq()` | `GROQ_API_KEY` | llama-3.3-70b-versatile |
| `WithDeepSeek()` | `DEEPSEEK_API_KEY` | deepseek-chat |
| `WithMistral()` | `MISTRAL_API_KEY` | mistral-large-latest |
| `WithTogether()` | `TOGETHER_API_KEY` | -- |
| `WithPerplexity()` | `PERPLEXITY_API_KEY` | -- |
| `WithGrok()` | `XAI_API_KEY` | grok-3 |
| `WithOllama()` | -- | llama3.1 |

Other options: `WithTools(...)`, `WithStorage(s)`, `WithKnowledge(k)`, `WithMemory()`, `WithDebug()`, `WithMaxLoops(n)`, `WithReasoning()`, `WithTrace(t)`, `WithEvents(bus)`, `WithHooks(h)`, `WithSummarize(n)`, `Unsafe()`.

## Tools

### Core (maintained, production-grade)

```go
import "github.com/saeedalam/agnogo/tools"
```

| Tool | What it does |
|------|-------------|
| `Calculator()` | Expression parser (precedence, functions, parentheses) |
| `Shell()` | Execute commands (allowlist, timeout, stdout/stderr) |
| `HTTP()` | Full HTTP client (headers, auth, configurable limits) |
| `File(baseDir)` | Read/write/list/append (atomic writes, symlink protection) |
| `SQL(db, readOnly)` | Queries with pagination, parameterized, schema listing |
| `JSON()` | Parse, format, validate, merge, JSONPath queries |
| `CSV()` | CSV to JSON conversion |
| `WebBrowser()` | Fetch URLs, extract links, HTML stripping |
| `DuckDuckGo()` | Web search |
| `Wikipedia()` | Article summaries |
| `GitHub(token)` | Repos, issues, PRs (pagination, rate limit aware) |
| `Slack(token)` | Messages, channels, threads, reactions |
| `Email(host, port, user, pass, from)` | SMTP email |
| `Docker()` | Containers, images, build (resource limits) |
| `Regex()` | Match, replace, extract with named groups |
| `Hash()` | SHA256, SHA512, MD5, HMAC |
| `TimeTool()` | Current time, timezone conversion, date math |

```go
agent := agnogo.Agent("You are helpful.",
    agnogo.Tools(tools.Calculator()...), agnogo.Tools(tools.Shell()...),
)
```

### Contrib (community, best-effort)

```go
import "github.com/saeedalam/agnogo/tools/contrib"
```

37 API integrations: Discord, Telegram, WhatsApp, Jira, Notion, Linear, GitLab, Reddit, YouTube, HackerNews, ArXiv, Giphy, Unsplash, OpenWeather, YFinance, Google Maps/Calendar/Sheets, and more. See [tools/contrib/README.md](tools/contrib/README.md).

```go
agent := agnogo.Agent("You are helpful.",
    agnogo.Tools(contrib.HackerNews()...), agnogo.Tools(contrib.OpenWeather(apiKey)...),
)
```

APIs change -- if a contrib tool breaks, PRs welcome.

## Typed Tools

```go
type WeatherIn struct {
    City string `json:"city" desc:"City name" required:"true"`
}
type WeatherOut struct {
    Temp float64 `json:"temperature"`
    Desc string  `json:"description"`
}

tool := agnogo.TypedTool[WeatherIn, WeatherOut]("weather", "Get weather",
    func(ctx context.Context, in WeatherIn) (WeatherOut, error) {
        return WeatherOut{Temp: 22.5, Desc: "Sunny in " + in.City}, nil
    },
)
agent := agnogo.Agent("Weather bot.", agnogo.WithTools(tool))
```

Struct tags drive the schema: `json` (name), `desc` (description), `required`, `enum`.

## HTTP Server

```go
agent.Serve(":8080",
    agnogo.WithCORS("*"),
    agnogo.WithAuth("secret-token"),
    agnogo.WithMaxConcurrent(100),
    agnogo.WithMaxBodySize(1<<20),
    agnogo.WithTimeouts(5*time.Second, 30*time.Second),
)
```

Endpoints: `POST /ask`, `POST /stream` (SSE), `GET /health`, `GET /tools`.

Embed in existing servers:

```go
mux.Handle("/agent/", http.StripPrefix("/agent", agent.Handler()))
```

## Pipelines

```go
// Sequential: output of each becomes input of next
resp, _ := extract.Then(summarize).Then(translate).Run(ctx, session, input)

// Parallel fan-out
resp, _ := agnogo.All(weather, news, stocks).Run(ctx, session, "Morning briefing")

// First response wins, others cancelled
resp, _ := agnogo.Race(gpt4, claude, gemini).Run(ctx, session, "Quick answer")

// Parallel map with bounded concurrency
results := agnogo.Map(ctx, agent, []string{"task1", "task2", "task3"}, 3)
```

## Resilience

```go
provider := openai.New(key, "gpt-4.1-mini")

safe := agnogo.Fallback(provider, anthropic.New(antKey, "claude-sonnet-4-5-20250514"))
safe = agnogo.CircuitBreaker(safe, agnogo.WithFailureThreshold(5))
safe = agnogo.RateLimiter(safe, 60)
safe = agnogo.TimeoutProvider(safe, 30*time.Second)

// Or try multiple in order
safe = agnogo.MultiProvider(provider, anthropicProvider, geminiProvider)
```

All wrappers implement `ModelProvider` and compose freely. Use `agnogo.CloseProvider(p)` for cleanup.

## Error Handling

```go
resp, err := agent.Ask(ctx, "Hello")
if err != nil {
    if agnogo.IsRetryable(err) { /* safe to retry */ }
    if agnogo.IsRateLimited(err) {
        time.Sleep(agnogo.RetryAfter(err))
    }
    var pe *agnogo.ProviderError
    if errors.As(err, &pe) {
        fmt.Println(pe.Provider, pe.StatusCode, pe.Message)
    }
}
```

`IsRetryable`, `IsRateLimited`, and `RetryAfter` are package-level functions, not methods.

## Observability

```go
metrics := agnogo.NewMetricsCollector()
agent := agnogo.New(agnogo.Config{
    Model: model, Instructions: "You are helpful.",
    Trace: metrics.Trace(),
})

snap := metrics.Snapshot()
fmt.Printf("Runs: %d, Tokens: %d/%d, Avg: %s\n",
    snap.TotalRuns, snap.TotalTokensIn, snap.TotalTokensOut, snap.AvgLatency)

// Expose as HTTP endpoint
http.Handle("/metrics", metrics.Handler())
```

```go
agnogo.Explain(agent)              // prints config summary
issues := agnogo.Validate(agent)   // returns []ValidationError
```

## Batch Processing

```go
// One-shot batch
tasks := []agnogo.WorkerTask{
    {ID: "1", Message: "Summarize doc A"},
    {ID: "2", Message: "Summarize doc B"},
}
results := agnogo.Batch(ctx, agent, tasks, 4)

// Long-lived worker pool
pool := agnogo.NewWorkerPool(agent, 4)
pool.Start(ctx)
pool.Submit(agnogo.WorkerTask{ID: "1", Message: "Hello"})
result := <-pool.Results()
pool.Stop()

// Benchmark
result := agnogo.Benchmark(ctx, agent, agnogo.BenchmarkConfig{
    Prompts: []string{"Hello", "What is Go?"},
    Concurrency: 3, Warmup: 1,
})
```

## Teams

```go
team := agnogo.NewTeam(agnogo.TeamConfig{Model: model})
team.Agent("booking", bookingAgent)
team.Agent("support", supportAgent)
resp, _ := team.Run(ctx, session, "I want to book a haircut")
```

## Workflows

```go
wf := agnogo.Sequential(
    agnogo.Step("extract", extractAgent),
    agnogo.Step("validate", validateAgent),
)

wf := agnogo.Parallel(
    agnogo.Step("weather", weatherAgent),
    agnogo.Step("news", newsAgent),
)

wf := agnogo.Loop(agent, func(resp *agnogo.Response, i int) bool {
    return strings.Contains(resp.Text, "DONE") || i >= 5
})

wf := agnogo.Condition(
    func(ctx context.Context, input string) bool { return isUrgent(input) },
    urgentWorkflow, normalWorkflow,
)

wf := agnogo.Route(
    func(ctx context.Context, input string) string { return classify(input) },
    map[string]agnogo.Workflow{"refund": refundWf, "general": generalWf},
)
```

## Knowledge (RAG)

```go
agent := agnogo.New(agnogo.Config{
    Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
        return myDB.Search(ctx, query, limit)
    }),
})
```

Vector DB backends: `knowledge/pgvector`, `knowledge/qdrant`, `knowledge/chromadb`, `vectordb/pinecone`.

## Memory

```go
// Pattern-based (zero LLM calls)
agent := agnogo.New(agnogo.Config{AutoMemory: true})
// "My name is Erik" -> session.GetMemory("name") == "Erik"

// LLM-based (richer extraction)
agent := agnogo.New(agnogo.Config{
    Memory: &agnogo.LLMMemory{Model: model, Fields: []string{"name", "email"}},
})
```

## Storage

```go
import "github.com/saeedalam/agnogo/storage/postgres" // or sqlite, redis, mysql

store := postgres.New(pool, postgres.Config{Table: "sessions"})
agent := agnogo.New(agnogo.Config{Storage: store})
resp, _ := agent.RunWithStorage(ctx, "session-123", "hello")
```

Backends: PostgreSQL, SQLite, Redis, MySQL, in-memory (`agnogo.NewMemoryStorage()`).

## Streaming

```go
// Token-level SSE (real provider streaming)
for event := range agent.RunStreamReal(ctx, session, "Tell me a story") {
    fmt.Print(event.Text)
}

// Word-level fallback (any provider)
for chunk := range agent.RunStream(ctx, session, "Tell me a story") {
    fmt.Print(chunk.Text)
}

// One-shot (no session needed)
for chunk := range agent.AskStream(ctx, "Tell me a story") {
    fmt.Print(chunk.Text)
}
```

## Guardrails

```go
agent.InputGuardrail("no-spam", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if isSpam(msg) { return errors.New("blocked") }
    return nil
})

agent.OutputGuardrail("no-pii", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if containsPII(msg) { return errors.New("PII detected") }
    return nil
})

// Hallucination detection: retries when LLM fabricates instead of using tools
agent.HallucinationGuard()
```

### Production Safety (`Reliable()`)

One-liner that enables cost budgets, PII detection, hallucination guard, tool validation, and confidence scoring:

```go
agent := agnogo.Agent("...", agnogo.Reliable())
```

Every component is pluggable — bring your own implementations:

```go
agent := agnogo.Agent("...", agnogo.Reliable(
    agnogo.WithCustomHallucination(myDetector),      // your hallucination checker
    agnogo.WithCustomPII(myGDPRLib),                 // your PII scanner
    agnogo.WithCustomCost(myBillingSystem),           // your cost tracker
    agnogo.WithCustomToolValidator(myValidator),      // your tool output checker
    agnogo.WithCustomConfidence(myScorer),            // your confidence scorer
    agnogo.WithReliableBudget(0.50, 5.00),           // custom budget limits
    agnogo.WithReliableConfidenceThreshold(0.7),     // custom threshold
))
```

Interfaces: `HallucinationChecker`, `PIIScanner`, `CostChecker`, `ToolOutputValidator`, `ConfidenceScorer`.

## MCP (Model Context Protocol)

Connect to any MCP server and use its tools. Zero external dependencies.

```go
import "github.com/saeedalam/agnogo/mcp"

// Stdio transport (subprocess)
tools, _ := mcp.Connect(ctx, "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp")
defer tools.Close()

agent := agnogo.Agent("...", agnogo.Tools(tools.ToolDefs()...))
```

## Eval Framework

Automated agent quality testing with assertions:

```go
eval := agnogo.NewEval(agent)
eval.Add("greeting", "Say hello", agnogo.Contains("hello"))
eval.Add("math", "What is 2+2?", agnogo.Contains("4"))
eval.Add("safety", "Harmful request", agnogo.NotContains("harmful content"))
eval.WithConcurrency(3) // run in parallel

report := eval.Run(ctx)
report.Print()          // human-readable summary
fmt.Println(report.JSON()) // machine-readable
```

Assertions: `Contains`, `NotContains`, `Exact`, `MatchesRegex`, `LengthBetween`, `Custom`.

## OpenTelemetry Export

Ship agent metrics to Datadog, Grafana, or any OTLP backend:

```go
import "github.com/saeedalam/agnogo/otel"

exporter := otel.NewExporter("http://localhost:4318/v1/metrics",
    otel.WithInterval(30 * time.Second),
    otel.WithServiceName("my-agent"),
)
defer exporter.Stop()

agent := agnogo.Agent("...", agnogo.WithTrace(exporter.Trace()))
```

Exports: runs, model calls, tool calls, tokens, errors, latency, guardrail blocks, per-tool counts.

## Graph Workflows

```go
g := agnogo.NewGraph()
g.AddNode("classify", classifyAgent).AddNode("refund", refundAgent).AddNode("support", supportAgent)
g.SetEntry("classify").SetEnd("refund", "support")
g.AddEdge("classify", "refund", func(ctx context.Context, state *agnogo.GraphState) bool {
    return strings.Contains(state.GetStr("last_response"), "REFUND")
})
g.AddEdge("classify", "support", nil) // default edge
resp, _ := g.Run(ctx, session, "I want a refund")
```

## Run Context (Dependency Injection)

```go
rctx := agnogo.NewRunContext()
rctx.Set("user_id", "u-123")
ctx := rctx.WithContext(context.Background())
// Inside any tool: rc := agnogo.RunCtx(ctx); rc.GetStr("user_id")
```

## Event Bus

```go
bus := agnogo.NewEventBus()
bus.On(agnogo.EventRunStart, func(e agnogo.Event) { log.Println("started") })
bus.On(agnogo.EventModelDone, func(e agnogo.Event) { log.Println("model done:", e.Data["duration"]) })
agent := agnogo.Agent("...", agnogo.WithEvents(bus))
```

## Middleware Hooks

```go
timer := func(ctx context.Context, a *agnogo.Core, s *agnogo.Session, msg string, next agnogo.NextFunc) (*agnogo.Response, error) {
    start := time.Now()
    resp, err := next(ctx, a, s, msg)
    log.Printf("took %s", time.Since(start))
    return resp, err
}
agent := agnogo.Agent("...", agnogo.WithHooks(timer))
```

## Session Summarization

```go
agent := agnogo.Agent("...", agnogo.WithSummarize(30)) // summarize after 30 messages
```

## Architecture

```
agnogo/
  agent.go             Core struct + run loop
  smart.go             Agent() constructor + auto-detection
  ask.go               Ask, AskStream, AskStructured
  typed_tool.go        TypedTool[In, Out]
  serve.go             HTTP server (Serve, Handler)
  pipeline.go          Then, All, Race, Map
  resilience.go        Fallback, CircuitBreaker, RateLimiter
  observe.go           MetricsCollector, Explain, Validate
  worker_pool.go       WorkerPool, Batch
  benchmark.go         Benchmark
  errors.go            ProviderError, ToolError
  session.go           Session state + memory + history
  knowledge.go         Knowledge interface + RAG injection
  memory.go            Pattern + LLM memory extraction
  guardrail.go         Input/output guardrails
  hallucination.go     HallucinationGuard
  graph.go             Graph workflows with conditional edges
  runctx.go            RunContext dependency injection
  events.go            EventBus pub/sub
  hook.go              Middleware hook chain
  summarize.go         Session summarization
  team.go              Multi-agent teams + routing
  workflow.go          Sequential, Parallel, Loop, Condition, Route
  streaming.go         Token-level SSE + fallback
  providers/           10 LLM providers
  tools/               35 built-in tools
  knowledge/           pgvector, Qdrant, ChromaDB
  vectordb/            Pinecone
  storage/             Postgres, SQLite, Redis, MySQL
```

## License

MIT
