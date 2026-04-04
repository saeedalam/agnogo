# Self-Improvement Loop

An AI agent system that reads research papers, analyzes agnogo's codebase, and generates code improvements.

## How it works

```
Researcher → Analyzer → Coder
```

1. **Researcher** searches ArXiv for papers on AI agents, reliability, hallucination detection
2. **Analyzer** reads agnogo source code, compares with paper techniques, identifies gaps
3. **Coder** writes improvements, runs tests, outputs PR-ready diffs

## Usage

```bash
cd cookbook/12_self_improve
OPENAI_API_KEY=sk-... go run main.go

# Search specific topics
OPENAI_API_KEY=sk-... go run main.go -topic "hallucination detection"
OPENAI_API_KEY=sk-... go run main.go -topic "agent cost optimization"
OPENAI_API_KEY=sk-... go run main.go -topic "tool use reliability"

# Run only researcher (save papers)
OPENAI_API_KEY=sk-... go run main.go -mode research

# Run only analyzer (analyze existing papers)
OPENAI_API_KEY=sk-... go run main.go -mode analyze

# Run full pipeline
OPENAI_API_KEY=sk-... go run main.go -mode full
```

## Output

- `papers/` — downloaded paper summaries (JSON)
- `knowledge/` — extracted techniques and patterns
- `output/` — generated code improvements with diffs
