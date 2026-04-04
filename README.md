# agnogo

Production-grade AI agent framework for Go. Zero external dependencies.

```go
agent := agnogo.Agent("You are a helpful assistant.")
answer, _ := agent.Ask(ctx, "What is the capital of Sweden?")
fmt.Println(answer) // Stockholm
```

## Install

```bash
go get github.com/saeedalam/agnogo
```

Requires Go 1.24+. No CGo. Single static binary.

## Why agnogo?

| | agnogo | Python frameworks |
|---|---|---|
| **Dependencies** | Zero. Go stdlib only. | 50-200+ pip packages |
| **Deployment** | Single binary, `scp` to server | venv, pip, CUDA, Docker |
| **Concurrency** | Goroutines — real parallelism | GIL — one thread at a time |
| **Safety** | Built-in PII, hallucination, cost guards | Bolt-on or missing |
| **Tracing** | Built-in structured tracing | Requires LangSmith ($39/mo) |
| **Type safety** | Compile-time checks | Runtime errors |

## Features

### Build Agents

```go
// One-liner with auto-detected provider
agent := agnogo.Agent("You are a booking assistant.")

// With explicit provider
agent := agnogo.Agent("...", agnogo.WithOpenAI("gpt-4.1-mini"))

// Full control
agent := agnogo.New(agnogo.Config{
    Model:        openai.New(key, "gpt-4.1-mini"),
    Instructions: "You are a booking assistant.",
})
```

10 providers: OpenAI, Anthropic, Gemini, Groq, DeepSeek, Mistral, Together, Perplexity, Grok, Ollama. [Full list →](GUIDE.md#model-providers)

### Add Tools

```go
agent.Tool("book", "Book an appointment", agnogo.Params{
    "date": {Type: "string", Desc: "Date (YYYY-MM-DD)", Required: true},
    "time": {Type: "string", Desc: "Time (HH:MM)", Required: true},
}, func(ctx context.Context, args map[string]string) (string, error) {
    return bookAppointment(args["date"], args["time"])
})
```

Type-safe tools with generics, 15 built-in tools, 37 contrib integrations. [Tools guide →](GUIDE.md#tools)

### Orchestrate Workflows

```go
// Simple: chain agents sequentially
resp, _ := agent1.Then(agent2).Then(agent3).Run(ctx, session, "input")

// Advanced: structured pipeline with HITL
wf := agnogo.NewWorkflowEngine("pipeline",
    agnogo.WfSequence("main",
        agnogo.WfStep("research", researchAgent),
        agnogo.WfParallel("gather", webAgent, newsAgent, dbAgent),
        agnogo.WfFunc("merge", mergeResults),           // pure Go, no LLM
        agnogo.WfStep("review", editor).WithConfirmation(), // human approval
    ),
)
output, err := wf.RunWorkflow(ctx, session, "Research topic X")
```

Steps: Sequential, Parallel, Loop, Condition, Router. [Workflow guide →](GUIDE.md#workflow-engine)

### Production Safety

```go
agent := agnogo.Agent("...", agnogo.Reliable()) // one line enables everything
```

- **Hallucination guard** — blocks fabricated dates, prices, stats. Pattern + TF-IDF semantic grounding.
- **PII detection** — emails, phones, credit cards (Luhn), SSNs, IPs. Block output or redact history.
- **Cost budgets** — per-run, per-session, per-minute limits with alerts.
- **Confidence scoring** — 0.0–1.0 score on every response. Auto-retry below threshold.
- **Tool validation** — reject empty, oversized, or malformed tool output.

Every component is pluggable via interfaces. [Reliability guide →](GUIDE.md#reliability-layer)

### Structured Tracing

See inside every `Run()`. No SaaS, no setup, no dependencies.

```go
sc := agnogo.NewSpanCollector()
agent := agnogo.Agent("...", agnogo.WithSpanCollector(sc))
resp, _ := agent.Run(ctx, session, "Book Thursday 2pm")
sc.Collect(resp).Print()
```

```
[run r_f17c] 2.5s | $0.0002 | 388 tok | 2 model | 1 tool
  ├─ [model]  call      1.3s  179 tok  $0.0001
  ├─ [tool]   get_time  <1ms  → "10:30 AM"
  └─ [model]  call      1.2s  209 tok  $0.0001
```

Persist traces, query by cost/errors, detect anomalies, replay with different models. [Tracing guide →](GUIDE.md#structured-tracing)

### Multi-Modal

```go
session.AddMediaMessage("user", "What's in this image?",
    []agnogo.Image{agnogo.ImageFromURL("https://example.com/photo.jpg")}, nil, nil)
resp, _ := agent.Run(ctx, session, "")
```

Images, audio, files. Auto-formatted for OpenAI, Anthropic, Gemini. MIME detection from magic bytes. [Multi-modal guide →](GUIDE.md#multi-modal-support)

### Advanced Reasoning

```go
agent := agnogo.Agent("...", agnogo.Reasoning) // chain-of-thought
agent := agnogo.Agent("...", agnogo.WithReasoningConfig(agnogo.ReasoningConfig{
    Mode: agnogo.ReasoningNative, // O1/O3, Claude thinking, DeepSeek-R1
}))
```

Auto-detects native reasoning models. Steps persisted in `resp.ReasoningSteps`. [Reasoning guide →](GUIDE.md#advanced-reasoning)

### Self-Improving Agents

```go
lm := agnogo.NewLearningMachine(model)
lm.AddStore(agnogo.NewUserProfileStore())     // remembers who you are
lm.AddStore(agnogo.NewEntityMemoryStore())     // builds knowledge graph
agent := agnogo.Agent("...", agnogo.WithLearning(lm))
```

First conversation: knows nothing. Tenth conversation: remembers your name, preferences, and every entity discussed. [Learning guide →](GUIDE.md#learning-machine)

### HTTP Server

```go
agent.Serve(":8080") // POST /ask, GET /health, SSE streaming
```

Built-in CORS, auth, concurrency limits, body size limits. [Server guide →](GUIDE.md#http-server)

### Pipelines & Concurrency

```go
resp, _ := agnogo.All(agent1, agent2, agent3).Run(ctx, session, "input")  // parallel
resp, _ := agnogo.Race(fast, slow, fallback).Run(ctx, session, "query")   // first wins
results := agnogo.Map(ctx, agent, inputs, 10)                             // map over inputs
```

[Pipelines guide →](GUIDE.md#pipelines-and-concurrency)

### Graph Workflows

```go
g := agnogo.NewGraph()
g.AddNode("classify", classifier).AddNode("refund", refundAgent)
g.AddFuncNode("transform", transformFn) // pure Go node, zero cost
g.SetEntry("classify").SetEnd("refund")
g.AddEdge("classify", "refund", conditionFn)
resp, _ := g.Run(ctx, session, "I want a refund")
```

Conditional edges, cycles, function nodes. [Graph guide →](GUIDE.md#graph-workflows)

## Architecture

```
agnogo/
  agent.go             Core agent engine (Run loop, concurrent tools)
  session.go           Session state, memory, history
  tool.go              Tool registry, typed tools
  provider.go          ModelProvider interface, 10 providers
  smart.go             Agent() constructor, auto-detection

  wfengine.go          Workflow engine (StepRunner, HITL)
  wfsteps.go           Step types (Agent, Func, Parallel, Loop, Condition, Router)
  graph.go             Graph workflows with conditional edges
  pipeline.go          Then, All, Race, Map

  spans.go             Structured tracing (SpanCollector, RunTrace)
  tracestore.go        Trace persistence (TraceStore, MemoryTraceStore)
  tracestore_file.go   File-based trace persistence
  traceintel.go        Anomaly detection, cost analysis, tool stats
  tracereplay.go       Replay traces with different agents

  reliability.go       Reliable() one-liner
  hallucination.go     Pattern-based hallucination detection
  hallucination_semantic.go  TF-IDF semantic grounding
  pii.go               PII detection, redaction, GDPR
  cost.go              Cost budgets and tracking
  confidence.go        Response confidence scoring
  toolvalidate.go      Tool output validation

  reasoning.go         Chain-of-thought, native reasoning
  learn.go             Learning machine (UserProfile, EntityMemory)
  media.go             Multi-modal (Image, Audio, File)
  eval.go              Agent evaluation framework

  serve.go             HTTP server
  streaming.go         SSE streaming
  resilience.go        Circuit breaker, rate limiter, fallback
  observe.go           Metrics collection
  events.go            Event bus

  providers/           Provider subpackages
  tools/               Built-in tools (15 core + 37 contrib)
  mcp/                 Model Context Protocol
  otel/                OpenTelemetry export
  cookbook/             Examples and tutorials
```

## Cookbook

| Example | What you learn |
|---------|---------------|
| [14_research_analyst](cookbook/14_research_analyst/) | Full pipeline: parallel research, HITL, learning, reasoning |
| [15_tracing](cookbook/15_tracing/) | Structured tracing: store, query, analyze, replay |
| [01_basics](cookbook/01_basics/) | First agent, tools, sessions |
| [04_workflows](cookbook/04_workflows/) | Sequential, parallel, loops |
| [05_guardrails](cookbook/05_guardrails/) | Input/output safety |

## Documentation

- **[Quick Start](QUICKSTART.md)** — 5 minutes to your first agent
- **[Complete Guide](GUIDE.md)** — Every feature with examples
- **[Roadmap](ROADMAP.md)** — What's shipped, what's next
- **[Changelog](CHANGELOG.md)** — Version history
- **[Contributing](CONTRIBUTING.md)** — How to contribute

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
