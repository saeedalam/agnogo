# AI Research Analyst

A full-featured research pipeline that demonstrates every major agnogo capability in a single, realistic project.

## What It Does

Takes any research question, plans a strategy, gathers data from multiple sources in parallel, synthesizes findings, refines iteratively, pauses for human review, and produces a formatted report — while learning user preferences across sessions.

## Architecture

```
                    ┌─────────────┐
                    │   Question   │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │    Plan     │  ← Advanced Reasoning (CoT / Native)
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼────┐ ┌────▼─────┐ ┌────▼──────┐
        │   Web    │ │   News   │ │ Technical │  ← Parallel Research
        │ Research │ │ Analysis │ │  Review   │
        └─────┬────┘ └────┬─────┘ └────┬──────┘
              │            │            │
              └────────────┼────────────┘
                           │
                    ┌──────▼──────┐
                    │ Synthesize  │  ← Pure Go (WfFunc, no LLM)
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Refine    │  ← Loop (iterative improvement)
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Depth Check │  ← Condition (expand if too short)
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Review    │  ← HITL (human approval)
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Format    │  ← Router (summary or detailed)
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Report    │
                    └─────────────┘
```

## Features Used

| Feature | Where | Why |
|---------|-------|-----|
| `WfSequence` | Main pipeline | Sequential orchestration |
| `WfParallel` | Research gathering | 3 sources searched concurrently |
| `WfFunc` | Synthesize step | Merge results without LLM call |
| `WfLoop` | Refinement | Iterative quality improvement |
| `WfCondition` | Depth check | Conditional deep dive |
| `WfRoute` | Output format | Route based on user preference |
| `WithConfirmation` | Review step | Human approves before publishing |
| `ReasoningConfig` | Planning agent | Multi-step research strategy |
| `Reliable()` | Research agents | Hallucination guard + confidence |
| `LearningMachine` | Across sessions | Remembers preferences + entities |
| `UserProfileStore` | User prefs | Name, role, format preference |
| `EntityMemoryStore` | Topic knowledge | Facts about researched subjects |
| `SessionContextStore` | Session summary | What was researched + decided |
| `ImageFromFile` | Optional | Analyze charts/screenshots |
| `AsyncPostProcess` | Post-run | Save learnings in background |
| `CostBudget` | Safety | $2/run, $10/session limit |
| `NewEval` | Testing | Automated quality assertions |

## Running

```bash
# Set your API key
export OPENAI_API_KEY=sk-...

# Run the research pipeline
go run main.go "Research the competitive landscape of AI agent frameworks"

# Run with a custom question
go run main.go "Compare Kubernetes vs Docker Swarm for production deployments"

# Run quality tests (requires API key)
go test -v -timeout 120s
```

## Files

- `main.go` — Full WorkflowEngine pipeline (recommended)
- `graph_version.go` — Same pipeline using Graph API (simpler alternative)
- `eval_test.go` — Automated quality tests using eval framework

## Customization

**Change the output format:**
```go
session.SetMemory("report_format", "summary") // or "detailed"
```

**Add your own research sources:**
```go
agnogo.WfParallel("gather",
    agnogo.WfStep("web", webAgent),
    agnogo.WfStep("arxiv", arxivAgent),     // add academic papers
    agnogo.WfStep("github", githubAgent),   // add code analysis
)
```

**Adjust refinement depth:**
```go
agnogo.WfLoop("refine", refineStep, func(out *agnogo.StepOutput, i int) bool {
    return i >= 4 // refine up to 5 times
}).WithMaxIterations(5)
```

**Skip human review:**
Remove `.WithConfirmation()` from the review step.
