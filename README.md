# agno-go

A lightweight Go agent development kit inspired by [Agno](https://github.com/agno-agi/agno).

Build AI agents with tools, knowledge, memory, teams, and guardrails — in pure Go.

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
        Instructions: "You are a helpful assistant.",
        AutoMemory:   true,
    })

    agent.Tool("weather", "Get weather for a city", agnogo.Params{
        "city": {Type: "string", Desc: "City name", Required: true},
    }, func(ctx context.Context, args map[string]string) (string, error) {
        return fmt.Sprintf("Weather in %s: Sunny, 22°C", args["city"]), nil
    })

    session := agnogo.NewSession("user-1")
    resp, _ := agent.Run(context.Background(), session, "What's the weather in Stockholm?")
    fmt.Println(resp.Text)
}
```

## Features

| Feature | Description |
|---------|-------------|
| **Agent** | Stateful agent with tool-calling loop |
| **Tools** | Register any Go function as a tool |
| **Knowledge** | Auto RAG injection for questions |
| **Memory** | Auto-extract facts from conversation |
| **Storage** | Persist sessions to any database |
| **Guardrails** | Input/output validation hooks |
| **Teams** | Route to sub-agents by intent |
| **Human-in-the-loop** | Require approval before tool execution |
| **Streaming** | Stream responses for WebSocket/SSE |
| **Tracing** | Hooks for every agent decision |
| **Providers** | OpenAI built-in, any LLM via interface |

## Tools

```go
// Register any Go function
agent.Tool("search", "Search the web", agnogo.Params{
    "query": {Type: "string", Desc: "Search query", Required: true},
}, mySearchFunc)

// Tool with human approval
agent.ToolWithApproval("transfer", "Transfer money", params, transferFunc,
    "Amounts over 1000 need approval")
```

## Teams

```go
team := agnogo.NewTeam(agnogo.TeamConfig{
    Model: agnogo.OpenAI(key, "gpt-4.1-mini"),
})
team.Agent("booking", bookingAgent)
team.Agent("support", supportAgent)

resp, _ := team.Run(ctx, session, "I want to book a haircut")
// → routes to "booking" agent
```

## Knowledge (RAG)

```go
agent := agnogo.New(agnogo.Config{
    Knowledge: agnogo.KnowledgeFunc(func(ctx context.Context, query string, limit int) (string, error) {
        return myVectorDB.Search(ctx, query, limit)
    }),
})
// Agent auto-searches for questions — no explicit tool call needed
```

## Human-in-the-loop

```go
resp, _ := agent.Run(ctx, session, "Transfer $5000 to Alice")
if resp.NeedsApproval {
    fmt.Println("Approval needed:", resp.Approval.Reason)
    // Human reviews → approves
    resp, _ = agent.Resume(ctx, session, true)
}
```

## Storage

```go
// Built-in memory storage (for testing)
agent := agnogo.New(agnogo.Config{
    Storage: agnogo.NewMemoryStorage(),
})

// Or implement the interface for your database
type Storage interface {
    Load(ctx context.Context, sessionID string) (*Session, error)
    Save(ctx context.Context, session *Session) error
}
```

## License

MIT
