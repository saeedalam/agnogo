# Agnogo Next Steps — Strategic Plan

## Current State (v0.6.0)

agnogo has strong fundamentals: concurrent tool execution, reliability layer (cost/PII/hallucination/confidence), graph orchestration with function nodes, MCP, OpenTelemetry, eval framework. 243+ tests, zero dependencies, 10 providers.

## Gap Analysis vs Agno Python

After deep analysis of [github.com/agno-agi/agno](https://github.com/agno-agi/agno), these are the significant gaps ordered by impact:

| Gap | Agno | agnogo | Impact |
|-----|------|--------|--------|
| Workflow engine | Steps, Loop, Parallel, Condition, Router, CEL, HITL, @pause | Basic Graph + Workflow | Critical |
| Multi-modal | Image, Audio, Video, File as first-class | None | High |
| Learning/Memory | 6 store types (user_profile, entity_memory, learned_knowledge, ...) | PatternMemory, LLMMemory | High |
| Reasoning | Provider-specific (Claude thinking, O1/O3, DeepSeek), multi-step with confidence | Basic chain-of-thought injection | High |
| Knowledge/RAG | 20+ readers, 7 chunking strategies, reranking, 20+ embedders | Basic Knowledge interface | Medium |
| Team modes | DELEGATE, DELEGATE_ALL, RESPOND_DIRECTLY, nested teams, shared state | Basic routing | Medium |
| Model providers | 48+ | 10 | Medium |
| Tools | 130+ | 35 | Medium |
| Session state | session_state dict, agentic state updates, state-in-context | Basic Session.State | Medium |
| Followup questions | Auto-generate follow-up suggestions | None | Low |
| Culture manager | Agent behavioral preferences | None | Low |
| Skills system | Structured reusable capabilities | None | Low |
| Remote agents | Cloud deployment | None | Low |
| Compression | Tool output summarization | None | Low |

---

## Phase 2B: Workflow Engine Overhaul (v0.7.0)

**Why first:** The workflow system is the core differentiator for enterprise adoption. Agno's workflow engine is their most sophisticated component (7,500+ lines). Our current Graph is ~280 lines.

### 2B.1 — Workflow Step Types

Add structured workflow steps matching Agno's model:

```go
// Step — atomic unit of work (agent, team, or custom function)
type Step struct {
    Name     string
    Agent    *Core          // mutually exclusive
    Team     *TeamConfig    // mutually exclusive
    Executor StepFunc       // mutually exclusive (custom function)
    MaxRetries    int
    SkipOnFailure bool
    RequiresConfirmation bool
    ConfirmationMessage  string
}

type StepFunc func(ctx context.Context, input *StepInput) (*StepOutput, error)

type StepInput struct {
    Input              string
    PreviousContent    string
    PreviousOutputs    map[string]*StepOutput
    SessionState       map[string]any
    AdditionalData     map[string]any
}

type StepOutput struct {
    StepName string
    Content  string
    Success  bool
    Error    string
    Metrics  *RunMetrics
    Stop     bool  // halt workflow
}
```

### 2B.2 — Control Flow Constructs

```go
// Sequential pipeline
type Steps struct {
    Steps []WorkflowStep  // interface implemented by Step, Loop, Parallel, Condition, Router
}

// Loop with break condition
type Loop struct {
    Steps         []WorkflowStep
    MaxIterations int
    EndCondition  func(iteration int, lastOutput *StepOutput, allSuccess bool) bool
}

// Parallel scatter-gather
type Parallel struct {
    Steps []WorkflowStep  // all execute concurrently
}

// Conditional branching
type Condition struct {
    Evaluator func(input *StepInput) bool
    Steps     []WorkflowStep  // if true
    ElseSteps []WorkflowStep  // if false
}

// Dynamic routing
type Router struct {
    Choices  map[string]WorkflowStep
    Selector func(input *StepInput) string  // returns choice name
}
```

### 2B.3 — Human-in-the-Loop (HITL)

Agno's HITL system is sophisticated. Port the core:
- `RequiresConfirmation` on Steps — pause, save state, return token
- `RequiresUserInput` — request structured input from user
- `OnReject` behavior — skip, cancel, or else-branch
- `OnError` behavior — fail, skip, or pause for human
- `ContinueRun(runID, approval)` — resume from pause point
- State persistence via Storage interface

### 2B.4 — Workflow Session & State

```go
type WorkflowSession struct {
    SessionID    string
    WorkflowID   string
    Runs         []WorkflowRunOutput
    SessionState map[string]any  // mutable state shared across steps
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

- Steps can read/write `SessionState` for cross-step communication
- State merging for Parallel steps (concurrent writes → merge)
- Workflow history: include N previous runs in context

**Files:** New `workflow_v2.go`, `workflow_step.go`, `workflow_types.go`, tests.

---

## Phase 3: Multi-Modal Support (v0.8.0)

**Why:** Every major LLM now supports vision. Agents that can't process images are limited.

### 3.1 — Media Types

```go
type Image struct {
    URL     string // remote URL
    Path    string // local file path
    Content []byte // raw bytes
    Detail  string // "low", "high", "auto"
}

type Audio struct {
    URL      string
    Path     string
    Content  []byte
    MimeType string
}

type File struct {
    URL      string
    Path     string
    Content  []byte
    Name     string
    MimeType string
}
```

### 3.2 — Message Extension

Extend `Message` struct to carry media:

```go
type Message struct {
    Role      string
    Content   string
    Images    []Image
    Audio     []Audio
    Files     []File
    ToolCalls []ToolCall
}
```

### 3.3 — Provider Adaptation

Each provider formats multi-modal differently:
- **OpenAI**: `content` array with `image_url` objects
- **Anthropic**: `content` array with `image` blocks (base64)
- **Gemini**: `parts` array with `inline_data`

Add `FormatMessages(messages []Message) []map[string]any` to each provider, letting the provider handle multi-modal encoding.

**Files:** New `media.go`, modify `session.go`, `provider.go`, each provider.

---

## Phase 4: Advanced Reasoning (v0.9.0)

**Why:** Reasoning is the quality differentiator. O1/O3, Claude extended thinking, and DeepSeek R1 produce dramatically better outputs.

### 4.1 — Provider-Specific Reasoning

Agno has per-provider reasoning handlers. Our `ReasoningConfig` is too generic.

```go
type ReasoningConfig struct {
    Enabled   bool
    Model     ModelProvider  // dedicated reasoning model (can differ from main)
    MinSteps  int            // minimum reasoning steps (default 1)
    MaxSteps  int            // maximum reasoning steps (default 10)
    Mode      ReasoningMode  // provider-specific mode
}

type ReasoningMode string
const (
    ReasoningDefault  ReasoningMode = "default"   // generic chain-of-thought
    ReasoningExtended ReasoningMode = "extended"   // Claude extended thinking
    ReasoningNative   ReasoningMode = "native"     // O1/O3, DeepSeek R1
)
```

### 4.2 — Reasoning Steps

```go
type ReasoningStep struct {
    Title      string
    Action     string
    Result     string
    Reasoning  string
    Confidence float64
    NextAction string  // "continue", "validate", "final_answer"
}
```

### 4.3 — Reasoning-Aware Providers

Add `SupportsReasoning() bool` and `ReasoningMode() ReasoningMode` to provider interface. Providers that support native reasoning (O1, Claude thinking) use it directly; others fall back to prompt-based chain-of-thought.

**Files:** Modify `reasoning.go`, each provider.

---

## Phase 5: Learning Machine (v1.0.0)

**Why:** Self-improving agents are the endgame. Agno's LearningMachine with 6 store types is their most forward-looking feature.

### 5.1 — Learning Stores

```go
type LearningStore interface {
    Store(ctx context.Context, key string, value any) error
    Recall(ctx context.Context, key string) (any, error)
    Search(ctx context.Context, query string, limit int) ([]LearningEntry, error)
    Delete(ctx context.Context, key string) error
}

type LearningEntry struct {
    Key       string
    Value     any
    Category  string  // "user_profile", "entity", "knowledge", "decision"
    CreatedAt time.Time
}
```

### 5.2 — Learning Machine

```go
type LearningMachine struct {
    UserProfile     LearningStore  // structured user attributes
    UserMemory      LearningStore  // unstructured observations
    EntityMemory    LearningStore  // knowledge about external entities
    LearnedKnowledge LearningStore // reusable insights
    SessionContext  LearningStore  // per-session context
    Model           ModelProvider  // for extraction
}
```

### 5.3 — Integration

- After each `Run()`, the learning machine extracts insights
- Learned knowledge is injected into future prompts
- Entity memories build a knowledge graph over time
- User profiles personalize responses

**Files:** New `learn.go`, `learn_store.go`, tests.

---

## Phase 6: Knowledge & RAG Improvements (v1.1.0)

### 6.1 — Document Readers

```go
type Reader interface {
    Read(ctx context.Context, source string) ([]Document, error)
}

// Built-in readers:
// - TextReader, JSONReader, CSVReader
// - PDFReader (Go stdlib — no CGo)
// - MarkdownReader
// - WebReader (fetch + extract text)
```

### 6.2 — Chunking Strategies

```go
type Chunker interface {
    Chunk(doc Document) []Chunk
}

// Built-in: FixedChunker, RecursiveChunker, MarkdownChunker
```

### 6.3 — Embedder Interface

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float64, error)
    Dimensions() int
}
```

Provider implementations: OpenAI, Anthropic, Gemini, Ollama.

**Files:** New `knowledge/reader.go`, `knowledge/chunker.go`, `knowledge/embedder.go`.

---

## Phase 7: Team Improvements (v1.2.0)

### 7.1 — Team Execution Modes

```go
type TeamMode string
const (
    TeamDelegate    TeamMode = "delegate"     // leader picks member
    TeamDelegateAll TeamMode = "delegate_all" // all members in parallel
    TeamRespond     TeamMode = "respond"      // members respond directly
)
```

### 7.2 — Hierarchical Teams

Teams can contain sub-teams for complex organizational patterns.

### 7.3 — Shared State

Team-level session state that members can read/write, enabling coordination without explicit message passing.

---

## Implementation Priority

```
v0.7.0  Workflow Engine Overhaul     ← biggest gap, enterprise-critical
v0.8.0  Multi-Modal Support          ← every LLM supports vision now
v0.9.0  Advanced Reasoning           ← quality differentiator
v1.0.0  Learning Machine             ← self-improving agents
v1.1.0  Knowledge/RAG Improvements   ← production RAG needs
v1.2.0  Team Improvements            ← multi-agent coordination
```

## Principles

1. **Zero dependencies** — everything uses Go stdlib
2. **Interface-first** — every component is pluggable
3. **Backward-compatible** — existing code keeps working
4. **Test-driven** — every feature ships with tests + race detector
5. **Production-grade** — no toys, every feature is battle-ready
