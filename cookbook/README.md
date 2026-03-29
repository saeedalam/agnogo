# agnogo Cookbook

Runnable examples for every major feature. Each file is a standalone `main()` — just set your API key and `go run` it.

## Prerequisites

```bash
export OPENAI_API_KEY=sk-...   # Required for most examples
```

## Examples

### 01 — Basics
| Example | What it demonstrates |
|---------|---------------------|
| `01_basics/simple_agent.go` | Hello world — agent with debug output and metrics |
| `01_basics/agent_with_tools.go` | Agent with weather and time tools |
| `01_basics/structured_output.go` | Force typed JSON responses |

### 02 — Memory & Knowledge
| Example | What it demonstrates |
|---------|---------------------|
| `02_memory_knowledge/auto_memory.go` | Pattern-based memory extraction (name, email) |
| `02_memory_knowledge/knowledge_rag.go` | RAG injection from a knowledge base |
| `02_memory_knowledge/session_storage.go` | Persist sessions and knowledge across turns |

### 03 — Teams
| Example | What it demonstrates |
|---------|---------------------|
| `03_teams/router_team.go` | Multi-agent team with intent-based routing |

### 04 — Workflows
| Example | What it demonstrates |
|---------|---------------------|
| `04_workflows/sequential.go` | Pipeline: extract → translate → summarize |
| `04_workflows/parallel.go` | Fan-out: weather + news + calendar simultaneously |
| `04_workflows/loop.go` | Iterative refinement until "FINAL:" |
| `04_workflows/condition.go` | If/else branching (urgent vs normal) |
| `04_workflows/router.go` | Dynamic routing to refund/tech/sales |

### 05 — Guardrails
| Example | What it demonstrates |
|---------|---------------------|
| `05_guardrails/input_output_guard.go` | Block injection + redact PII |

### 06 — Streaming
| Example | What it demonstrates |
|---------|---------------------|
| `06_streaming/stream_response.go` | Word-by-word streaming output |

### 07 — Advanced
| Example | What it demonstrates |
|---------|---------------------|
| `07_advanced/human_approval.go` | Human-in-the-loop tool approval |
| `07_advanced/cancel_run.go` | Cancel agent execution mid-flight |
| `07_advanced/cli_agent.go` | Interactive terminal chat with commands |
| `07_advanced/builtin_tools.go` | Calculator, JSON tools from tools package |

### 08 — Providers
| Example | What it demonstrates |
|---------|---------------------|
| `08_providers/multi_provider.go` | Same agent on OpenAI, Claude, Gemini, Ollama |

## Running

```bash
# From the repo root:
go run ./cookbook/01_basics/simple_agent.go
go run ./cookbook/01_basics/agent_with_tools.go
go run ./cookbook/04_workflows/parallel.go
go run ./cookbook/07_advanced/builtin_tools.go

# With verbose debug:
AGNOGO_DEBUG=true AGNOGO_DEBUG_LEVEL=2 go run ./cookbook/01_basics/simple_agent.go

# Different providers:
ANTHROPIC_API_KEY=sk-... go run ./cookbook/08_providers/multi_provider.go anthropic
```

## Debug Output

All examples include `Debug: &debug` which shows:
- Run start/end banners with run ID and session ID
- Model calls with latency
- Tool calls with status and timing
- Token usage (input/output/total)
- Total run duration
- Knowledge searches, memory extraction, guardrail blocks

Set `VerboseDebug()` for level 2 (full message dumps, tool args, results).
