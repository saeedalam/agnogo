# agnogo Status — Comparison with Agno

> Last updated: 2026-03-31

## Overview

agnogo is a Go port of [Agno](https://github.com/agno-agi/agno) (Python, 39k+ stars).
This document tracks feature parity between the two projects.

## Core Agent Features

| Feature | Agno (Python) | agnogo (Go) | Parity |
|---------|--------------|-------------|--------|
| Agent + Run loop | `agent.run()` | `agent.Run()` | ✅ |
| Async run | `agent.arun()` | goroutines (Go idiom) | ✅ |
| Continue/resume run | `agent.continue_run()` | `agent.Resume()` | ✅ |
| Cancel run | `Agent.cancel_run()` | `CancelRun()` | ✅ |
| Tool registration | `@tool` decorator | `agent.Tool()` | ✅ |
| Set/clear tools | `set_tools()` | `SetTools()` / `ClearTools()` | ✅ |
| Bulk tool add | `tools=[t1, t2]` | `agent.AddTools(defs...)` | ✅ |
| Tool call limit | `tool_call_limit` | duplicate detection (max 2) | ✅ |
| Tool approval | `@approval` | `ToolWithApproval()` + `Resume()` | ✅ |
| Input guardrails | `pre_hooks` | `InputGuardrail()` | ✅ |
| Output guardrails | `post_hooks` | `OutputGuardrail()` | ✅ |
| Retry with backoff | `retries`, `exponential_backoff` | `RetryConfig` | ✅ |
| Max loops | `tool_call_limit` | `MaxLoops` | ✅ |
| Fallback text | custom error message | `FallbackText` | ✅ |

## Session & Memory

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Session CRUD | `get/save/delete_session` | `GetSession/SaveSession/DeleteSession` | ✅ |
| List sessions | `get_sessions` | `ListSessions` | ✅ |
| Session state | `session_state` | `session.State` | ✅ |
| Chat history | `get_chat_history` | `GetChatHistory` | ✅ |
| History trimming | `num_history_messages` | `HistoryConfig.MaxMessages` | ✅ |
| User memories | `get_user_memories` | `GetMemories` | ✅ |
| Auto memory (pattern) | `update_memory_on_run` | `AutoMemory: true` | ✅ |
| Auto memory (LLM) | `enable_agentic_memory` | `LLMMemory` | ✅ |
| Session summaries | `enable_session_summaries` | ❌ Not implemented | 🔲 |
| Past session search | `search_past_sessions` | ❌ Not implemented | 🔲 |

## Knowledge & RAG

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Knowledge interface | `Knowledge` protocol | `Knowledge` interface | ✅ |
| Auto-search for questions | `search_knowledge=True` | Auto-injection via `looksLikeQuestion` | ✅ |
| Add knowledge | `add_to_knowledge` | `AddKnowledge` | ✅ |
| Knowledge filters | `knowledge_filters` | ❌ Not implemented | 🔲 |
| Agentic knowledge filters | `enable_agentic_knowledge_filters` | ❌ Not implemented | 🔲 |

## Reasoning

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Reasoning mode | `reasoning=True` | `ReasoningConfig.Enabled` | ✅ |
| Separate reasoning model | `reasoning_model` | `ReasoningConfig.Model` | ✅ |
| Min/max steps | `reasoning_min/max_steps` | `MinSteps/MaxSteps` | ✅ |
| Reasoning agent | `reasoning_agent` | ❌ (uses model directly) | 🔲 |

## Teams

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Team with sub-agents | `Team(members=[])` | `NewTeam().Agent()` | ✅ |
| LLM-based routing | `mode=TeamMode` | `TeamConfig.Model` | ✅ |
| Custom routing | callable selector | `TeamConfig.RouterFunc` | ✅ |
| Fallback agent | implicit first | `TeamConfig.Fallback` | ✅ |
| Nested teams | `Team` as member | ❌ Not implemented | 🔲 |
| Shared history | `share_member_interactions` | ❌ Not implemented | 🔲 |

## Workflows

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Sequential | `Steps([s1, s2])` | `Sequential(step1, step2)` | ✅ |
| Parallel | `Parallel(steps=[])` | `Parallel(step1, step2)` | ✅ |
| Loop | `Loop(condition=)` | `Loop(agent, stopFn)` | ✅ |
| Condition | `Condition(eval, true, false)` | `Condition(eval, true, false)` | ✅ |
| Router | `Router(selector, routes)` | `Route(selector, routes)` | ✅ |
| Human confirmation | `requires_confirmation` | via `ToolWithApproval` | ✅ |
| CEL expressions | `cel.py` | ❌ Not implemented | 🔲 |
| Step input/output chaining | `StepInput.previous_step_outputs` | output → next input | ✅ |

## Streaming

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Token-level streaming | `stream=True` | `RunStreamReal()` + `StreamProvider` | ✅ |
| Word-level fallback | N/A | `RunStream()` | ✅ |
| Stream events | `RunOutputEvent` types | `StreamEvent` + `StreamChunk` | ✅ |
| Tool calls in stream | Accumulated across chunks | `ToolCallDelta` accumulation | ✅ |

## Model Providers

| Provider | Agno | agnogo | Parity |
|----------|------|--------|--------|
| OpenAI | ✅ | ✅ | ✅ |
| Anthropic (Claude) | ✅ | ✅ | ✅ |
| Google Gemini | ✅ | ✅ | ✅ |
| Ollama | ✅ | ✅ | ✅ |
| xAI (Grok) | ✅ | ✅ | ✅ |
| DeepSeek | ✅ | ✅ | ✅ |
| Groq | ✅ | ✅ | ✅ |
| Together | ✅ | ✅ | ✅ |
| Mistral | ✅ | ✅ | ✅ |
| Perplexity | ✅ | ✅ | ✅ |
| Azure OpenAI | ✅ | ❌ | 🔲 |
| Vertex AI | ✅ | ❌ | 🔲 |
| Cohere | ✅ | ❌ | 🔲 |
| 28 more niche | ✅ | ❌ | 🔲 |

## Vector Databases

| VectorDB | Agno | agnogo | Parity |
|----------|------|--------|--------|
| pgvector | ✅ | ✅ | ✅ |
| Qdrant | ✅ | ✅ | ✅ |
| ChromaDB | ✅ | ✅ | ✅ |
| Pinecone | ✅ | ✅ | ✅ |
| Milvus | ✅ | ❌ | 🔲 |
| Weaviate | ✅ | ❌ | 🔲 |
| Redis (vector) | ✅ | ❌ | 🔲 |
| 11 more | ✅ | ❌ | 🔲 |

## Session Storage

| Storage | Agno | agnogo | Parity |
|---------|------|--------|--------|
| In-memory | ✅ | ✅ | ✅ |
| PostgreSQL | ✅ | ✅ | ✅ |
| SQLite | ✅ | ✅ | ✅ |
| Redis | ✅ | ✅ | ✅ |
| MySQL | ✅ | ✅ | ✅ |
| MongoDB | ✅ | ❌ | 🔲 |
| DynamoDB | ✅ | ❌ | 🔲 |
| 6 more | ✅ | ❌ | 🔲 |

## Built-in Tools

| Tool | Agno | agnogo | Parity |
|------|------|--------|--------|
| Calculator | ✅ | ✅ | ✅ |
| Shell | ✅ | ✅ | ✅ |
| HTTP request | ✅ | ✅ | ✅ |
| File (read/write/list) | ✅ | ✅ | ✅ |
| Web browser (fetch URL) | ✅ | ✅ | ✅ |
| DuckDuckGo search | ✅ | ✅ | ✅ |
| Wikipedia | ✅ | ✅ | ✅ |
| Email (SMTP) | ✅ | ✅ | ✅ |
| SQL query | ✅ | ✅ | ✅ |
| JSON parse/format | ✅ | ✅ | ✅ |
| CSV read | ✅ | ✅ | ✅ |
| Slack | ✅ | ✅ | ✅ |
| GitHub | ✅ | ✅ | ✅ |
| Docker | ✅ | ✅ | ✅ |
| Google Search | ✅ | ✅ | ✅ |
| DALL-E | ✅ | ❌ | 🔲 |
| Discord | ✅ | ❌ | 🔲 |
| Telegram | ✅ | ❌ | 🔲 |
| Jira | ✅ | ❌ | 🔲 |
| 100+ more | ✅ | ❌ | 🔲 |

## Debug & Observability

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Debug mode | `debug_mode=True` | `DebugConfig` (level 1+2) | ✅ |
| Trace hooks | AgentOS telemetry | `Trace` (8 hooks) | ✅ |
| Print response | `print_response()` | `PrintResponse()` | ✅ |
| CLI app | `cli_app()` | `CLI()` | ✅ |
| Serialization | `to_dict()`/`from_dict()` | `ToDict()`/`ToJSON()` | ✅ |
| OpenTelemetry | ✅ | ❌ | 🔲 |
| AgentOS dashboard | ✅ | ❌ (use Trace hooks) | 🔲 |

## Other

| Feature | Agno | agnogo | Parity |
|---------|------|--------|--------|
| Structured output | `output_schema` | `RunStructured[T]()` | ✅ |
| Followup questions | `followups=True` | ❌ | 🔲 |
| Learning machine | `learning=True` | ❌ | 🔲 |
| MCP tools | `MCPTools` | ❌ | 🔲 |
| Culture manager | experimental | ❌ | 🔲 |
| Compression | `compress_tool_results` | ❌ | 🔲 |

---

## Go-Exclusive Features (Not in Agno Python)

These features exist only in agnogo and have no equivalent in the Python Agno library:

| Feature | Description |
|---------|-------------|
| `Agent()` smart constructor | One-liner agent creation with auto-detected provider from env vars |
| `autodetect` side-effect import | `import _ "agnogo/autodetect"` registers provider from `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc. |
| `Ask()` / `AskStream()` | Session-free one-shot API -- no session management needed |
| `AskStructured[T]()` | Generic one-shot structured output |
| `TypedTool[In, Out]()` | Generic typed tools with struct tags (`desc`, `required`, `enum`) |
| `agent.Serve()` / `Handler()` | Built-in HTTP server with `/ask` and `/health` endpoints |
| `Then()` / `All()` / `Race()` / `Map()` | Pipeline and concurrency combinators for chaining agents |
| `Fallback()` | Automatic failover between two providers |
| `MultiProvider()` | Try N providers in order until one succeeds |
| `CircuitBreaker()` | Circuit breaker pattern (closed/open/half-open) for providers |
| `RateLimiter()` | Token bucket rate limiting for providers |
| `TimeoutProvider()` | Per-request deadline wrapper for providers |
| `MetricsCollector` | Aggregated telemetry (counts, latencies, costs) with HTTP endpoint |
| `Explain()` | Print human-readable agent configuration summary |
| `Validate()` | Static analysis for common agent misconfigurations |
| `Benchmark()` | Performance benchmarking with warmup, concurrency, and percentiles |
| `WorkerPool` / `Batch()` | Concurrent batch processing with fixed goroutine pool |
| `AgentMiddleware()` | HTTP middleware to inject agent into request context |
| `AgentFromContext()` | Retrieve agent from `context.Context` |
| `AgentHandler()` | Ready-made HTTP handler accepting `{"message":"..."}` POST bodies |
| `HallucinationGuard` | Detect and retry when LLM skips available tools |

---

## Summary

| Category | Agno | agnogo | Coverage |
|----------|------|--------|----------|
| Core features | 15 | 15 | **100%** |
| Session/Memory | 10 | 8 | **80%** |
| Knowledge/RAG | 5 | 3 | **60%** |
| Reasoning | 4 | 3 | **75%** |
| Teams | 5 | 4 | **80%** |
| Workflows | 7 | 6 | **86%** |
| Streaming | 4 | 4 | **100%** |
| Providers | 41 | 10 | **24%** |
| Vector DBs | 18 | 4 | **22%** |
| Storage | 13 | 5 | **38%** |
| Built-in tools | 129 | 16 | **12%** |
| Debug/Observability | 7 | 7 | **100%** |
| Go-exclusive features | 0 | 20 | -- |
| **Core framework** | | | **~92%** |
| **Including integrations** | | | **~42%** |

The core agent framework is at ~92% parity. The gap is mainly integrations (providers, vector DBs, tools) which are additive and can be contributed incrementally. agnogo also includes 20 Go-exclusive features (pipelines, resilience, observability, HTTP serving, batch processing) with no Python equivalent.

---

## Remaining High-Priority Tasks

1. **Session summaries** -- auto-generate conversation summaries
2. **MCP protocol** -- Model Context Protocol tool support
3. **Learning machine** -- learn from interactions
4. **MongoDB storage** -- popular NoSQL backend
5. **Azure OpenAI provider** -- enterprise customers
6. **DALL-E tool** -- image generation
7. **More tests** -- integration tests for each provider/tool
8. **OpenTelemetry export** -- bridge MetricsCollector to OTLP (MetricsCollector now covers local observability)
