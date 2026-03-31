# agnogo

The Go agent framework that does what Python frameworks wish they could.

Build AI agents with type-safe tools, pipelines, resilience patterns, built-in HTTP servers, and real concurrency -- not async/await pretending to be parallel.

**Zero external dependencies** | 10 LLM providers | 16 built-in tools | 4 vector DBs | 5 storage backends | 133 tests

[![Go Reference](https://pkg.go.dev/badge/github.com/saeedalam/agnogo.svg)](https://pkg.go.dev/github.com/saeedalam/agnogo)

## Install

```bash
go get github.com/saeedalam/agnogo
```

## Quick Start

### The New Way -- one line to an agent

```go
package main

import (
    "context"
    "fmt"

    _ "github.com/saeedalam/agnogo/autodetect" // auto-register providers from env
    "github.com/saeedalam/agnogo"
)

func main() {
    agent := agnogo.Agent("You are a helpful assistant. Be concise.")

    answer, _ := agent.Ask(context.Background(), "What is the capital of France?")
    fmt.Println(answer)
}
```

`Agent()` auto-detects your provider from environment variables (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.), applies safe defaults (retry, history trimming, hallucination guard), and returns a ready-to-use `*Core`.

### Power-User Way -- full control

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/saeedalam/agnogo"
    "github.com/saeedalam/agnogo/providers/openai"
)

func main() {
    agent := agnogo.New(agnogo.Config{
        Model:        openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini"),
        Instructions: "You are a helpful assistant. Be concise.",
    })

    session := agnogo.NewSession("user-1")
    resp, _ := agent.Run(context.Background(), session, "What is the capital of France?")
    fmt.Println(resp.Text)
}
```

Both `Agent()` and `New()` return `*agnogo.Core`.

## What's Different from Python

| | Python (Agno) | Go (agnogo) |
|---|---|---|
| Concurrency | asyncio (single thread) | goroutines (real parallelism) |
| Type safety | Runtime type errors | Compile-time with `TypedTool[In, Out]` |
| Dependencies | pip install pulls the world | Zero external dependencies |
| HTTP server | Bring your own FastAPI | `agent.Serve(":8080")` built in |
| Pipelines | Manual orchestration | `agent.Then(agent2).Then(agent3)` |
| Fan-out | asyncio.gather | `All(a1, a2, a3)` with real goroutines |
| Resilience | DIY or third-party | `CircuitBreaker`, `RateLimiter`, `Fallback` built in |
| Observability | External tools | `NewMetricsCollector()` with cost tracking |
| Deployment | Container + ASGI server | Single static binary |
| Batch | Loop it yourself | `Batch(ctx, agent, tasks, concurrency)` |
| Benchmarking | Roll your own | `Benchmark(ctx, agent, cfg)` built in |

## Features

| Feature | Description |
|---------|-------------|
| **Tools** | Register any Go function as a tool |
| **Typed Tools** | `TypedTool[In, Out]` -- compile-time type-safe tool definitions |
| **Ask / AskStream** | One-shot calls, no session boilerplate |
| **Structured Output** | `AskStructured[T]` -- parse responses into Go structs |
| **HTTP Server** | `agent.Serve(":8080")` -- /ask, /stream, /health, /tools |
| **Pipelines** | `agent.Then(agent2)` -- sequential chaining |
| **Fan-Out** | `All(a1, a2, a3)` -- parallel with goroutines |
| **Race** | `Race(a1, a2)` -- first response wins |
| **Map** | `Map(ctx, agent, inputs, n)` -- parallel map over inputs |
| **Fallback** | `Fallback(primary, secondary)` -- provider failover |
| **Circuit Breaker** | `CircuitBreaker(provider)` -- stop hammering failed providers |
| **Rate Limiter** | `RateLimiter(provider, rpm)` -- token bucket rate limiting |
| **Multi-Provider** | `MultiProvider(p1, p2, p3)` -- try in order |
| **Timeout** | `TimeoutProvider(provider, dur)` -- per-request timeout |
| **Observability** | `NewMetricsCollector()` -- runs, tokens, latency, cost |
| **Batch** | `Batch(ctx, agent, tasks, n)` -- one-shot batch processing |
| **Worker Pool** | `NewWorkerPool(agent, n)` -- long-lived batch processor |
| **Benchmark** | `Benchmark(ctx, agent, cfg)` -- latency percentiles, throughput |
| **Middleware** | `AgentMiddleware(agent)` -- inject agent into HTTP handlers |
| **Explain / Validate** | `Explain(agent)`, `Validate(agent)` -- inspect config |
| **Knowledge** | Auto RAG injection for questions |
| **Memory** | Learn facts from conversations (pattern + LLM) |
| **Storage** | Persist sessions to Postgres, SQLite, Redis, MySQL |
| **Guardrails** | Block bad input/output |
| **Teams** | Route to sub-agents by intent |
| **Workflows** | Sequential, Parallel, Loop, Condition, Router |
| **Streaming** | Token-level SSE + word-level fallback |
| **Reasoning** | Multi-step reasoning with confidence scores |
| **Human-in-the-loop** | Require approval before actions |
| **CLI** | Interactive terminal with commands |
| **Debug & Tracing** | Two-level debug, 8 trace hooks |

## One-Liner Examples

```go
// One-shot question
answer, _ := agent.Ask(ctx, "Summarize this document")

// Streaming
for chunk := range agent.AskStream(ctx, "Tell me a story") {
    fmt.Print(chunk.Text)
}

// Structured output
var result struct {
    Name  string `json:"name"`
    Score int    `json:"score"`
}
agnogo.AskStructured(ctx, agent, "Rate Go vs Python", &result)

// HTTP server in one line
agent.Serve(":8080")

// Pipeline
resp, _ := extract.Then(validate).Then(transform).Run(ctx, session, input)

// Parallel fan-out
resp, _ := agnogo.All(weather, news, stocks).Run(ctx, session, "Morning briefing")

// First response wins
resp, _ := agnogo.Race(fast, slow, fallback).Run(ctx, session, "Quick answer")

// Parallel map
results := agnogo.Map(ctx, agent, []string{"task1", "task2", "task3"}, 3)

// Explain config
agnogo.Explain(agent)

// Validate config
issues := agnogo.Validate(agent)
```

## Typed Tools

No more `map[string]string`. Define tools with real Go types:

```go
type WeatherInput struct {
    City string `json:"city" desc:"City name" required:"true"`
    Unit string `json:"unit" desc:"Temperature unit" enum:"C,F"`
}

type WeatherOutput struct {
    Temp float64 `json:"temperature"`
    Desc string  `json:"description"`
}

func getWeather(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
    return WeatherOutput{Temp: 22.5, Desc: "Sunny in " + in.City}, nil
}

tool := agnogo.TypedTool[WeatherInput, WeatherOutput]("weather", "Get weather", getWeather)

agent := agnogo.Agent("You are a weather bot.", agnogo.WithTools(tool))
```

Struct tags drive parameter schemas: `json` for the name, `desc` for the description, `required` for required fields, `enum` for allowed values. Type mismatches are caught at compile time.

## HTTP Server

Turn any agent into an API with one call:

```go
agent := agnogo.Agent("You are a helpful assistant.")
agent.Serve(":8080", agnogo.WithCORS("*"), agnogo.WithAuth("secret-token"))
```

Endpoints:
- `POST /ask` -- `{"message": "..."}` returns `{"text": "..."}`
- `POST /stream` -- SSE streaming response
- `GET /health` -- `{"status": "ok", "tools": 3, "active_runs": 0}`
- `GET /tools` -- list registered tools

For embedding in existing servers:

```go
mux := http.NewServeMux()
mux.Handle("/agent/", http.StripPrefix("/agent", agent.Handler()))
```

Or use the middleware to inject an agent into any handler chain:

```go
mux.Handle("/chat", agnogo.AgentMiddleware(agent)(chatHandler))

// Inside the handler:
agent := agnogo.AgentFromContext(r.Context())
```

## Pipelines and Concurrency

### Sequential Pipeline

Each agent's output becomes the next agent's input:

```go
extract := agnogo.Agent("Extract key facts from text.")
summarize := agnogo.Agent("Summarize the facts into bullet points.")
translate := agnogo.Agent("Translate to Swedish.")

resp, _ := extract.Then(summarize).Then(translate).Run(ctx, session, longDocument)
```

### Parallel Fan-Out

Run multiple agents on the same input simultaneously:

```go
resp, _ := agnogo.All(weather, news, calendar).
    WithMerge(func(outputs []string) string {
        return "BRIEFING:\n" + strings.Join(outputs, "\n---\n")
    }).
    Run(ctx, session, "Morning briefing for Stockholm")
```

### Race

First non-error response wins, others are cancelled:

```go
resp, _ := agnogo.Race(gpt4, claude, gemini).Run(ctx, session, "Quick question")
```

### Map

Process a list of inputs in parallel with bounded concurrency:

```go
results := agnogo.Map(ctx, agent, []string{
    "Summarize doc 1",
    "Summarize doc 2",
    "Summarize doc 3",
}, 3) // 3 concurrent workers
```

## Resilience

Wrap any provider with production-grade resilience patterns:

```go
provider := openai.New(key, "gpt-4.1-mini")

// Failover: try OpenAI, fall back to Anthropic
safe := agnogo.Fallback(provider, anthropic.New(antKey, "claude-sonnet-4-5-20250514"))

// Circuit breaker: stop calling after 5 consecutive failures
safe = agnogo.CircuitBreaker(safe, agnogo.WithFailureThreshold(5))

// Rate limit: max 60 requests per minute
safe = agnogo.RateLimiter(safe, 60)

// Per-request timeout
safe = agnogo.TimeoutProvider(safe, 30*time.Second)

// Try multiple providers in order
safe = agnogo.MultiProvider(provider, anthropicProvider, geminiProvider)
```

All resilience wrappers implement `ModelProvider`, so they compose freely.

## Observability

```go
metrics := agnogo.NewMetricsCollector()

agent := agnogo.New(agnogo.Config{
    Model:        model,
    Instructions: "You are helpful.",
    Trace:        metrics.Trace(), // auto-wires all hooks
})

// After some runs...
snap := metrics.Snapshot()
fmt.Printf("Runs: %d, Tokens: %d in / %d out, Avg latency: %s\n",
    snap.TotalRuns, snap.TotalTokensIn, snap.TotalTokensOut, snap.AvgLatency)

// Cost tracking included
cost := agnogo.NewCostTracker()
cost.Estimate("gpt-4o", usage) // returns USD estimate
```

`Explain` and `Validate` help debug configuration:

```go
agnogo.Explain(agent)   // prints human-readable config summary
issues := agnogo.Validate(agent) // returns []ValidationError
```

## Batch Processing

### One-Shot Batch

```go
tasks := []agnogo.WorkerTask{
    {ID: "1", Message: "Summarize doc A"},
    {ID: "2", Message: "Summarize doc B"},
    {ID: "3", Message: "Summarize doc C"},
}
results := agnogo.Batch(ctx, agent, tasks, 4) // 4 concurrent workers
for _, r := range results {
    fmt.Printf("Task %s: %s (took %s)\n", r.TaskID, r.Response.Text, r.Duration)
}
```

### Long-Lived Worker Pool

```go
pool := agnogo.NewWorkerPool(agent, 4)
pool.Start(ctx)

pool.Submit(agnogo.WorkerTask{ID: "1", Message: "Hello"})
result := <-pool.Results()

pool.Stop()
```

### Benchmarking

```go
result := agnogo.Benchmark(ctx, agent, agnogo.BenchmarkConfig{
    Prompts:     []string{"Hello", "What is Go?", "Explain concurrency"},
    Concurrency: 3,
    Warmup:      1,
})
fmt.Println(result)
// Benchmark: 3 requests, 0 errors
//   Throughput: 2.50 req/s
//   Latency:    avg=400ms p50=380ms p95=520ms p99=520ms
```

## Providers (10)

```go
import (
    "github.com/saeedalam/agnogo/providers/openai"
    "github.com/saeedalam/agnogo/providers/anthropic"
    "github.com/saeedalam/agnogo/providers/gemini"
    "github.com/saeedalam/agnogo/providers/ollama"
    "github.com/saeedalam/agnogo/providers/grok"
    "github.com/saeedalam/agnogo/providers/deepseek"
    "github.com/saeedalam/agnogo/providers/groq"
    "github.com/saeedalam/agnogo/providers/together"
    "github.com/saeedalam/agnogo/providers/mistral"
    "github.com/saeedalam/agnogo/providers/perplexity"
)

openai.New("sk-...", "gpt-4.1-mini")                         // OpenAI
anthropic.New("sk-ant-...", "claude-sonnet-4-5-20250514")     // Claude
gemini.New("AIza...", "gemini-2.5-flash")                     // Google Gemini
ollama.New("llama3.1")                                        // Local Ollama
grok.New("xai-...", "grok-3")                                 // xAI Grok
deepseek.New("sk-...", "deepseek-chat")                       // DeepSeek
groq.New("gsk-...", "llama-3.3-70b-versatile")                // Groq
together.New("...", "meta-llama/Llama-3.3-70B-Instruct-Turbo") // Together
mistral.New("...", "mistral-large-latest")                    // Mistral
perplexity.New("pplx-...", "llama-3.1-sonar-large-128k-online") // Perplexity
```

All providers use OpenAI-compatible APIs except Anthropic (native Claude format) and Gemini (native function calling format). Custom base URLs supported via `WithBaseURL()`.

Or skip explicit imports and let auto-detection handle it:

```go
import _ "github.com/saeedalam/agnogo/autodetect"

agent := agnogo.Agent("You are helpful.") // picks provider from env vars
```

## Tools

```go
// Register individually
agent.Tool("search", "Search the web", agnogo.Params{
    "query": {Type: "string", Desc: "Search query", Required: true},
}, searchFn)

// Bulk add
agent.AddTools(
    agnogo.ToolDef{Name: "t1", Desc: "Tool 1", Fn: fn1},
    agnogo.ToolDef{Name: "t2", Desc: "Tool 2", Fn: fn2},
)

// Replace all tools
agent.SetTools(agnogo.ToolDef{Name: "only", Desc: "Only tool", Fn: fn})

// Clear all
agent.ClearTools()
```

### Built-in Tools (16)

```go
import "github.com/saeedalam/agnogo/tools"

agent.AddTools(
    tools.Calculator(),       // Math operations
    tools.Shell(),            // Execute shell commands
    tools.HTTPRequest(),      // HTTP GET/POST/PUT/DELETE
    tools.FileRead(),         // Read files
    tools.FileWrite(),        // Write files
    tools.FileList(),         // List directory
    tools.WebBrowser(),       // Fetch URL content
    tools.DuckDuckGo(),       // Web search
    tools.Wikipedia(),        // Wikipedia lookup
    tools.Email("smtp..", 587, "user", "pass"), // Send email
    tools.SQL(db),            // SQL queries
    tools.JSONParse(),        // Parse JSON
    tools.JSONFormat(),       // Format JSON
    tools.CSVRead(),          // Read CSV files
    tools.Slack("xoxb-..."), // Post to Slack
    tools.GitHub("ghp_..."), // GitHub API
    tools.Docker(),           // Docker management
    tools.GoogleSearch("...", "..."), // Google Custom Search
)
```

## Knowledge (RAG)

```go
// Simple function
agent := agnogo.New(agnogo.Config{
    Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
        return myDB.Search(ctx, query, limit)
    }),
})

// Vector database backends
import (
    "github.com/saeedalam/agnogo/knowledge/pgvector"
    "github.com/saeedalam/agnogo/knowledge/qdrant"
    "github.com/saeedalam/agnogo/knowledge/chromadb"
    "github.com/saeedalam/agnogo/vectordb/pinecone"
)

pgvector.New(pool, pgvector.Config{Table: "chunks", EmbedFunc: embedFn})
qdrant.New("http://localhost:6333", "collection", embedFn)
chromadb.New("http://localhost:8000", "collection")
pinecone.New("https://idx-....pinecone.io", "api-key", embedFn)
```

Knowledge is automatically injected when the user asks a question (detected via heuristics for English + Swedish).

## Memory

```go
// Pattern-based (zero LLM calls)
agent := agnogo.New(agnogo.Config{AutoMemory: true})
// "My name is Erik" -> session.GetMemory("name") == "Erik"
// "anna@test.com"   -> session.GetMemory("email") == "anna@test.com"

// LLM-based (richer extraction)
agent := agnogo.New(agnogo.Config{
    MemoryExtractor: agnogo.NewLLMMemory(model),
})
```

## Storage

```go
import (
    "github.com/saeedalam/agnogo/storage/postgres"
    "github.com/saeedalam/agnogo/storage/sqlite"
    "github.com/saeedalam/agnogo/storage/redis"
    "github.com/saeedalam/agnogo/storage/mysql"
)

// PostgreSQL
store := postgres.New(pool, postgres.Config{Table: "sessions"})

// SQLite
store := sqlite.New("agents.db")

// Redis (zero-dep, raw RESP protocol)
store := redis.New("localhost:6379", redis.Config{TTL: 24 * time.Hour})

// MySQL
store := mysql.New(db, mysql.Config{Table: "sessions"})

// In-memory (testing)
store := agnogo.NewMemoryStorage()

// Use with agent
agent := agnogo.New(agnogo.Config{Storage: store})
resp, _ := agent.RunWithStorage(ctx, "session-123", "hello")
```

## Teams

```go
team := agnogo.NewTeam(agnogo.TeamConfig{
    Model: model, // LLM picks which agent to route to
})
team.Agent("booking", bookingAgent)
team.Agent("support", supportAgent)
resp, _ := team.Run(ctx, session, "I want to book a haircut")

// Or with custom routing function
team := agnogo.NewTeam(agnogo.TeamConfig{
    RouterFunc: func(ctx context.Context, msg string, agents []string) (string, error) {
        if strings.Contains(msg, "book") { return "booking", nil }
        return "support", nil
    },
})
```

## Workflows

```go
// Sequential: extract -> validate -> book
wf := agnogo.Sequential(
    agnogo.Step("extract", extractAgent),
    agnogo.Step("validate", validateAgent),
    agnogo.Step("book", bookAgent),
)

// Parallel: fetch weather + news simultaneously
wf := agnogo.Parallel(
    agnogo.Step("weather", weatherAgent),
    agnogo.Step("news", newsAgent),
)

// Loop: refine until done
wf := agnogo.Loop(agent, func(resp *agnogo.Response, i int) bool {
    return strings.Contains(resp.Text, "DONE") || i >= 5
}).WithMaxIterations(10)

// Condition: branch based on input
wf := agnogo.Condition(
    func(ctx context.Context, input string) bool { return isUrgent(input) },
    urgentWorkflow,
    normalWorkflow,
)

// Router: dynamic routing to named workflows
wf := agnogo.Route(
    func(ctx context.Context, input string) string { return classify(input) },
    map[string]agnogo.Workflow{
        "refund":  refundWorkflow,
        "general": generalWorkflow,
    },
)
```

## Streaming

```go
// Token-level (real SSE from provider)
ch := agent.RunStreamReal(ctx, session, "Tell me a story")
for event := range ch {
    if event.Error != nil { break }
    fmt.Print(event.Text)
}

// Word-level fallback (any provider)
ch := agent.RunStream(ctx, session, "Tell me a story")
for chunk := range ch {
    fmt.Print(chunk.Text)
}

// One-shot streaming (no session needed)
for chunk := range agent.AskStream(ctx, "Tell me a story") {
    fmt.Print(chunk.Text)
}
```

## Reasoning

```go
agent := agnogo.New(agnogo.Config{
    Reasoning: agnogo.ReasoningConfig{
        Enabled:  true,
        Model:    reasoningModel, // optional separate model
        MinSteps: 3,
        MaxSteps: 6,
    },
})
// Agent runs multi-step reasoning: analyze -> strategize -> plan -> execute -> validate -> answer
```

## Structured Output

```go
type BookingResult struct {
    Service string `json:"service"`
    Date    string `json:"date"`
    Time    string `json:"time"`
}

// With session
var result BookingResult
err := agnogo.RunStructured(ctx, agent, session, "Book a haircut tomorrow at 14:00", &result)

// Without session (one-shot)
err := agnogo.AskStructured(ctx, agent, "Book a haircut tomorrow at 14:00", &result)
```

## Human-in-the-Loop

```go
agent.ToolWithApproval("delete", "Delete account", params, deleteFn, "Requires admin approval")

resp, _ := agent.Run(ctx, session, "Delete my account")
if resp.NeedsApproval {
    fmt.Printf("Tool %s needs approval: %s\n", resp.Approval.ToolName, resp.Approval.Reason)
    resp, _ = agent.Resume(ctx, session, true) // approve or deny
}
```

## Guardrails

```go
// Block bad input
agent.InputGuardrail("no-spam", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if isSpam(msg) { return errors.New("Spam blocked.") }
    return nil
})

// Block bad output
agent.OutputGuardrail("no-pii", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if containsPII(msg) { return errors.New("PII detected, response blocked.") }
    return nil
})
```

## Cancel Run

```go
ctx, runID := agnogo.RegisterRun(ctx, "run-1")
go agent.Run(ctx, session, "Long task...")

// Later:
agnogo.CancelRun(runID)
fmt.Println(agnogo.ActiveRunCount()) // 0
```

## CLI

```go
agent.CLI() // Interactive terminal
```

Commands: `exit`, `clear`, `memory`, `history`, `tools`. Supports human approval prompts.

## Debug and Tracing

```go
// Debug output
agent := agnogo.New(agnogo.Config{
    Debug: agnogo.DebugConfig{Enabled: true, Level: 2},
})

// Trace hooks
agent := agnogo.New(agnogo.Config{
    Trace: agnogo.DefaultTrace(), // logs all events
})

// Custom trace
agent := agnogo.New(agnogo.Config{
    Trace: &agnogo.Trace{
        OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) { /* ... */ },
        OnModelCall: func(msgs []agnogo.Message, resp *agnogo.ModelResponse, dur time.Duration) { /* ... */ },
    },
})
```

## Serialization

```go
dict := agent.ToDict()        // map[string]any
jsonBytes, _ := agent.ToJSON() // []byte
fmt.Println(agent.String())    // "Core(tools=[t1, t2], ...)"
```

## Session Operations

```go
session := agnogo.NewSession("s1")

// Memory (extracted facts)
session.SetMemory("name", "Erik")
name := session.GetMemory("name")

// State (arbitrary key-value)
session.Set("step", "checkout")
step := session.GetStr("step")
count := session.Increment("visits") // atomic counter

// Metadata
session.SetMeta("org", "acme")

// History
session.AddMessage("user", "hello")
session.AddMessage("assistant", "hi!")
session.AddToolResult("call-1", "result")
```

## Comparison with Agno Python

| Category | Agno (Python) | agnogo (Go) | Notes |
|----------|--------------|-------------|-------|
| Core features | 15 | 15 | 100% parity |
| One-shot API | N/A | `Ask`, `AskStream`, `AskStructured` | Go-only |
| Typed tools | N/A | `TypedTool[In, Out]` | Go-only, compile-time safe |
| HTTP server | N/A | `Serve`, `Handler`, `AgentMiddleware` | Go-only |
| Pipelines | N/A | `Then`, `All`, `Race`, `Map` | Go-only |
| Resilience | N/A | `Fallback`, `CircuitBreaker`, `RateLimiter` | Go-only |
| Observability | N/A | `MetricsCollector`, `CostTracker` | Go-only |
| Batch | N/A | `Batch`, `WorkerPool`, `Benchmark` | Go-only |
| Session/Memory | 10 | 8 | 80% |
| Workflows | 7 | 6 | 86% |
| Streaming | 4 | 4 | 100% |
| Providers | 41 | 10 | 24% (additive) |
| Vector DBs | 18 | 4 | 22% (additive) |
| Storage | 13 | 5 | 38% (additive) |
| Built-in tools | 129 | 16 | 12% (additive) |

The core agent framework exceeds Agno Python in features. The gap is only in integrations (providers, tools, vector DBs), which are additive and do not affect capability.

For the full feature-by-feature comparison, see [STATUS.md](STATUS.md).

For comprehensive usage documentation, see [GUIDE.md](GUIDE.md).

## Architecture

```
agnogo/
├── agent.go          # Core struct + run loop
├── smart.go          # Agent() convenience constructor + auto-detection
├── ask.go            # Ask, AskStream, AskStructured one-shot API
├── tool.go           # Tool registry + function defs
├── typed_tool.go     # TypedTool[In, Out] generic tools
├── serve.go          # Built-in HTTP server (Serve, Handler)
├── middleware.go      # AgentMiddleware, AgentFromContext
├── pipeline.go       # Then, All, Race, Map
├── resilience.go     # Fallback, CircuitBreaker, RateLimiter, MultiProvider, TimeoutProvider
├── observe.go        # MetricsCollector, CostTracker, Explain, Validate
├── worker_pool.go    # NewWorkerPool, Batch
├── benchmark.go      # Benchmark
├── session.go        # Session state + memory + history
├── provider.go       # ModelProvider interface
├── knowledge.go      # Knowledge interface + auto-injection
├── memory.go         # Pattern + LLM memory extraction
├── storage.go        # Storage interface + in-memory impl
├── guardrail.go      # Input/output guardrails
├── hallucination.go  # HallucinationGuard
├── team.go           # Multi-agent teams + routing
├── workflow.go       # Sequential/Parallel/Loop/Condition/Router
├── streaming.go      # Token-level SSE + fallback
├── reasoning.go      # Multi-step reasoning
├── structured.go     # RunStructured[T]
├── human.go          # Human-in-the-loop approval
├── cancel.go         # Run cancellation registry
├── retry.go          # Retry with exponential backoff
├── history.go        # History trimming
├── debug.go          # Debug output
├── trace.go          # 8 trace hooks
├── cli.go            # Interactive CLI
├── serialize.go      # ToDict/ToJSON/String
├── errors.go         # Sentinel errors
├── autodetect/       # Auto-register providers from env vars
├── providers/        # 10 LLM providers
├── tools/            # 16 built-in tools
├── knowledge/        # pgvector, Qdrant, ChromaDB
├── vectordb/         # Pinecone
└── storage/          # Postgres, SQLite, Redis, MySQL
```

## License

MIT
