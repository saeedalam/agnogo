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

### v0.4.0 — Reliability Layer
- Pluggable `Reliable()` one-liner with sensible defaults
- Cost management: per-run, per-session, per-minute budget enforcement with callbacks
- PII detection: emails, phone numbers, credit cards (Luhn), SSNs, IP addresses
- PII guardrails: block output, redact stored history, custom patterns, GDPR compliance
- Agent state machine: explicit state transitions, audit trail, checkpoint/resume
- Tool output validation: size limits, non-empty checks, JSON validation
- Confidence scoring: 0.0-1.0 heuristic scoring with automatic retry below threshold
- Semantic hallucination detection via TF-IDF cosine similarity (+ hybrid mode)
- All components pluggable via interfaces: `HallucinationChecker`, `PIIScanner`, `ToolOutputValidator`, `ConfidenceScorer`, `CostChecker`

### v0.5.0 — MCP, Observability, Eval
- MCP Protocol integration (stdio transport, zero external dependencies)
- OpenTelemetry export (OTLP metrics: runs, tokens, errors, latency, per-tool counts)
- Agent evaluation framework with assertions (`Contains`, `NotContains`, `Exact`, `MatchesRegex`, `Custom`)
- Concurrent eval runs with configurable parallelism

### v0.6.0 — Performance & Graph Orchestration
- Concurrent tool execution: multiple tool calls fire in parallel via goroutines
- Async post-processing: memory extraction, session save, and summarization run in background
- Graph function nodes: `AddFuncNode()` for pure Go data processing between LLM steps
- Consistency checking between runs

## What's Next

### Future

- **A/B testing** — test different prompts/models with traffic splitting
- **Graph time-travel** — state snapshots + resume from any node
- **Graph map-reduce** — scatter-gather pattern for parallel agent instances
- **Graph human-in-the-loop** — approval edges with suspend/resume
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
