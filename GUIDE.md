# agnogo — Complete Guide

## Installation

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

### Built-in Tools
```go
import "github.com/saeedalam/agnogo/tools"

agent.AddTools(tools.Calculator()...)                          // math operations
agent.AddTools(tools.Shell("echo", "ls", "cat")...)           // shell (with allowlist)
agent.AddTools(tools.HTTP()...)                                // HTTP requests
agent.AddTools(tools.File("/safe/dir")...)                     // file read/write/list
agent.AddTools(tools.DuckDuckGo()...)                          // web search
agent.AddTools(tools.Wikipedia()...)                           // Wikipedia lookup
agent.AddTools(tools.WebBrowser()...)                          // fetch & read URLs
agent.AddTools(tools.Email("smtp.gmail.com", 465, user, pass, from)...) // SMTP email
agent.AddTools(tools.SQL(db, true)...)                         // SQL queries (read-only)
agent.AddTools(tools.JSON()...)                                // JSON parse/format
agent.AddTools(tools.CSV()...)                                 // CSV → JSON
agent.AddTools(tools.Slack("xoxb-token")...)                   // Slack messaging
agent.AddTools(tools.GitHub("ghp_token")...)                   // GitHub API
agent.AddTools(tools.Docker()...)                              // Docker management
agent.AddTools(tools.GoogleSearch("api-key", "cx-id")...)      // Google search
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
// "My name is Erik" → session.GetMemory("name") == "Erik"
// "erik@example.com" → session.GetMemory("email") == "erik@example.com"
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
// → automatically routed to "booking" agent
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
// 🤖 The answer is 4.
//
// > memory
//   name: Erik
//
// > tools
//   🔧 calculator — Perform math
//   🔧 web_search — Search the web
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
fmt.Println(agent.String()) // "Agent{tools: [calculator, web_search], max_loops: 8}"
```
