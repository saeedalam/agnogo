package agnogo

import (
	"context"
	"fmt"
	"sync"
)

// GraphState holds shared state that flows through a graph execution.
// It is thread-safe via sync.RWMutex.
//
// Nodes store their responses in state so downstream nodes can read them.
// The keys "last_response" and "<node_name>_response" are set automatically
// after each node runs.
type GraphState struct {
	data map[string]any
	mu   sync.RWMutex
}

// NewGraphState creates a new empty GraphState.
func NewGraphState() *GraphState {
	return &GraphState{
		data: make(map[string]any),
	}
}

// Set stores a value in the state.
func (s *GraphState) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get retrieves a value from the state.
func (s *GraphState) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// GetStr retrieves a value as a string. Returns "" if not found or not a string.
func (s *GraphState) GetStr(key string) string {
	v := s.Get(key)
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	return ""
}

// GetInt retrieves a value as an int. Returns 0 if not found or not numeric.
func (s *GraphState) GetInt(key string) int {
	v := s.Get(key)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// GetBool retrieves a value as a bool. Returns false if not found or not a bool.
func (s *GraphState) GetBool(key string) bool {
	v := s.Get(key)
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GraphFunc is a pure Go function that can serve as a graph node.
// It receives the graph state and can read/modify it directly.
// Set state["last_response"] to control the input passed to the next node.
type GraphFunc func(ctx context.Context, state *GraphState) error

type graphNode struct {
	name  string
	agent *Core     // non-nil for agent nodes
	fn    GraphFunc // non-nil for function nodes
}

type graphEdge struct {
	from      string
	to        string
	condition func(ctx context.Context, state *GraphState) bool // nil = default/unconditional
}

// Graph is a directed workflow graph where agents are nodes and edges define
// conditional flow. Unlike Sequential/Parallel, graphs support cycles, branching,
// and state-based routing.
//
//	g := agnogo.NewGraph()
//	g.AddNode("classify", classifyAgent)
//	g.AddNode("refund", refundAgent)
//	g.AddNode("support", supportAgent)
//	g.AddNode("escalate", escalateAgent)
//	g.SetEntry("classify")
//	g.AddEdge("classify", "refund", func(ctx context.Context, state *GraphState) bool {
//	    return state.GetStr("intent") == "refund"
//	})
//	g.AddEdge("classify", "support", nil) // default edge (no condition)
//	g.AddEdge("support", "escalate", func(ctx context.Context, state *GraphState) bool {
//	    return state.GetInt("attempts") > 2
//	})
//	g.AddEdge("escalate", "support", nil) // cycle back
//	g.SetEnd("refund", "support") // terminal nodes
//
//	resp, _ := g.Run(ctx, session, "I want a refund")
type Graph struct {
	nodes    map[string]*graphNode
	edges    []graphEdge
	entry    string
	endNodes map[string]bool
	maxSteps int // prevent infinite loops (default 20)
}

// NewGraph creates an empty graph with a default maxSteps of 20.
func NewGraph() *Graph {
	return &Graph{
		nodes:    make(map[string]*graphNode),
		edges:    nil,
		endNodes: make(map[string]bool),
		maxSteps: 20,
	}
}

// AddNode registers an agent as a named node in the graph. Chainable.
func (g *Graph) AddNode(name string, agent *Core) *Graph {
	g.nodes[name] = &graphNode{name: name, agent: agent}
	return g
}

// AddFuncNode registers a pure Go function as a graph node. Function nodes
// execute without an LLM call — they read and modify GraphState directly.
// Set state["last_response"] to control the input to the next node. Chainable.
func (g *Graph) AddFuncNode(name string, fn GraphFunc) *Graph {
	if fn == nil {
		panic("agnogo: AddFuncNode fn must not be nil")
	}
	g.nodes[name] = &graphNode{name: name, fn: fn}
	return g
}

// AddEdge adds a directed edge from one node to another with an optional condition.
// If condition is nil, the edge is a default/unconditional edge that fires only
// when no conditional edge from the same source matches. Chainable.
func (g *Graph) AddEdge(from, to string, condition func(ctx context.Context, state *GraphState) bool) *Graph {
	g.edges = append(g.edges, graphEdge{from: from, to: to, condition: condition})
	return g
}

// SetEntry sets the starting node of the graph. Chainable.
func (g *Graph) SetEntry(name string) *Graph {
	g.entry = name
	return g
}

// SetEnd marks one or more nodes as terminal. When a terminal node finishes
// and no outgoing edge matches, the graph returns successfully. Chainable.
func (g *Graph) SetEnd(names ...string) *Graph {
	for _, n := range names {
		g.endNodes[n] = true
	}
	return g
}

// WithMaxSteps sets the maximum number of node executions to prevent infinite loops.
// Chainable.
func (g *Graph) WithMaxSteps(n int) *Graph {
	g.maxSteps = n
	return g
}

// Run executes the graph starting from the entry node.
// At each step:
//  1. Run the current node's agent with the current input
//  2. Store the response in GraphState as "last_response" and "<node_name>_response"
//  3. Evaluate outgoing edges in order -- first matching condition wins
//  4. If current node is an end node and no edge matches, return
//  5. If no edge matches and node is not an end node, return error
//  6. Pass the response text as input to the next node
//  7. Stop if maxSteps reached
func (g *Graph) Run(ctx context.Context, session *Session, input string) (*Response, error) {
	if g.entry == "" {
		return nil, fmt.Errorf("agnogo: graph has no entry node")
	}
	if _, ok := g.nodes[g.entry]; !ok {
		return nil, fmt.Errorf("agnogo: entry node %q not found", g.entry)
	}

	state := NewGraphState()
	currentNode := g.entry
	currentInput := input
	var allTools []string

	for step := 0; step < g.maxSteps; step++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		node, ok := g.nodes[currentNode]
		if !ok {
			return nil, fmt.Errorf("agnogo: node %q not found", currentNode)
		}

		var nodeText string

		if node.fn != nil {
			// Function node: execute directly, no LLM call.
			// Set last_response to the current input so fn can read it.
			// If fn doesn't modify last_response, the input passes through.
			state.Set("last_response", currentInput)
			if err := node.fn(ctx, state); err != nil {
				return nil, fmt.Errorf("agnogo: func node %q failed: %w", currentNode, err)
			}
			nodeText = state.GetStr("last_response")
			state.Set(currentNode+"_response", nodeText)
		} else {
			// Agent node: run the LLM agent
			resp, err := node.agent.Run(ctx, session, currentInput)
			if err != nil {
				return nil, fmt.Errorf("agnogo: node %q failed: %w", currentNode, err)
			}
			allTools = append(allTools, resp.ToolsCalled...)
			nodeText = resp.Text
			state.Set("last_response", nodeText)
			state.Set(currentNode+"_response", nodeText)
		}

		// Evaluate outgoing edges
		next := g.resolveNext(ctx, currentNode, state)

		if next == "" {
			// No edge matched
			if g.endNodes[currentNode] {
				return &Response{Text: nodeText, ToolsCalled: allTools}, nil
			}
			return nil, fmt.Errorf("agnogo: node %q is not an end node and no edge matched", currentNode)
		}

		currentInput = nodeText
		currentNode = next
	}

	return nil, fmt.Errorf("agnogo: graph exceeded maxSteps (%d)", g.maxSteps)
}

// resolveNext evaluates outgoing edges from a node and returns the next node name.
// Conditional edges are evaluated first; default edges (condition == nil) fire only
// if no conditional edge matched. Returns "" if no edge matches.
func (g *Graph) resolveNext(ctx context.Context, from string, state *GraphState) string {
	var defaultTarget string

	for _, edge := range g.edges {
		if edge.from != from {
			continue
		}
		if edge.condition == nil {
			// Remember the first default edge but keep looking for conditional matches
			if defaultTarget == "" {
				defaultTarget = edge.to
			}
			continue
		}
		if edge.condition(ctx, state) {
			return edge.to
		}
	}

	return defaultTarget
}
