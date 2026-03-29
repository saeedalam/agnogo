# agno-go

A lightweight Go agent development kit inspired by [Agno](https://github.com/agno-agi/agno) (Python, 39k+ stars).

Build AI agents with tools, knowledge, memory, teams, and guardrails — in pure Go.

**Zero external dependencies.** 28 tests. 5 LLM providers. 3 vector DBs.

## Install

```bash
go get github.com/saeedalam/agnogo
```

## Providers

```go
import (
    "github.com/saeedalam/agnogo/providers/openai"
    "github.com/saeedalam/agnogo/providers/anthropic"
    "github.com/saeedalam/agnogo/providers/gemini"
    "github.com/saeedalam/agnogo/providers/ollama"
    "github.com/saeedalam/agnogo/providers/grok"
)

openai.New("sk-...", "gpt-4.1-mini")           // OpenAI
anthropic.New("sk-ant-...", "claude-sonnet-4-5-20250514") // Claude
gemini.New("AIza...", "gemini-2.5-flash")       // Google Gemini
ollama.New("llama3.1")                          // Local Ollama
grok.New("xai-...", "grok-3")                   // xAI Grok
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

- **Tools** — register any Go function as a tool
- **Knowledge** — auto RAG injection for questions
- **Memory** — learn facts from conversations
- **Storage** — persist sessions to any database
- **Guardrails** — block bad input/output
- **Teams** — route to sub-agents by intent
- **Human-in-the-loop** — require approval before actions
- **Streaming** — stream responses for WebSocket/SSE
- **Tracing** — hooks for every agent decision
- **Providers** — OpenAI built-in, any LLM via interface

## Tools

```go
agent.Tool("search", "Search the web", agnogo.Params{
    "query": {Type: "string", Desc: "Search query", Required: true},
}, searchFn)

agent.Tool("send_email", "Send email", params, sendEmailFn)
agent.Tool("create_ticket", "Create ticket", params, createTicketFn)
```

## Knowledge (RAG)

```go
agent := agnogo.New(agnogo.Config{
    Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
        return myVectorDB.Search(ctx, query, limit)
    }),
})
```

## Memory

```go
agent := agnogo.New(agnogo.Config{AutoMemory: true})
// "My name is Erik" → session.GetMemory("name") == "Erik"
```

## Teams

```go
team := agnogo.NewTeam(agnogo.TeamConfig{Model: model})
team.Agent("booking", bookingAgent)
team.Agent("support", supportAgent)
resp, _ := team.Run(ctx, session, "I want to book a haircut")
```

## Human-in-the-Loop

```go
agent.ToolWithApproval("delete", "Delete account", params, deleteFn, "Requires admin approval")
resp, _ := agent.Run(ctx, session, "Delete my account")
if resp.NeedsApproval {
    resp, _ = agent.Resume(ctx, session, true) // approve
}
```

## Guardrails

```go
agent.InputGuardrail("no-spam", func(ctx context.Context, s *agnogo.Session, msg string) error {
    if isSpam(msg) { return errors.New("Spam blocked.") }
    return nil
})
```

## Tracing

```go
agent := agnogo.New(agnogo.Config{Trace: agnogo.DefaultTrace()})
// Logs every model call, tool call, knowledge search, memory extraction
```

## Structured Output

Force the model to return typed JSON:

```go
type BookingResult struct {
    Service string `json:"service"`
    Date    string `json:"date"`
    Time    string `json:"time"`
    Staff   string `json:"staff"`
}

var result BookingResult
err := agnogo.RunStructured(ctx, agent, session, "Book a haircut tomorrow at 14:00", &result)
// result.Service == "Haircut", result.Date == "2026-04-01", result.Time == "14:00"
```

## Workflows

Compose agents into sequential, parallel, or loop workflows:

```go
// Sequential: extract → validate → book
wf := agnogo.Sequential(
    agnogo.Step("extract", extractAgent),
    agnogo.Step("validate", validateAgent),
    agnogo.Step("book", bookAgent),
)
resp, _ := wf.Run(ctx, session, "Book a haircut tomorrow at 14:00")

// Parallel: fetch weather + news + calendar at once
wf := agnogo.Parallel(
    agnogo.Step("weather", weatherAgent),
    agnogo.Step("news", newsAgent),
    agnogo.Step("calendar", calendarAgent),
)
resp, _ := wf.Run(ctx, session, "Morning briefing")

// Loop: refine until done
wf := agnogo.Loop(refinementAgent, func(resp *agnogo.Response, i int) bool {
    return strings.Contains(resp.Text, "DONE") || i >= 5
})
```

## Knowledge Backends

```go
import (
    "github.com/saeedalam/agnogo/knowledge/pgvector"
    "github.com/saeedalam/agnogo/knowledge/qdrant"
    "github.com/saeedalam/agnogo/knowledge/chromadb"
)

pgvector.New(pool, pgvector.Config{Table: "chunks", EmbedFunc: embedFn})
qdrant.New("http://localhost:6333", "collection", embedFn)
chromadb.New("http://localhost:8000", "collection")
```

## Comparison with Agno

| Feature | Agno (Python) | agno-go |
|---------|--------------|---------|
| Tools | ✅ `@tool` | ✅ `agent.Tool()` |
| Knowledge | ✅ | ✅ pgvector, Qdrant, ChromaDB |
| Memory | ✅ | ✅ Pattern + LLM |
| Storage | ✅ | ✅ Interface + MemoryStorage |
| Guardrails | ✅ | ✅ Input + Output |
| Teams | ✅ | ✅ LLM + custom routing |
| Human-in-the-loop | ✅ | ✅ `ToolWithApproval` + `Resume` |
| Streaming | ✅ | ✅ `RunStream()` |
| Structured output | ✅ | ✅ `RunStructured[T]()` |
| Workflows | ❌ | ✅ Sequential, Parallel, Loop |
| Tracing | ✅ | ✅ 8 trace hooks |
| Providers | 10+ | ✅ OpenAI, Claude, Gemini, Ollama, Grok |
| Dependencies | Many | **Zero** (stdlib only) |
| Language | Python | **Go** |

## License

MIT
