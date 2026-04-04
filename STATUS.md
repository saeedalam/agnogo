# agnogo Status -- Comparison with Agno

> Last updated: 2026-03-31

## Overview

agnogo is a Go port of [Agno](https://github.com/agno-agi/agno) (Python, 39k+ stars).
This document tracks feature parity between the two projects.

## Core Agent Features

| Feature | Agno (Python) | agnogo (Go) | Parity |
|---------|--------------|-------------|--------|
| Agent + Run loop | `agent.run()` | `agent.Run()` | Done |
| Async run | `agent.arun()` | goroutines (Go idiom) | Done |
| Continue/resume run | `agent.continue_run()` | `agent.Resume()` | Done |
| Cancel run | `Agent.cancel_run()` | `CancelRun()` | Done |
| Tool registration | `@tool` decorator | `agent.Tool()` | Done |
| Set/clear tools | `set_tools()` | `SetTools()` / `ClearTools()` | Done |
| Bulk tool add | `tools=[t1, t2]` | `agent.AddTools(defs...)` | Done |
| Tool call limit | `tool_call_limit` | duplicate detection (max 2) | Done |
| Tool approval | `@approval` | `ToolWithApproval()` + `Resume()` | Done |
| Input guardrails | `pre_hooks` | `InputGuardrail()` | Done |
| Output guardrails | `post_hooks` | `OutputGuardrail()` | Done |
| Retry with backoff | `retries`, `exponential_backoff` | `RetryConfig` | Done |
| Max loops | `tool_call_limit` | `MaxLoops` | Done |
| Fallback text | custom error message | `FallbackText` | Done |

## Session & Memory

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Session CRUD | `get/save/delete_session` | `GetSession/SaveSession/DeleteSession` | Done |
| List sessions | `get_sessions` | `ListSessions` | Done |
| Session state | `session_state` | `session.State` | Done |
| Chat history | `get_chat_history` | `GetChatHistory` | Done |
| History trimming | `num_history_messages` | `HistoryConfig.MaxMessages` | Done |
| User memories | `get_user_memories` | `GetMemories` | Done |
| Auto memory (pattern) | `update_memory_on_run` | `AutoMemory: true` | Done |
| Auto memory (LLM) | `enable_agentic_memory` | `LLMMemory` | Done |
| Session summaries | `enable_session_summaries` | `WithSummarize(n)` | Done |
| Past session search | `search_past_sessions` | -- | Todo |

## Knowledge & RAG

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Knowledge interface | `Knowledge` protocol | `Knowledge` interface | Done |
| Auto-search for questions | `search_knowledge=True` | Auto-injection via `looksLikeQuestion` | Done |
| Add knowledge | `add_to_knowledge` | `AddKnowledge` | Done |
| Knowledge filters | `knowledge_filters` | -- | Todo |
| Agentic knowledge filters | `enable_agentic_knowledge_filters` | -- | Todo |

## Reasoning

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Reasoning mode | `reasoning=True` | `ReasoningConfig.Enabled` | Done |
| Separate reasoning model | `reasoning_model` | `ReasoningConfig.Model` | Done |
| Min/max steps | `reasoning_min/max_steps` | `MinSteps/MaxSteps` | Done |
| Reasoning agent | `reasoning_agent` | -- (uses model directly) | Todo |

## Teams

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Team with sub-agents | `Team(members=[])` | `NewTeam().Agent()` | Done |
| LLM-based routing | `mode=TeamMode` | `TeamConfig.Model` | Done |
| Custom routing | callable selector | `TeamConfig.RouterFunc` | Done |
| Fallback agent | implicit first | `TeamConfig.Fallback` | Done |
| Nested teams | `Team` as member | -- | Todo |
| Shared history | `share_member_interactions` | -- | Todo |

## Workflows

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Sequential | `Steps([s1, s2])` | `Sequential(step1, step2)` | Done |
| Parallel | `Parallel(steps=[])` | `Parallel(step1, step2)` | Done |
| Loop | `Loop(condition=)` | `Loop(agent, stopFn)` | Done |
| Condition | `Condition(eval, true, false)` | `Condition(eval, true, false)` | Done |
| Router | `Router(selector, routes)` | `Route(selector, routes)` | Done |
| Human confirmation | `requires_confirmation` | via `ToolWithApproval` | Done |
| CEL expressions | `cel.py` | -- | Todo |
| Step input/output chaining | `StepInput.previous_step_outputs` | output -> next input | Done |

## Streaming

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Token-level streaming | `stream=True` | `RunStreamReal()` + `StreamProvider` | Done |
| Word-level fallback | N/A | `RunStream()` | Done |
| Stream events | `RunOutputEvent` types | `StreamEvent` + `StreamChunk` | Done |
| Tool calls in stream | Accumulated across chunks | `ToolCallDelta` accumulation | Done |

## Model Providers

| Provider | Agno | agnogo | Parity |
|----------|------|--------|--------|
| OpenAI | Yes | Yes | Done |
| Anthropic (Claude) | Yes | Yes | Done |
| Google Gemini | Yes | Yes | Done |
| Ollama | Yes | Yes | Done |
| xAI (Grok) | Yes | Yes | Done |
| DeepSeek | Yes | Yes | Done |
| Groq | Yes | Yes | Done |
| Together | Yes | Yes | Done |
| Mistral | Yes | Yes | Done |
| Perplexity | Yes | Yes | Done |
| Azure OpenAI | Yes | -- | Todo |
| Vertex AI | Yes | -- | Todo |
| Cohere | Yes | -- | Todo |
| 28 more niche | Yes | -- | Todo |

## Vector Databases

| VectorDB | Agno | agnogo | Parity |
|----------|------|--------|--------|
| pgvector | Yes | Yes | Done |
| Qdrant | Yes | Yes | Done |
| ChromaDB | Yes | Yes | Done |
| Pinecone | Yes | Yes | Done |
| Milvus | Yes | -- | Todo |
| Weaviate | Yes | -- | Todo |
| Redis (vector) | Yes | -- | Todo |
| 11 more | Yes | -- | Todo |

## Session Storage

| Storage | Agno | agnogo | Parity |
|---------|------|--------|--------|
| In-memory | Yes | Yes | Done |
| PostgreSQL | Yes | Yes | Done |
| SQLite | Yes | Yes | Done |
| Redis | Yes | Yes | Done |
| MySQL | Yes | Yes | Done |
| MongoDB | Yes | -- | Todo |
| DynamoDB | Yes | -- | Todo |
| 6 more | Yes | -- | Todo |

## Built-in Tools

| Tool | Agno | agnogo | Parity |
|------|------|--------|--------|
| Calculator | Yes | Yes | Done |
| Shell | Yes | Yes | Done |
| HTTP request | Yes | Yes | Done |
| File (read/write/list) | Yes | Yes | Done |
| Web browser (fetch URL) | Yes | Yes | Done |
| DuckDuckGo search | Yes | Yes | Done |
| Wikipedia | Yes | Yes | Done |
| Email (SMTP) | Yes | Yes | Done |
| SQL query | Yes | Yes | Done |
| JSON parse/format | Yes | Yes | Done |
| CSV read | Yes | Yes | Done |
| Slack | Yes | Yes | Done |
| GitHub | Yes | Yes | Done |
| Docker | Yes | Yes | Done |
| Google Search | Yes | Yes | Done |
| Env | Yes | Yes | Done |
| Regex | -- | Yes | Go-only |
| Base64 | -- | Yes | Go-only |
| Hash (SHA-256, MD5) | -- | Yes | Go-only |
| UUID | -- | Yes | Go-only |
| TimeTool | -- | Yes | Go-only |
| TemplateTool | -- | Yes | Go-only |
| YAML | -- | Yes | Go-only |
| XML | -- | Yes | Go-only |
| Diff | -- | Yes | Go-only |
| Archive (tar/zip) | -- | Yes | Go-only |
| Crypto (AES) | -- | Yes | Go-only |
| DNS | -- | Yes | Go-only |
| TCP | -- | Yes | Go-only |
| Markdown | -- | Yes | Go-only |
| PDFTool | -- | Yes | Go-only |
| ImageTool | -- | Yes | Go-only |
| CronTool | -- | Yes | Go-only |
| Semver | -- | Yes | Go-only |
| MetricsTool | -- | Yes | Go-only |
| DALL-E | Yes | -- | Todo |
| Discord | Yes | -- | Todo |
| Telegram | Yes | -- | Todo |
| Jira | Yes | -- | Todo |
| 100+ more | Yes | -- | Todo |

## Debug & Observability

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Debug mode | `debug_mode=True` | `DebugConfig` (level 1+2) | Done |
| Trace hooks | AgentOS telemetry | `Trace` (8 hooks) | Done |
| Print response | `print_response()` | `PrintResponse()` | Done |
| CLI app | `cli_app()` | `CLI()` | Done |
| Serialization | `to_dict()`/`from_dict()` | `ToDict()`/`ToJSON()` | Done |
| OpenTelemetry | Yes | `otel.NewExporter()` | Done |
| AgentOS dashboard | Yes | -- (use Trace hooks + OTLP) | Todo |

## Other

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Structured output | `output_schema` | `RunStructured[T]()` | Done |
| MCP tools | `MCPTools` | `mcp.Connect()` | Done |
| Eval framework | -- | `NewEval()` with assertions | Go-only |
| Followup questions | `followups=True` | -- | Todo |
| Learning machine | `learning=True` | -- | Todo |
| Culture manager | experimental | -- | Todo |
| Compression | `compress_tool_results` | -- | Todo |

## Reliability Layer (v0.4.0+)

| Feature | Description | Status |
|---------|-------------|--------|
| `Reliable()` | One-liner: enables all reliability features with sensible defaults | Done |
| Cost management | Per-run, per-session, per-minute budget enforcement with callbacks | Done |
| PII detection | Email, phone, credit card (Luhn), SSN, IP address detection | Done |
| PII guardrails | Block output, redact stored history, custom patterns | Done |
| State machine | 8 states, validated transitions, audit trail, checkpoint/resume | Done |
| Tool validation | Output size limits, non-empty checks, JSON validation | Done |
| Confidence scoring | 0.0-1.0 heuristic (tools, hedging, sources) with retry threshold | Done |
| Hallucination guard | Pattern-based (dates, times, prices, weather) with severity levels | Done |
| Semantic grounding | TF-IDF cosine similarity against tool outputs | Done |
| Hybrid detection | Regex when no tools called, TF-IDF when tools called | Done |
| Pluggable interfaces | `HallucinationChecker`, `PIIScanner`, `ToolOutputValidator`, `ConfidenceScorer` | Done |

## Performance & Graph (v0.6.0)

| Feature | Description | Status |
|---------|-------------|--------|
| Concurrent tool calls | Multiple tool calls fire in parallel via goroutines | Done |
| Async post-processing | Memory, save, summarize run in background goroutine | Done |
| Graph function nodes | `AddFuncNode()` for pure Go nodes between LLM steps | Done |
| Consistency checking | Verify response consistency across multiple runs | Done |

---

## Go-Exclusive Features (Not in Agno Python)

These features exist only in agnogo and have no equivalent in the Python Agno library:

| Feature | Description |
|---------|-------------|
| `Agent()` smart constructor | One-liner agent creation with auto-detected provider from env vars (single import) |
| `WithOpenAI()` / `WithAnthropic()` | Explicit provider selection without manual import of provider packages |
| `Ask()` / `AskStream()` | Session-free one-shot API -- no session management needed |
| `AskStructured[T]()` | Generic one-shot structured output |
| `TypedTool[In, Out]()` | Generic typed tools with struct tags (`desc`, `required`, `enum`) |
| `agent.Serve()` / `Handler()` | Built-in HTTP server with `/ask` and `/health` endpoints |
| `WithMaxConcurrent()` / `WithMaxBodySize()` | Serve hardening: concurrency limits and request body size limits |
| `Then()` / `All()` / `Race()` / `Map()` | Pipeline and concurrency combinators for chaining agents |
| `Fallback()` | Automatic failover between two providers |
| `MultiProvider()` | Try N providers in order until one succeeds |
| `CircuitBreaker()` | Circuit breaker pattern (closed/open/half-open) for providers |
| `RateLimiter()` | Token bucket rate limiting with `Close()` for cleanup |
| `TimeoutProvider()` | Per-request deadline wrapper for providers |
| `Closeable` / `CloseProvider()` | Resource cleanup interface for providers and wrappers |
| `ProviderError` / `ToolError` | Structured errors with `IsRetryable()`, `IsRateLimited()`, `RetryAfter()` (package-level functions) |
| `StreamProvider` | Real SSE streaming for OpenAI-compatible providers |
| `MetricsCollector` | Aggregated telemetry (counts, latencies, costs) with HTTP endpoint |
| `Explain()` | Print human-readable agent configuration summary |
| `Validate()` | Static analysis for common agent misconfigurations |
| `Benchmark()` | Performance benchmarking with warmup, concurrency, and percentiles |
| `WorkerPool` / `Batch()` | Concurrent batch processing with fixed goroutine pool |
| `AgentMiddleware()` | HTTP middleware to inject agent into request context |
| `AgentFromContext()` | Retrieve agent from `context.Context` |
| `AgentHandler()` | Ready-made HTTP handler accepting `{"message":"..."}` POST bodies |
| `HallucinationGuard` | Severity levels ("likely" blocks, "possible" warns), weather/financial/time patterns |
| `NewGraph()` | Graph workflows with conditional edges and shared state |
| `NewRunContext()` / `RunCtx()` | Dependency injection: pass user info, tenant, flags to tools via context |
| `NewEventBus()` / `WithEvents()` | Pub/sub event system for decoupled observability |
| `WithHooks()` | Middleware hook chain wrapping every Run call |
| `WithSummarize(n)` | Auto-summarize old messages to save context window |
| `Reliable()` | One-liner production safety: cost budgets, PII, hallucination, tool validation, confidence |
| `PIIScanner` / `PIIConfig` | Regex PII detection with Luhn validation, redaction, GDPR compliance |
| `CostBudget` | Per-run, per-session, per-minute cost enforcement with callbacks |
| `StateMachine` / `Checkpoint` | Agent lifecycle states with transitions, audit trail, crash recovery |
| `ConfidenceScore` | Heuristic 0.0-1.0 scoring with configurable retry threshold |
| `SemanticHallucinationChecker` | TF-IDF cosine similarity grounding (zero dependencies) |
| `HybridHallucinationChecker` | Regex + TF-IDF combined (regex when no tools, TF-IDF when tools called) |
| Concurrent tool execution | Multiple tool calls fire in parallel via goroutines (automatic) |
| `AsyncPostProcess` | Memory/save/summarize in background — `Run()` returns immediately |
| `AddFuncNode()` | Pure Go function nodes in graphs (no LLM call, zero cost) |
| `mcp.Connect()` | MCP Protocol integration (stdio transport, zero dependencies) |
| `otel.NewExporter()` | OpenTelemetry OTLP export (runs, tokens, errors, latency) |
| `NewEval()` | Agent evaluation framework with assertions and parallel runs |
| `NewWorkflowEngine()` | Structured workflow with StepRunner, data flow, HITL, error modes |
| `WfStep/WfFunc/WfSequence/WfParallel/WfLoop/WfCondition/WfRoute` | Composable step types with nesting |
| `ErrWorkflowPaused` / `ResumeWorkflow()` | Human-in-the-loop pause/resume for workflows |
| `Image` / `Audio` / `File` | Multi-modal types with URL/Path/Bytes + MIME auto-detection |
| `AddMediaMessage()` | Attach images/audio/files to session messages |
| `NativeReasoner` interface | Provider-specific reasoning (O1/O3, Claude thinking, DeepSeek-R1) |
| `NextAction` enum | Reasoning control flow: continue, validate, final_answer, reset |
| `Response.ReasoningSteps` | Chain-of-thought steps persisted in response |
| `LearningMachine` | Self-improving agents with multiple learning stores |
| `UserProfileStore` | Structured user facts with incremental merge |
| `SessionContextStore` | Session summaries (decisions, outcomes, topics) |
| `EntityMemoryStore` | External entity knowledge with fact/event deduplication |
| `SpanCollector` / `RunTrace` | Structured agent tracing — model/tool/guardrail spans with timing, tokens, cost |
| `RunTrace.Print()` / `.JSON()` | Human-readable trace tree + machine-readable JSON export |
| `TraceStore` / `MemoryTraceStore` | Persist traces across restarts, query by cost/errors/session/time |
| `TraceAnalyzer` | Cost summary, anomaly detection (mean+2σ), per-tool stats, error reports |
| `Replay()` / `TraceDiff` | Re-run stored traces with different agents, structured comparison |
| 19 utility tools | regex, base64, hash, uuid, time, env, template, yaml, xml, diff, archive, crypto, dns, tcp, markdown, pdf, image, cron, semver, metrics |

---

## Summary

| Category | Agno | agnogo | Coverage |
|----------|------|--------|----------|
| Core features | 15 | 15 | 100% |
| Session/Memory | 10 | 8 | 80% |
| Knowledge/RAG | 5 | 3 | 60% |
| Reasoning | 4 | 3 | 75% |
| Teams | 5 | 4 | 80% |
| Workflows | 7 | 6 | 86% |
| Streaming | 4 | 4 | 100% |
| Providers | 41 | 10 | 24% |
| Vector DBs | 18 | 4 | 22% |
| Storage | 13 | 5 | 38% |
| Built-in tools | 129 | 35 | 27% |
| Debug/Observability | 7 | 7 | 100% |
| Go-exclusive features | 0 | 58 | -- |
| Tests | | 590+ | |
| Core framework | | | ~98% |
| Including integrations | | | ~50% |

The core agent framework is at ~98% parity with Agno Python. agnogo includes 58 Go-exclusive features spanning reliability (cost/PII/hallucination/confidence), performance (concurrent tools, async post-processing), graph orchestration, structured workflow engine with HITL, multi-modal support, advanced reasoning (native + CoT), self-improving learning machine, MCP protocol, OpenTelemetry export, eval framework, plus Go-native patterns (pipelines, resilience, observability, HTTP serving, batch processing, structured errors).

---

## Remaining High-Priority Tasks

1. **Graph time-travel** -- state snapshots + resume from any node
2. **Graph map-reduce** -- scatter-gather for parallel agent instances
3. **Graph human-in-the-loop** -- approval edges with suspend/resume
4. **Learning machine** -- learn from interactions
5. **MongoDB storage** -- popular NoSQL backend
6. **Azure OpenAI provider** -- enterprise customers
7. **A/B testing** -- traffic splitting for prompts/models

## Completed in v1.0.0

- Learning Machine (`LearningMachine`, `LearningStore` interface)
- `UserProfileStore`: structured user facts with incremental merge
- `SessionContextStore`: session summaries (summary, decisions, outcomes, topics)
- `EntityMemoryStore`: external entity knowledge with fact/event deduplication
- Context injection before model calls, extraction after responses
- `WithLearning(lm)` option for `Agent()` constructor

## Completed in v0.9.0

- Advanced reasoning: `ReasoningAuto`, `ReasoningCoT`, `ReasoningNative` modes
- `NativeReasoner` interface for providers with built-in thinking
- `extractThinking()` for `<think>`/`<thinking>` tag parsing
- `NextAction` enum: `continue`, `validate`, `final_answer`, `reset`
- `Response.ReasoningSteps` — chain-of-thought steps persisted in response
- `WithReasoningConfig()` option with parameterized CoT prompt
- Session history included in reasoning context

## Completed in v0.8.0

- Multi-modal: `Image`, `Audio`, `File` types with URL/Path/Bytes sources
- Constructors: `ImageFromURL`, `ImageFromFile`, `ImageFromBytes`, `AudioFromFile`, `FileFromPath`
- MIME detection from magic bytes (JPEG, PNG, GIF, WebP, PDF)
- Provider formatting: OpenAI (image_url), Anthropic (base64 blocks), Gemini (inline_data)
- `Session.AddMediaMessage()` + `Run(ctx, session, "")` for media messages
- HTTP timeout (30s), status code check, Content-Type charset stripping

## Completed in v0.7.0

- Workflow engine: `StepRunner` interface, `WorkflowEngine`, `StepInput`/`StepOutput`
- Step types: `AgentStep`, `FuncStep`, `Steps`, `ParallelSteps`, `LoopStep`, `ConditionStep`, `RouterStep`
- Error handling: `OnErrorFail`, `OnErrorSkip`, `OnErrorPause`
- HITL: `RequiresConfirmation`, `ErrWorkflowPaused`, `ResumeWorkflow()`
- Retry: `MaxRetries`, `RetryDelay`
- `WorkflowAdapter` for backward compatibility with existing `Workflow` types
- Convenience constructors: `WfStep`, `WfFunc`, `WfSequence`, `WfParallel`, `WfLoop`, `WfCondition`, `WfRoute`

## Completed in v0.6.0

- Concurrent tool execution (parallel goroutines, ordered collection)
- Async post-processing (`AsyncPostProcess` option, `PostProcessDone` channel)
- Graph function nodes (`AddFuncNode()` for pure Go processing)
- Consistency checking between runs

## Completed in v0.5.0

- MCP Protocol integration (`mcp.Connect()`, stdio transport)
- OpenTelemetry export (`otel.NewExporter()`, OTLP metrics)
- Agent evaluation framework (`NewEval()`, assertions, parallel runs)

## Completed in v0.4.0

- `Reliable()` one-liner with pluggable components
- Cost management (`CostBudget`, per-run/session/minute enforcement)
- PII detection and GDPR compliance (`PIIScanner`, `PIIConfig`, Luhn validation)
- Agent state machine (`StateMachine`, `Checkpoint`, crash recovery)
- Tool output validation (`ToolValidator`, size/format/JSON checks)
- Confidence scoring (`ConfidenceScore`, heuristic 0.0-1.0 with retry threshold)
- Semantic hallucination detection (TF-IDF cosine similarity, hybrid mode)
- Pluggable reliability interfaces (`HallucinationChecker`, `PIIScanner`, `ToolOutputValidator`, `ConfidenceScorer`)

## Completed in v0.2.0

- Single import (no more `import _ "autodetect"`)
- Structured errors (`ProviderError`, `ToolError` with fields `Tool`, `Message`, `Err`)
- Package-level error helpers: `IsRetryable()`, `IsRateLimited()`, `RetryAfter()`
- Provider upgrades (connection pooling, 429 handling, Retry-After, `StreamProvider`)
- `Closeable` interface and `CloseProvider()` for resource cleanup
- Serve hardening (`WithMaxConcurrent()`, `WithMaxBodySize()`)
- Hallucination guard severity levels ("likely" blocks, "possible" warns)
- 10 upgraded tools (expression parser calculator, HTML stripping, pagination, configurable limits)
- 19 new utility tools (regex, base64, hash, uuid, time, env, template, yaml, xml, diff, archive, crypto, dns, tcp, markdown, pdf, image, cron, semver, metrics)
- 222 tests (up from 133)
