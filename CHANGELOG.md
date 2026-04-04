# Changelog

## v1.0.2 — Public Release Cleanup
- Removed internal planning documents and build artifacts
- Updated .gitignore for public release
- Added LICENSE, CONTRIBUTING.md, CHANGELOG.md
- Fixed test count in STATUS.md (590+)

## v1.0.1 — Structured Agent Tracing + Trace Intelligence
- `SpanCollector`: zero-config structured tracing for every `Run()`
- `TraceStore` interface with `MemoryTraceStore` and `FileTraceStore`
- `TraceAnalyzer`: cost summaries, anomaly detection (mean+2σ), tool stats
- `Replay()`: re-run stored traces with different agents, get structured diff
- `CostTrend`: detect gradual cost drift between time windows
- `OnRunStart` trace hook for reliable session ID capture
- `Model` field in `ModelResponse` for accurate per-provider cost estimation
- JSON round-trip fixes for Duration, SpanKind, SpanStatus serialization

## v1.0.0 — Learning Machine
- `LearningMachine` coordinates multiple `LearningStore` implementations
- `UserProfileStore`: structured user facts with incremental merge
- `SessionContextStore`: session summaries (decisions, outcomes, topics)
- `EntityMemoryStore`: external entity knowledge with fact/event deduplication
- Context injection before model calls, extraction after responses

## v0.9.0 — Advanced Reasoning
- Three modes: Auto (detect native), CoT (chain-of-thought), Native (O1/O3/Claude)
- `NativeReasoner` interface for provider-specific thinking
- `NextAction` enum: continue, validate, final_answer, reset
- `Response.ReasoningSteps` for analytics and UI rendering
- Session history included in reasoning context

## v0.8.0 — Multi-Modal Support
- `Image`, `Audio`, `File` types with URL/Path/Bytes sources
- Provider formatting: OpenAI (image_url), Anthropic (base64), Gemini (inline_data)
- MIME detection from magic bytes (JPEG, PNG, GIF, WebP, PDF)
- `Session.AddMediaMessage()` for attaching media

## v0.7.0 — Workflow Engine
- `StepRunner` interface with composable step types
- Steps: AgentStep, FuncStep, Steps, ParallelSteps, LoopStep, ConditionStep, RouterStep
- Error handling: OnErrorFail, OnErrorSkip, OnErrorPause
- HITL: RequiresConfirmation, ErrWorkflowPaused, ResumeWorkflow
- WorkflowAdapter for backward compatibility

## v0.6.0 — Performance & Graph
- Concurrent tool execution via goroutines
- Async post-processing (AsyncPostProcess option)
- Graph function nodes (AddFuncNode)
- Consistency checking between runs

## v0.5.0 — MCP, Observability, Eval
- MCP Protocol integration (stdio transport)
- OpenTelemetry export (OTLP metrics)
- Agent evaluation framework with assertions

## v0.4.0 — Reliability Layer
- `Reliable()` one-liner with pluggable components
- Cost management, PII/GDPR, state machine, tool validation
- Confidence scoring, semantic hallucination detection
- Pluggable interfaces for all components

## v0.3.0 — Tool Ecosystem
- 15 core tools, 37 contrib integrations
- Middleware hooks, event bus, RunContext

## v0.2.0 — Production Hardening
- Structured errors, real streaming, panic recovery
- Hallucination guard with severity levels
- Graph workflows, session summarization

## v0.1.0 — Foundation
- Agent creation, 10 LLM providers, typed tools
- HTTP server, pipelines, resilience patterns
- Observability, batch processing
