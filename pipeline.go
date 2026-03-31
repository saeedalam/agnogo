package agnogo

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ── Pipeline (sequential chaining) ─────────────────────────
// Pipeline chains agents sequentially: each agent's output text becomes the
// next agent's input. All tool calls are collected across the chain.
//
//	resp, _ := agent1.Then(agent2).Then(agent3).Run(ctx, session, "Hello")
type Pipeline struct {
	agents []*Core
}

// Then starts a two-agent pipeline. The receiver runs first, then next.
func (a *Core) Then(next *Core) *Pipeline {
	return &Pipeline{agents: []*Core{a, next}}
}

// Then appends an agent to the pipeline and returns the same pipeline
// for continued chaining.
func (p *Pipeline) Then(next *Core) *Pipeline {
	p.agents = append(p.agents, next)
	return p
}

// Run executes every agent in order. The first agent receives input; each
// subsequent agent receives the previous agent's Response.Text. Tool calls
// from all agents are merged into the final Response.
func (p *Pipeline) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	current := input
	var allTools []string

	for i, agent := range p.agents {
		resp, err := agent.Run(ctx, session, current)
		if err != nil {
			return nil, fmt.Errorf("pipeline stage %d: %w", i, err)
		}
		allTools = append(allTools, resp.ToolsCalled...)
		current = resp.Text

		// Pause the pipeline when human approval is required.
		if resp.NeedsApproval {
			resp.ToolsCalled = allTools
			return resp, nil
		}
	}

	return &Response{Text: current, ToolsCalled: allTools}, nil
}

// ── FanOut (parallel execution) ─────────────────────────────
// FanOut runs multiple agents concurrently on the same input and merges
// their outputs.
//
//	resp, _ := agnogo.All(a1, a2, a3).WithMerge(myMerge).Run(ctx, session, "input")
type FanOut struct {
	agents []*Core
	merge  func([]string) string
}

// All creates a FanOut that runs every agent in parallel on the same input.
func All(agents ...*Core) *FanOut {
	return &FanOut{
		agents: agents,
		merge: func(outputs []string) string {
			return strings.Join(outputs, "\n\n---\n\n")
		},
	}
}

// WithMerge sets a custom function to combine agent outputs. The slice
// order matches the order agents were passed to All.
func (f *FanOut) WithMerge(fn func([]string) string) *FanOut {
	f.merge = fn
	return f
}

// Run spawns one goroutine per agent. Each goroutine receives a cloned
// session (new ID, copied Memory and Metadata). Results are collected
// in agent order. If any agent errors, it is returned immediately.
func (f *FanOut) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	type result struct {
		index int
		resp  *Response
		err   error
	}

	ch := make(chan result, len(f.agents))

	for i, agent := range f.agents {
		go func(idx int, a *Core) {
			cloned := cloneSession(session, fmt.Sprintf("%s_fanout_%d", session.ID, idx))
			resp, err := a.Run(ctx, cloned, input)
			ch <- result{index: idx, resp: resp, err: err}
		}(i, agent)
	}

	outputs := make([]string, len(f.agents))
	var allTools []string

	for range f.agents {
		r := <-ch
		if r.err != nil {
			return nil, fmt.Errorf("fanout agent %d: %w", r.index, r.err)
		}
		outputs[r.index] = r.resp.Text
		allTools = append(allTools, r.resp.ToolsCalled...)
	}

	merged := f.merge(outputs)
	return &Response{Text: merged, ToolsCalled: allTools}, nil
}

// ── RaceGroup (first wins) ──────────────────────────────────
// RaceGroup runs multiple agents concurrently and returns the first
// non-error result, cancelling the remaining agents.
//
//	resp, _ := agnogo.Race(fast, slow, fallback).Run(ctx, session, "query")
type RaceGroup struct {
	agents []*Core
}

// Race creates a RaceGroup. The first agent to return a non-error result wins.
func Race(agents ...*Core) *RaceGroup {
	return &RaceGroup{agents: agents}
}

// Run spawns one goroutine per agent with a derived cancellable context.
// The first successful result wins and cancels all other agents.
// If every agent fails, the last error is returned.
func (r *RaceGroup) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		resp *Response
		err  error
	}

	ch := make(chan result, len(r.agents))

	for i, agent := range r.agents {
		go func(idx int, a *Core) {
			cloned := cloneSession(session, fmt.Sprintf("%s_race_%d", session.ID, idx))
			resp, err := a.Run(ctx, cloned, input)
			ch <- result{resp: resp, err: err}
		}(i, agent)
	}

	var lastErr error
	for range r.agents {
		res := <-ch
		if res.err != nil {
			lastErr = res.err
			continue
		}
		cancel() // signal remaining goroutines to stop
		return res.resp, nil
	}

	return nil, fmt.Errorf("all race agents failed, last error: %w", lastErr)
}

// ── Map (parallel map over inputs) ──────────────────────────
// MapResult holds the outcome of running a single input through an agent.
type MapResult struct {
	Input    string
	Response *Response
	Err      error
}

// Map runs agent against every input string concurrently, bounded by
// concurrency. Each input gets its own session. Results are returned in
// the same order as inputs.
func Map(ctx context.Context, agent *Core, inputs []string, concurrency int) []MapResult {
	if concurrency <= 0 {
		concurrency = 1
	}

	results := make([]MapResult, len(inputs))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, inp := range inputs {
		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			sess := NewSession(generateRunID())
			resp, err := agent.Run(ctx, sess, text)
			results[idx] = MapResult{Input: text, Response: resp, Err: err}
		}(i, inp)
	}

	wg.Wait()
	return results
}

// cloneSession creates a new Session with the given id and copies Memory
// and Metadata from the source session. History is not copied so that
// parallel agents start with a clean conversation.
func cloneSession(src *Session, id string) *Session {
	s := NewSession(id)
	src.mu.Lock()
	defer src.mu.Unlock()
	for k, v := range src.Memory {
		s.Memory[k] = v
	}
	for k, v := range src.Metadata {
		s.Metadata[k] = v
	}
	return s
}
