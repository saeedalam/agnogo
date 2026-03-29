# agno-go

A lightweight Go agent development kit inspired by [Agno](https://github.com/agno-agi/agno) (Python, 39k+ stars).

Build AI agents with tools, knowledge, memory, teams, and guardrails — in pure Go.

**Zero external dependencies.** One package. ~900 lines. 28 tests.

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
)

func main() {
    agent := agnogo.New(agnogo.Config{
        Model:        agnogo.OpenAI(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini"),
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

## Comparison with Agno

| Feature | Agno (Python) | agno-go |
|---------|--------------|---------|
| Tools | ✅ `@tool` | ✅ `agent.Tool()` |
| Knowledge | ✅ | ✅ `KnowledgeFunc()` |
| Memory | ✅ | ✅ `AutoMemory` |
| Storage | ✅ | ✅ Interface |
| Guardrails | ✅ | ✅ |
| Teams | ✅ | ✅ |
| Human-in-the-loop | ✅ | ✅ |
| Streaming | ✅ | ✅ |
| Tracing | ✅ | ✅ |
| Dependencies | Many | **Zero** |

## License

MIT
