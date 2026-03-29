# agnogo

A Go agent framework inspired by [Agno](https://github.com/agno-agi/agno) — the Python agent SDK with 39k+ stars.

Build AI agents with tools, knowledge, memory, teams, workflows, and guardrails — in pure Go.

**Zero external dependencies** · 10 LLM providers · 16 built-in tools · 4 vector DBs · 5 storage backends · 69 tests

[![Go Reference](https://pkg.go.dev/badge/github.com/saeedalam/agnogo.svg)](https://pkg.go.dev/github.com/saeedalam/agnogo)

## Install

```bash
go get github.com/saeedalam/agnogo
```

## Quick Start

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

    agent.Tool("weather", "Get weather for a city", agnogo.Params{
        "city": {Type: "string", Desc: "City name", Required: true},
    }, func(ctx context.Context, args map[string]string) (string, error) {
        return fmt.Sprintf("Sunny, 22°C in %s", args["city"]), nil
    })

    session := agnogo.NewSession("user-1")
    resp, _ := agent.Run(context.Background(), session, "What's the weather in Stockholm?")
    fmt.Println(resp.Text)
}
```

## Features

| Feature | Description |
|---------|-------------|
| **Tools** | Register any Go function as a tool |
| **Knowledge** | Auto RAG injection for questions |
| **Memory** | Learn facts from conversations (pattern + LLM) |
| **Storage** | Persist sessions to Postgres, SQLite, Redis, MySQL |
| **Guardrails** | Block bad input/output |
| **Teams** | Route to sub-agents by intent |
| **Workflows** | Sequential, Parallel, Loop, Condition, Router |
| **Streaming** | Token-level SSE + word-level fallback |
| **Reasoning** | Multi-step reasoning with confidence scores |
| **Structured Output** | Force typed JSON responses |
| **Human-in-the-loop** | Require approval before actions |
| **Cancel Run** | Cancel any running agent |
| **CLI** | Interactive terminal with commands |
| **Serialization** | `ToDict()`, `ToJSON()`, `String()` |
| **Debug** | Two-level debug output |
| **Tracing** | 8 trace hooks for observability |

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
// "My name is Erik" → session.GetMemory("name") == "Erik"
// "anna@test.com"   → session.GetMemory("email") == "anna@test.com"

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
// Sequential: extract → validate → book
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
// Agent runs multi-step reasoning: analyze → strategize → plan → execute → validate → answer
```

## Structured Output

```go
type BookingResult struct {
    Service string `json:"service"`
    Date    string `json:"date"`
    Time    string `json:"time"`
}

var result BookingResult
err := agnogo.RunStructured(ctx, agent, session, "Book a haircut tomorrow at 14:00", &result)
// result.Service == "Haircut"
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

## Debug & Tracing

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
        OnToolCall: func(name string, args map[string]string) { /* ... */ },
        OnModelResponse: func(text string) { /* ... */ },
    },
})
```

## Serialization

```go
dict := agent.ToDict()        // map[string]any
jsonBytes, _ := agent.ToJSON() // []byte
fmt.Println(agent.String())    // "Agent(tools=[t1, t2], ...)"
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

## Comparison with Agno

| Category | Agno (Python) | agnogo (Go) | Coverage |
|----------|--------------|-------------|----------|
| Core features | 15 | 15 | **100%** |
| Session/Memory | 10 | 8 | **80%** |
| Workflows | 7 | 6 | **86%** |
| Streaming | 4 | 4 | **100%** |
| Providers | 41 | 10 | **24%** |
| Vector DBs | 18 | 4 | **22%** |
| Storage | 13 | 5 | **38%** |
| Built-in tools | 129 | 16 | **12%** |
| **Core framework** | | | **~90%** |

The core agent framework is at ~90% parity with Agno. The gap is mainly integrations (providers, tools, vector DBs) which are additive.

For the full feature-by-feature comparison, see [STATUS.md](STATUS.md).

For comprehensive usage documentation, see [GUIDE.md](GUIDE.md).

## Architecture

```
agnogo/
├── agent.go          # Core agent + run loop
├── tool.go           # Tool registry + function defs
├── session.go        # Session state + memory + history
├── provider.go       # ModelProvider interface
├── knowledge.go      # Knowledge interface + auto-injection
├── memory.go         # Pattern + LLM memory extraction
├── storage.go        # Storage interface + in-memory impl
├── guardrail.go      # Input/output guardrails
├── team.go           # Multi-agent teams + routing
├── workflow.go       # Sequential/Parallel/Loop/Condition/Router
├── streaming.go      # Token-level SSE + fallback
├── reasoning.go      # Multi-step reasoning
├── human.go          # Human-in-the-loop approval
├── cancel.go         # Run cancellation registry
├── retry.go          # Retry with exponential backoff
├── history.go        # History trimming
├── debug.go          # Debug output
├── cli.go            # Interactive CLI
├── serialize.go      # ToDict/ToJSON/String
├── structured.go     # RunStructured[T]
├── errors.go         # Sentinel errors
├── providers/        # 10 LLM providers
├── tools/            # 16 built-in tools
├── knowledge/        # pgvector, Qdrant, ChromaDB
├── vectordb/         # Pinecone
└── storage/          # Postgres, SQLite, Redis, MySQL
```

## License

MIT
