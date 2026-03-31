# agnogo Cookbook

Runnable examples for every major feature. Each file is a standalone `main()` -- set your API key and `go run` it.

All examples use the new `agnogo.Agent()` constructor which auto-detects your provider from environment variables.

## Prerequisites

```bash
export OPENAI_API_KEY=sk-...   # Or any supported provider key
```

## Examples

### 01 -- Basics
| Example | What it demonstrates |
|---------|---------------------|
| `01_basics/simple_agent.go` | Hello world -- 5-line agent with CLI |
| `01_basics/agent_with_tools.go` | Agent with weather and time tools |
| `01_basics/structured_output.go` | Parse LLM response into a Go struct |

### 02 -- Memory and Knowledge
| Example | What it demonstrates |
|---------|---------------------|
| `02_memory_knowledge/auto_memory.go` | Pattern-based memory extraction (name, email) |
| `02_memory_knowledge/knowledge_rag.go` | RAG injection from a knowledge base |
| `02_memory_knowledge/session_storage.go` | Persist sessions across turns |

### 03 -- Teams
| Example | What it demonstrates |
|---------|---------------------|
| `03_teams/router_team.go` | Multi-agent team with intent-based routing |

### 04 -- Workflows
| Example | What it demonstrates |
|---------|---------------------|
| `04_workflows/sequential.go` | Pipeline: extract, translate, summarize |
| `04_workflows/parallel.go` | Fan-out: weather + news + calendar simultaneously |
| `04_workflows/loop.go` | Iterative refinement until "FINAL:" |
| `04_workflows/condition.go` | If/else branching (urgent vs normal) |
| `04_workflows/router.go` | Dynamic routing to refund/tech/sales |

### 05 -- Guardrails
| Example | What it demonstrates |
|---------|---------------------|
| `05_guardrails/input_output_guard.go` | Block injection + redact PII |

### 06 -- Streaming
| Example | What it demonstrates |
|---------|---------------------|
| `06_streaming/stream_response.go` | Word-by-word streaming with AskStream |

### 07 -- Advanced
| Example | What it demonstrates |
|---------|---------------------|
| `07_advanced/human_approval.go` | Human-in-the-loop tool approval |
| `07_advanced/cancel_run.go` | Cancel agent execution mid-flight |
| `07_advanced/cli_agent.go` | Interactive terminal with memory and tools |
| `07_advanced/builtin_tools.go` | Calculator and JSON tools from tools package |

### 08 -- Providers
| Example | What it demonstrates |
|---------|---------------------|
| `08_providers/multi_provider.go` | Auto-detect or explicitly select OpenAI/Claude/Gemini/Ollama |

### 09 -- Easy Mode (New in v0.2.0)
| Example | What it demonstrates |
|---------|---------------------|
| `09_easy/one_liner.go` | Simplest agent: `Agent()` + `Ask()` in 10 lines |
| `09_easy/typed_tools.go` | Type-safe tools with `TypedTool[In, Out]()` generics |
| `09_easy/http_server.go` | Serve agent as HTTP API with `agent.Serve()` |
| `09_easy/pipeline.go` | Chain agents with `Then()` for sequential pipelines |
| `09_easy/resilience.go` | Provider failover with `Fallback()` + `CircuitBreaker()` |
| `09_easy/batch.go` | Parallel batch processing with `Batch()` |

### 10 -- Production Features (New in v0.2.0)
| Example | What it demonstrates |
|---------|---------------------|
| `10_production/graph_workflow.go` | Graph workflow: classify/refund/support branching |
| `10_production/run_context.go` | Dependency injection: tools access user info via context |
| `10_production/event_bus.go` | Event-driven observability with pub/sub |
| `10_production/middleware.go` | Hook chain: timing + logging middleware on every run |
| `10_production/summarize.go` | Auto-summarize old messages to save context window |

## Running

```bash
# From the repo root:
go run ./cookbook/01_basics/simple_agent.go
go run ./cookbook/09_easy/one_liner.go
go run ./cookbook/09_easy/typed_tools.go

# With verbose debug:
AGNOGO_DEBUG=true AGNOGO_DEBUG_LEVEL=2 go run ./cookbook/01_basics/simple_agent.go

# Different providers:
ANTHROPIC_API_KEY=sk-ant-... go run ./cookbook/08_providers/multi_provider.go
OLLAMA_HOST=http://localhost:11434 go run ./cookbook/09_easy/one_liner.go
```

## Debug Output

Examples using `WithDebug()` show:
- Run start/end with run ID and session ID
- Model calls with latency
- Tool calls with status and timing
- Token usage (input/output/total)
- Total run duration
