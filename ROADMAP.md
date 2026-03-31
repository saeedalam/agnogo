# agnogo Roadmap

## Vision

Every agent framework focuses on features. agnogo focuses on **reliability** — making AI agents safe enough for production. Agents that won't burn your money, leak your data, or hallucinate to your customers.

## What's Shipped

### v0.1.0 — Foundation
- One-line agent creation with auto-provider detection
- 10 LLM providers (OpenAI, Anthropic, Gemini, Ollama, Groq, DeepSeek, Mistral, Together, Perplexity, Grok)
- Typed tools with generics (`TypedTool[In, Out]`)
- HTTP server (`agent.Serve()`)
- Pipelines, parallel fan-out, race (`Then`, `All`, `Race`, `Map`)
- Circuit breaker, fallback, rate limiter for providers
- Observability (MetricsCollector, CostTracker, EventBus)
- Batch processing (WorkerPool, Benchmark)

### v0.2.0 — Production Hardening
- Single import (no more `autodetect` package)
- Structured errors (ProviderError, ToolError, IsRetryable)
- Provider deduplication (shared HTTP logic)
- Real streaming for OpenAI, Anthropic, Gemini
- Panic recovery in tools and hooks
- 14 adversarial tests (malformed args, panics, races, 429s)
- Session summarization with topic extraction and recall
- Hallucination guard with severity levels
- Graph workflows with conditional edges and cycles

### v0.3.0 — Tool Ecosystem
- 15 core tools (maintained, production-grade)
- 37 contrib API integrations (Discord, Telegram, WhatsApp, Jira, Notion, Linear, GitLab, Reddit, YouTube, etc.)
- Middleware hooks for pre/post processing
- Event bus with OnAll, Filter, EventCount
- RunContext for dependency injection into tools

## What's Next

### v0.4.0 — Reliability Layer (in progress)

The features that make agents safe for production. No other framework does this well.

#### Cost Management
Stop runaway agents from burning money.

```go
agent := agnogo.Agent("...", agnogo.WithBudget(agnogo.CostBudget{
    MaxPerRun:     0.50,  // max $0.50 per conversation turn
    MaxPerSession: 5.00,  // max $5 per session lifetime
    MaxPerMinute:  2.00,  // rate limit on spend
    OnExceeded: func(spent, limit float64) {
        alert.Send(fmt.Sprintf("Budget exceeded: $%.2f / $%.2f", spent, limit))
    },
}))
```

- Real-time cost tracking in the agent loop
- Budget enforcement: stops mid-run if limit exceeded
- Per-session cumulative cost tracking
- Cost alerts and callbacks

#### PII Detection + GDPR Compliance
Don't let agents leak personal data.

```go
agent := agnogo.Agent("...", agnogo.WithPIIGuard(agnogo.PIIConfig{
    BlockOutput:   true,              // block PII in agent responses
    RedactInput:   true,              // redact PII from stored history
    AllowedFields: []string{"email"}, // user consented to email sharing
    OnDetected:    auditLog,          // compliance audit trail
}))
```

- Detect: emails, phone numbers, credit cards (Luhn), SSNs, IP addresses
- Block PII in output or redact from stored history
- GDPR helpers: `PurgeUserData()`, `ExportUserData()`, consent tracking
- Custom PII patterns for domain-specific data

#### Agent State Machine
Know exactly what your agent is doing.

```go
States: idle → processing → waiting_tool → waiting_approval → complete
                         → error → budget_exceeded
```

- Explicit state transitions with validation
- Audit trail of all state changes
- Checkpoint and resume: crash recovery mid-conversation
- State-based hooks: run code on state entry/exit

#### Tool Output Validation
Don't trust tool results blindly.

```go
agent := agnogo.Agent("...", agnogo.WithToolValidation(agnogo.ToolValidator{
    MaxOutputSize:   50000,  // reject tool output over 50KB
    RequireNonEmpty: true,   // reject empty results
    JSONValidate:    true,   // validate JSON is well-formed
}))
```

- Validate tool output size, format, non-emptiness
- Smart loop detection: detect A-B-A-B cycling patterns, not just same-call repeats
- Auto-retry with different prompt when tool returns garbage

#### Confidence Scoring
Know when to trust the agent's response.

```go
agent := agnogo.Agent("...", agnogo.WithConfidence(0.5))
// Responses below 0.5 confidence trigger automatic retry with tool instructions
```

- Score 0.0-1.0 based on: tool usage, hallucination check, hedging language, source count
- Configurable threshold: below threshold triggers retry
- Confidence metadata on every response

#### One-Line Production Safety

```go
// Enable ALL reliability features with sensible defaults
agent := agnogo.Agent("...", agnogo.Reliable())

// Equivalent to:
// - Cost budget: $1/run, $10/session
// - PII guard: block output, redact input
// - Tool validation: non-empty, JSON check, 50KB limit
// - Confidence threshold: 0.5
// - Enhanced loop detection
// - Hallucination guard (already default)
```

### Future (v0.5.0+)

- **MCP Protocol** — Model Context Protocol integration for standardized tool interop
- **OpenTelemetry export** — plug into existing observability stacks
- **A/B testing** — test different prompts/models with traffic splitting
- **Agent evaluation** — automated quality scoring of agent responses
- **Multi-turn planning** — agent plans multiple steps before executing
- **Long-term memory** — cross-session memory with embedding search

## Philosophy

1. **Reliability over features** — a boring agent that always works beats a fancy one that sometimes crashes
2. **Zero dependencies** — every feature uses Go stdlib only
3. **Honest tooling** — core tools are maintained; contrib tools are best-effort
4. **Go-native patterns** — context propagation, interfaces, goroutines — not Python translated to Go
5. **Production-first** — cost management, PII detection, state machines are not afterthoughts

## Contributing

The reliability layer is where contributions matter most. If you've run agents in production and know what breaks, open an issue or PR.

- Core tools: maintained by us, high bar for quality
- Contrib tools: community-maintained, lower bar, PRs welcome
- Reliability features: the differentiator — contributions here shape the project's direction
