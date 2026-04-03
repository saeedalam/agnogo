# AI Research Analyst

An agent that researches any question and writes a report. Read `main.go` top to bottom — it's a story in 6 chapters.

## The Story

**Chapter 1: The Agents** — Four specialists, each with a clear role. The planner thinks before acting (reasoning). The researcher hunts for facts (reliability). The editor polishes. The formatter delivers.

**Chapter 2: The Pipeline** — Seven acts, read like a screenplay:
```
Plan → Research (3 in parallel) → Merge (pure Go) → Polish (loop) → Quality gate → Human review → Deliver
```

**Chapter 3: Memory** — First conversation: knows nothing. Second conversation: remembers your name, what you researched, and how you like your reports.

**Chapter 4: Go** — One function call starts everything.

**Chapter 5: The Human Gate** — The pipeline pauses. You review. You approve or cancel.

**Chapter 6: The Report** — Done.

## Features Used (Naturally)

```
Chapter 1       → Agent(), Reliable(), WithReasoningConfig(), WithBudget()
Chapter 2, Act 1 → WfStep (agent step)
Chapter 2, Act 2 → WfParallel (concurrent research)
Chapter 2, Act 3 → WfFunc (pure Go, no LLM)
Chapter 2, Act 4 → WfLoop (iterative refinement)
Chapter 2, Act 5 → WfCondition (quality gate)
Chapter 2, Act 6 → WithConfirmation (HITL pause/resume)
Chapter 2, Act 7 → WfRoute (format selection from memory)
Chapter 3       → LearningMachine, UserProfile, SessionContext, EntityMemory
Chapter 4       → AddMediaMessage (optional images), AsyncPostProcess
Chapter 5       → ErrWorkflowPaused, ResumeWorkflow
Chapter 6       → Response.ReasoningSteps
```

## Run

```bash
export OPENAI_API_KEY=sk-...
go run main.go "What makes Go better than Python for AI agents?"
go run main.go "Compare Kubernetes vs Docker Swarm"
go run main.go "Research the history of neural networks"
```

## Files

| File | What | Read it to learn |
|------|------|-----------------|
| `main.go` | The full pipeline | How to build a production agent workflow |
| `graph_version.go` | Same thing, simpler API | When to use Graph vs WorkflowEngine |
| `eval_test.go` | Quality tests | How to test agent output automatically |
