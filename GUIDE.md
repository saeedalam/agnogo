# agnogo -- Complete Guide

## Installation

```bash
go get github.com/saeedalam/agnogo
```

## Quick Start

Single import, no autodetect needed:

```go
package main

import (
    "context"
    "fmt"

    "github.com/saeedalam/agnogo"
)

func main() {
    agent := agnogo.Agent("You are a helpful assistant.")
    answer, _ := agent.Ask(context.Background(), "Hello!")
    fmt.Println(answer)
}
```

Or specify a provider explicitly (no env vars needed):

```go
agent := agnogo.Agent("You are helpful.", agnogo.WithOpenAI())
agent := agnogo.Agent("You are helpful.", agnogo.WithAnthropic("claude-sonnet-4-5-20250514"))
```

Power-user mode -- explicit provider, tools, memory, and debug:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/saeedalam/agnogo"
    "github.com/saeedalam/agnogo/providers/openai"
    "github.com/saeedalam/agnogo/tools"
)

func main() {
    agent := agnogo.New(agnogo.Config{
        Model:        openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini"),
        Instructions: "You are a helpful assistant.",
        AutoMemory:   true,
        Debug:        &agnogo.DefaultDebug(),
    })

    // Add built-in tools
    agent.AddTools(tools.Calculator()...)
    agent.AddTools(tools.DuckDuckGo()...)
    agent.AddTools(tools.Wikipedia()...)

    // Interactive CLI
    agent.CLI()
}
```

---

## One-Shot (Ask)

Skip session management entirely -- just ask a question and get an answer:

```go
// Simple question/answer
answer, err := agent.Ask(ctx, "What is the capital of France?")

// Streaming one-shot
ch := agent.AskStream(ctx, "Tell me a story")
for chunk := range ch {
    fmt.Print(chunk.Text)
    if chunk.Done { break }
}

// Structured one-shot -- parse the answer into a typed struct
var result MyStruct
err := agnogo.AskStructured[MyStruct](ctx, agent, "Extract the data", &result)
```

---

## Error Handling

All provider and tool errors are structured. Error classification uses package-level
functions, not methods on the error value:

```go
resp, err := agent.Run(ctx, session, msg)
if err != nil {
    if agnogo.IsRateLimited(err) {
        delay := agnogo.RetryAfter(err)
        time.Sleep(delay)
        // retry...
    }
    if !agnogo.IsRetryable(err) {
        log.Fatal("permanent error:", err)
    }
}
```

For fine-grained inspection, unwrap to the concrete type:

```go
var pe *agnogo.ProviderError
if errors.As(err, &pe) {
    fmt.Println(pe.Provider, pe.StatusCode)
}

var te *agnogo.ToolError
if errors.As(err, &te) {
    fmt.Println(te.Tool, te.Message, te.Err)
}
```

`IsRetryable()` returns true for 429, 500, 502, 503, 504. `IsRateLimited()` returns true for 429. `RetryAfter()` parses the Retry-After header when present.

---

## Typed Tools

Define tools using Go generics and struct tags instead of manual parameter maps:

```go
type WeatherInput struct {
    City string `json:"city" desc:"City name" required:"true"`
    Unit string `json:"unit" desc:"celsius or fahrenheit" enum:"celsius,fahrenheit"`
}

type WeatherOutput struct {
    Temp    float64 `json:"temp"`
    Summary string  `json:"summary"`
}

tool := agnogo.TypedTool[WeatherInput, WeatherOutput](
    "get_weather", "Get weather for a city",
    func(ctx context.Context, in WeatherInput) (WeatherOutput, error) {
        return fetchWeather(in.City, in.Unit)
    },
)

agent.AddTools(tool)
```

Struct tags supported: `json` (name), `desc` (description), `required` ("true"), `enum` (comma-separated values).

---

## HTTP Server

Expose an agent as an HTTP API with one call:

```go
// Quick start -- serves /ask (POST) and /health (GET)
agent.Serve(":8080")

// With hardening
agent.Serve(":8080",
    agnogo.WithMaxConcurrent(100),  // limit concurrent requests
    agnogo.WithMaxBodySize(1<<20),  // 1 MB max request body
)

// Or get the http.Handler to mount on your own mux
mux := http.NewServeMux()
mux.Handle("/agent/", http.StripPrefix("/agent", agent.Handler()))
http.ListenAndServe(":8080", mux)
```

---

## Pipelines and Concurrency

Chain agents and run them concurrently:

```go
// Sequential pipeline -- output of one feeds into the next
result, _ := agent1.Then(agent2).Then(agent3).Run(ctx, "input")

// Fan-out -- run agents in parallel, collect all results
results, _ := agnogo.All(agent1, agent2, agent3).Run(ctx, "input")

// Race -- return the first agent to finish
result, _ := agnogo.Race(agent1, agent2).Run(ctx, "input")

// Map -- apply one agent to many inputs concurrently
results := agnogo.Map(ctx, agent, []string{"a", "b", "c"}, 4) // 4 workers
```

---

## Resilience

Wrap providers for fault tolerance:

```go
// Fallback to a secondary provider on error
model := agnogo.Fallback(primaryModel, backupModel)

// Try multiple providers in order
model := agnogo.MultiProvider(openaiModel, claudeModel, geminiModel)

// Circuit breaker -- stop hammering a failing provider
model := agnogo.CircuitBreaker(openaiModel,
    agnogo.WithFailureThreshold(5),
    agnogo.WithResetTimeout(30*time.Second),
)

// Rate limiter -- token bucket, blocks until a token is available
model := agnogo.RateLimiter(openaiModel, 60) // 60 requests per minute

// Timeout -- per-request deadline
model := agnogo.TimeoutProvider(openaiModel, 30*time.Second)

// Cleanup -- Close() on providers that implement Closeable
rl := agnogo.RateLimiter(openaiModel, 60)
defer agnogo.CloseProvider(rl) // safe no-op if not Closeable
```

---

## Observability

Inspect, validate, and measure your agents:

```go
// Explain prints a human-readable summary of the agent's configuration
agnogo.Explain(agent)

// Validate checks for common misconfigurations
errs := agnogo.Validate(agent)
for _, e := range errs {
    fmt.Println(e.Field, e.Message)
}

// MetricsCollector aggregates telemetry across runs
mc := agnogo.NewMetricsCollector()
agent := agnogo.New(agnogo.Config{
    Trace: mc.Trace(),
})
// ... run the agent ...
snap := mc.Snapshot() // MetricsSnapshot with counts, latencies, costs

// Expose metrics as an HTTP endpoint (JSON)
http.Handle("/metrics", mc.Handler())
```

---

## Batch Processing

Process many messages through an agent concurrently:

```go
// WorkerPool -- long-lived pool for streaming work
pool := agnogo.NewWorkerPool(agent, 4) // 4 goroutines
pool.Start(ctx)

pool.Submit(agnogo.WorkerTask{ID: "t1", Message: "Summarize doc A"})
pool.Submit(agnogo.WorkerTask{ID: "t2", Message: "Summarize doc B"})

for result := range pool.Results() {
    fmt.Println(result.ID, result.Text)
}
pool.Stop()

// Batch -- one-shot convenience for a slice of tasks
results := agnogo.Batch(ctx, agent, tasks, 4) // 4 concurrent workers
```

---

## HTTP Middleware

Integrate agents into existing HTTP servers:

```go
// Inject the agent into request context
mux.Handle("/chat", agnogo.AgentMiddleware(agent)(chatHandler))

// Retrieve it downstream
func chatHandler(w http.ResponseWriter, r *http.Request) {
    agent := agnogo.AgentFromContext(r.Context())
    resp, _ := agent.Ask(r.Context(), r.URL.Query().Get("q"))
    fmt.Fprint(w, resp)
}

// Or use the built-in handler that accepts {"message":"..."} POST bodies
mux.HandleFunc("POST /chat", agnogo.AgentHandler(agent))
```

---

## Benchmark

Measure agent performance with configurable prompts, warmup, and concurrency:

```go
result := agnogo.Benchmark(ctx, agent, agnogo.BenchmarkConfig{
    Prompts:     []string{"Hello", "How are you?", "What is Go?"},
    Concurrency: 4,
    Warmup:      2,
})
fmt.Printf("Avg: %s, P99: %s, Throughput: %.1f req/s\n",
    result.AvgLatency, result.P99Latency, result.Throughput)
```

BenchmarkConfig fields: `Prompts`, `Concurrency`, `Warmup`.
BenchmarkResult fields: `P50Latency`, `P95Latency`, `P99Latency`, `AvgLatency`, `ErrorCount`, `Throughput`.

---

## Graph Workflows

Define a directed graph of agents with conditional edges. The graph runs the entry node, then follows edges based on predicates until it reaches an end node:

```go
g := agnogo.NewGraph()
g.AddNode("classify", classifyAgent)
g.AddNode("refund", refundAgent)
g.AddNode("support", supportAgent)

g.SetEntry("classify")
g.SetEnd("refund", "support")

// Conditional edge: route to "refund" if the classifier says REFUND
g.AddEdge("classify", "refund", func(ctx context.Context, state *agnogo.GraphState) bool {
    return strings.Contains(state.GetStr("last_response"), "REFUND")
})
// Default edge (nil predicate): taken when no conditional edge matches
g.AddEdge("classify", "support", nil)

resp, _ := g.Run(ctx, session, "I want a refund")
```

State is shared across nodes via `*GraphState`. Each node's response is stored in `state.GetStr("last_response")` for downstream predicates.

---

## Run Context (Dependency Injection)

Pass request-scoped data (user ID, tenant, feature flags) to tools without threading extra parameters through every function:

```go
rctx := agnogo.NewRunContext()
rctx.Set("user_id", "u-123")
rctx.Set("tenant", "acme")

ctx := rctx.WithContext(context.Background())
resp, _ := agent.Run(ctx, session, "Check my balance")

// Inside any tool function:
func checkBalance(ctx context.Context, args map[string]string) (string, error) {
    rc := agnogo.RunCtx(ctx)
    userID := rc.GetStr("user_id") // "u-123"
    // ... look up balance for userID
}
```

---

## Event Bus

Decouple observability from agent logic with a pub/sub event system:

```go
bus := agnogo.NewEventBus()

bus.On(agnogo.EventRunStart, func(e agnogo.Event) {
    log.Println("run started:", e.Data["run_id"])
})
bus.On(agnogo.EventModelDone, func(e agnogo.Event) {
    log.Println("model done:", e.Data["duration"])
})
bus.On(agnogo.EventToolCall, func(e agnogo.Event) {
    log.Println("tool called:", e.Data["tool"])
})

agent := agnogo.Agent("You are helpful.", agnogo.WithEvents(bus))
```

Built-in event types: `EventRunStart`, `EventRunEnd`, `EventModelCall`, `EventModelDone`, `EventToolCall`, `EventToolDone`.

---

## Middleware Hooks

Wrap every `Run` call with reusable middleware. Hooks follow the standard middleware pattern -- call `next` to continue:

```go
timer := func(ctx context.Context, a *agnogo.Core, s *agnogo.Session, msg string, next agnogo.NextFunc) (*agnogo.Response, error) {
    start := time.Now()
    resp, err := next(ctx, a, s, msg)
    log.Printf("run took %s", time.Since(start))
    return resp, err
}

logger := func(ctx context.Context, a *agnogo.Core, s *agnogo.Session, msg string, next agnogo.NextFunc) (*agnogo.Response, error) {
    log.Printf("input: %s", msg)
    resp, err := next(ctx, a, s, msg)
    if resp != nil { log.Printf("output: %s", resp.Text) }
    return resp, err
}

agent := agnogo.Agent("You are helpful.", agnogo.WithHooks(timer, logger))
```

Hooks compose in order: the first hook wraps the second, which wraps the core run loop.

---

## Session Summarization

Automatically compress old messages into a summary to stay within the context window:

```go
agent := agnogo.Agent("You are helpful.", agnogo.WithSummarize(30))
```

When a session exceeds 30 messages, the oldest messages are replaced with a single summary message generated by the LLM. The summary preserves key facts and conversation context while reducing token usage.

---

## Model Providers

### OpenAI
```go
import "github.com/saeedalam/agnogo/providers/openai"
model := openai.New("sk-...", "gpt-4.1-mini")
model := openai.New("sk-...", "gpt-4o", agnogo.ModelConfig{MaxTokens: 4000})
```

### Anthropic (Claude)
```go
import "github.com/saeedalam/agnogo/providers/anthropic"
model := anthropic.New("sk-ant-...", "claude-sonnet-4-5-20250514")
```

### Google Gemini
```go
import "github.com/saeedalam/agnogo/providers/gemini"
model := gemini.New("AIza...", "gemini-2.5-flash")
```

### Ollama (Local)
```go
import "github.com/saeedalam/agnogo/providers/ollama"
model := ollama.New("llama3.1")                    // localhost:11434
model := ollama.New("mistral", "http://gpu:11434") // remote
```

### Other Providers
```go
import "github.com/saeedalam/agnogo/providers/groq"      // groq.New(key, "llama-3.3-70b-versatile")
import "github.com/saeedalam/agnogo/providers/deepseek"   // deepseek.New(key, "deepseek-chat")
import "github.com/saeedalam/agnogo/providers/together"   // together.New(key, "meta-llama/Llama-3.3-70B-Instruct-Turbo")
import "github.com/saeedalam/agnogo/providers/mistral"    // mistral.New(key, "mistral-large-latest")
import "github.com/saeedalam/agnogo/providers/perplexity" // perplexity.New(key, "sonar-pro")
import "github.com/saeedalam/agnogo/providers/grok"       // grok.New(key, "grok-3")
```

### Custom Provider
```go
type MyProvider struct{}
func (p *MyProvider) ChatCompletion(ctx context.Context, msgs []agnogo.Message, tools []map[string]any) (*agnogo.ModelResponse, error) {
    // Your implementation
}
```

---

## Tools

### Register Custom Tools
```go
agent.Tool("get_weather", "Get weather for a city", agnogo.Params{
    "city": {Type: "string", Desc: "City name", Required: true},
    "unit": {Type: "string", Desc: "celsius or fahrenheit", Enum: []string{"celsius", "fahrenheit"}},
}, func(ctx context.Context, args map[string]string) (string, error) {
    return getWeather(args["city"], args["unit"])
})
```

### Built-in Tools (35)
```go
import "github.com/saeedalam/agnogo/tools"

// --- Core (16: configurable limits, expression parser, HTML stripping, pagination) ---
agent.AddTools(tools.Calculator()...)                          // expression parser calculator
agent.AddTools(tools.Shell("echo", "ls", "cat")...)           // shell (with allowlist)
agent.AddTools(tools.HTTP()...)                                // HTTP requests
agent.AddTools(tools.File("/safe/dir")...)                     // file read/write/list (pagination)
agent.AddTools(tools.DuckDuckGo()...)                          // web search (configurable limits)
agent.AddTools(tools.Wikipedia()...)                           // Wikipedia lookup
agent.AddTools(tools.WebBrowser()...)                          // fetch & read URLs (HTML stripping)
agent.AddTools(tools.Email("smtp.gmail.com", 465, u, p, f)...)// SMTP email
agent.AddTools(tools.SQL(db, true)...)                         // SQL queries (read-only)
agent.AddTools(tools.JSON()...)                                // JSON parse/format
agent.AddTools(tools.CSV()...)                                 // CSV -> JSON
agent.AddTools(tools.Slack("xoxb-token")...)                   // Slack messaging
agent.AddTools(tools.GitHub("ghp_token")...)                   // GitHub API
agent.AddTools(tools.Docker()...)                               // Docker management
agent.AddTools(tools.GoogleSearch("api-key", "cx-id")...)      // Google search
agent.AddTools(tools.Env()...)                                 // read environment variables

// --- Utility (19) ---
agent.AddTools(tools.Regex()...)          // regex match/replace
agent.AddTools(tools.Base64()...)         // base64 encode/decode
agent.AddTools(tools.Hash()...)           // SHA-256, MD5, etc.
agent.AddTools(tools.UUID()...)           // generate UUIDs
agent.AddTools(tools.TimeTool()...)       // current time, parse, format
agent.AddTools(tools.TemplateTool()...)   // Go text/template rendering
agent.AddTools(tools.YAML()...)           // YAML parse/format
agent.AddTools(tools.XML()...)            // XML parse/format
agent.AddTools(tools.Diff()...)           // text diff
agent.AddTools(tools.Archive()...)        // tar/zip create/extract
agent.AddTools(tools.Crypto()...)         // encrypt/decrypt (AES)
agent.AddTools(tools.DNS()...)            // DNS lookup
agent.AddTools(tools.TCP()...)            // TCP connect/send
agent.AddTools(tools.Markdown()...)       // markdown to HTML
agent.AddTools(tools.PDFTool()...)        // PDF text extraction
agent.AddTools(tools.ImageTool()...)      // image metadata/resize
agent.AddTools(tools.CronTool()...)       // cron expression parser
agent.AddTools(tools.Semver()...)         // semantic version compare
agent.AddTools(tools.MetricsTool()...)    // Prometheus-style metrics
```

### Tool with Human Approval
```go
agent.ToolWithApproval("delete_data", "Delete user data", params, deleteFn,
    "Data deletion requires admin approval")

resp, _ := agent.Run(ctx, session, "Delete my account")
if resp.NeedsApproval {
    // Show to human in your UI
    fmt.Println(resp.Approval.Reason)
    // Human approves
    resp, _ = agent.Resume(ctx, session, true)
}
```

---

## Knowledge (RAG)

### Inline Function
```go
agent := agnogo.New(agnogo.Config{
    Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
        return myDB.SearchDocuments(ctx, query, limit)
    }),
})
```

### PostgreSQL pgvector
```go
import "github.com/saeedalam/agnogo/knowledge/pgvector"
kb := pgvector.New(pool, pgvector.Config{
    Table:     "document_chunks",
    EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
        return openaiEmbed(ctx, text) // your embedding function
    },
})
agent := agnogo.New(agnogo.Config{Knowledge: kb})
```

### Other Vector DBs
```go
import "github.com/saeedalam/agnogo/knowledge/qdrant"
import "github.com/saeedalam/agnogo/knowledge/chromadb"
import "github.com/saeedalam/agnogo/vectordb/pinecone"

qdrant.New("http://localhost:6333", "collection", embedFn)
chromadb.New("http://localhost:8000", "collection")
pinecone.New("https://xxx.pinecone.io", "api-key", embedFn)
```

---

## Memory

### Auto-extract Facts (Pattern-based, Free)
```go
agent := agnogo.New(agnogo.Config{AutoMemory: true})
// "My name is Erik" -> session.GetMemory("name") == "Erik"
// "erik@example.com" -> session.GetMemory("email") == "erik@example.com"
```

### LLM-based Extraction (More Accurate, Costs Tokens)
```go
agent := agnogo.New(agnogo.Config{
    Memory: &agnogo.LLMMemory{
        Model:  openai.New(key, "gpt-4.1-mini"),
        Fields: []string{"name", "company", "role", "preferences"},
    },
})
```

---

## Session Storage

### In-memory (Testing)
```go
agent := agnogo.New(agnogo.Config{Storage: agnogo.NewMemoryStorage()})
resp, _ := agent.RunWithStorage(ctx, "session-123", "Hello!")
```

### PostgreSQL
```go
import "github.com/saeedalam/agnogo/storage/postgres"
store, _ := postgres.New(db) // auto-creates table
agent := agnogo.New(agnogo.Config{Storage: store})
```

### SQLite
```go
import "github.com/saeedalam/agnogo/storage/sqlite"
store, _ := sqlite.New(db) // auto-creates table
```

### Redis
```go
import "github.com/saeedalam/agnogo/storage/redis"
store := redis.New("localhost:6379", redis.WithTTL(24*time.Hour))
```

### MySQL
```go
import "github.com/saeedalam/agnogo/storage/mysql"
store, _ := mysql.New(db)
```

### Session Management
```go
agent.GetSession(ctx, "session-123")
agent.SaveSession(ctx, session)
agent.DeleteSession(ctx, "session-123")
agent.ListSessions(ctx, 50)
agent.GetChatHistory(ctx, "session-123")
agent.GetMemories(ctx, "session-123")
```

---

## Guardrails

### Input Guardrail (Block Bad Input)
```go
agent.InputGuardrail("no-profanity", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if containsProfanity(msg) {
        return errors.New("Please keep the conversation respectful.")
    }
    return nil
})
```

### Output Guardrail (Block Bad Output)
```go
agent.OutputGuardrail("no-pii", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if containsPhoneNumber(msg) {
        return errors.New("I cannot share personal contact information.")
    }
    return nil
})
```

---

## Teams (Multi-Agent)

### LLM-based Routing
```go
team := agnogo.NewTeam(agnogo.TeamConfig{
    Model: openai.New(key, "gpt-4.1-mini"),
})
team.Agent("booking", bookingAgent)
team.Agent("support", supportAgent)
team.Agent("complaint", complaintAgent)

resp, _ := team.Run(ctx, session, "I want to book a haircut")
// -> automatically routed to "booking" agent
```

### Custom Routing
```go
team := agnogo.NewTeam(agnogo.TeamConfig{
    RouterFunc: func(ctx context.Context, msg string, agents []string) (string, error) {
        if strings.Contains(msg, "book") { return "booking", nil }
        if strings.Contains(msg, "angry") { return "complaint", nil }
        return "support", nil
    },
})
```

---

## Workflows

### Sequential
```go
wf := agnogo.Sequential(
    agnogo.Step("extract", extractAgent),
    agnogo.Step("validate", validateAgent),
    agnogo.Step("execute", executeAgent),
)
resp, _ := wf.Run(ctx, session, "Process this order")
```

### Parallel
```go
wf := agnogo.Parallel(
    agnogo.Step("weather", weatherAgent),
    agnogo.Step("news", newsAgent),
    agnogo.Step("calendar", calendarAgent),
)
resp, _ := wf.Run(ctx, session, "Morning briefing")
```

### Loop
```go
wf := agnogo.Loop(refinementAgent, func(resp *agnogo.Response, i int) bool {
    return strings.Contains(resp.Text, "DONE") || i >= 5
})
```

### Condition
```go
wf := agnogo.Condition(
    func(ctx context.Context, input string) bool {
        return strings.Contains(input, "urgent")
    },
    urgentWorkflow,  // true branch
    normalWorkflow,  // false branch
)
```

### Router
```go
wf := agnogo.Route(
    func(ctx context.Context, input string) string {
        if strings.Contains(input, "refund") { return "refund" }
        return "general"
    },
    map[string]agnogo.Workflow{
        "refund":  refundWorkflow,
        "general": generalWorkflow,
    },
)
```

---

## Reasoning (Chain-of-Thought)

```go
agent := agnogo.New(agnogo.Config{
    Reasoning: &agnogo.ReasoningConfig{
        Enabled:  true,
        MinSteps: 2,
        MaxSteps: 6,
        Model:    openai.New(key, "gpt-4.1-mini"), // cheap model for thinking
    },
})
// Agent thinks step-by-step before responding
```

---

## Streaming

### Token-level (Real SSE)
```go
ch := agent.RunStreamReal(ctx, session, "Tell me about Go")
for chunk := range ch {
    if chunk.Error != nil { break }
    fmt.Print(chunk.Text) // prints token by token
    if chunk.Done { break }
}
```

### Word-level (Fallback)
```go
ch := agent.RunStream(ctx, session, "Hello")
for chunk := range ch {
    fmt.Print(chunk.Text)
    if chunk.Done { break }
}
```

---

## Structured Output

```go
type BookingResult struct {
    Service string `json:"service"`
    Date    string `json:"date"`
    Time    string `json:"time"`
}

var result BookingResult
err := agnogo.RunStructured(ctx, agent, session,
    "Book a haircut tomorrow at 14:00", &result)
// result.Service == "Haircut"
```

---

## Retry & History

```go
agent := agnogo.New(agnogo.Config{
    Retry: &agnogo.RetryConfig{
        MaxRetries:         3,
        InitialDelay:       time.Second,
        ExponentialBackoff: true,
    },
    History: &agnogo.HistoryConfig{
        MaxMessages:     50,  // trim old messages
        MaxToolMessages: 20,  // limit tool results
    },
})
```

---

## Debug Mode

```go
// Level 1: key decisions (tool calls, responses)
agent := agnogo.New(agnogo.Config{Debug: &agnogo.DefaultDebug()})

// Level 2: everything (messages, args, results)
agent := agnogo.New(agnogo.Config{Debug: &agnogo.VerboseDebug()})

// Custom output
agent := agnogo.New(agnogo.Config{
    Debug: &agnogo.DebugConfig{
        Enabled: true, Level: 2,
        Printer: func(s string) { myLogger.Info(s) },
    },
})
```

---

## Tracing

```go
agent := agnogo.New(agnogo.Config{
    Trace: &agnogo.Trace{
        OnModelCall: func(msgs []agnogo.Message, resp *agnogo.ModelResponse, dur time.Duration) {
            metrics.RecordModelLatency(dur)
        },
        OnToolCall: func(name string, args map[string]string, result string, dur time.Duration, err error) {
            metrics.RecordToolCall(name, dur, err)
        },
        OnKnowledge: func(query, result string, dur time.Duration) { ... },
        OnMemory:    func(key, value string) { ... },
        OnGuardrail: func(name, direction string, blocked bool) { ... },
        OnApproval:  func(a agnogo.HumanApproval) { ... },
        OnRouting:   func(agentName, msg string) { ... },
        OnSessionSave: func(s *agnogo.Session, err error) { ... },
    },
})
// Or use defaults: agnogo.DefaultTrace() logs via slog
```

---

## CLI App

```go
agent.CLI()
// Interactive terminal:
// > What's 2+2?
// The answer is 4.
//
// > memory
//   name: Erik
//
// > tools
//   calculator -- Perform math
//   web_search -- Search the web
//
// > exit
// Goodbye!
```

---

## Cancel a Run

```go
ctx, runID := agnogo.RegisterRun(context.Background(), "run-123")
go agent.Run(ctx, session, "Long task...")

// Later:
agnogo.CancelRun("run-123")
```

---

## Serialization

```go
data := agent.ToDict()  // map[string]any
json, _ := agent.ToJSON() // []byte
fmt.Println(agent.String()) // "Core{tools: [calculator, web_search], max_loops: 8}"
```
