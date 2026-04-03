package agnogo

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestGraphLinear(t *testing.T) {
	// A -> B -> C, verify output is from C
	modelA := &mockModel{responses: []ModelResponse{{Text: "from-A"}}}
	modelB := &mockModel{responses: []ModelResponse{{Text: "from-B"}}}
	modelC := &mockModel{responses: []ModelResponse{{Text: "from-C"}}}

	agentA := New(Config{Model: modelA})
	agentB := New(Config{Model: modelB})
	agentC := New(Config{Model: modelC})

	g := NewGraph()
	g.AddNode("A", agentA)
	g.AddNode("B", agentB)
	g.AddNode("C", agentC)
	g.SetEntry("A")
	g.SetEnd("C")
	g.AddEdge("A", "B", nil)
	g.AddEdge("B", "C", nil)

	resp, err := g.Run(context.Background(), NewSession("linear"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "from-C" {
		t.Errorf("got %q, want %q", resp.Text, "from-C")
	}
}

func TestGraphBranching(t *testing.T) {
	// classify routes to refund OR support based on state
	classifyModel := &mockModel{responses: []ModelResponse{{Text: "classified"}}}
	refundModel := &mockModel{responses: []ModelResponse{{Text: "refund-done"}}}
	supportModel := &mockModel{responses: []ModelResponse{{Text: "support-done"}}}

	classifyAgent := New(Config{Model: classifyModel})
	refundAgent := New(Config{Model: refundModel})
	supportAgent := New(Config{Model: supportModel})

	g := NewGraph()
	g.AddNode("classify", classifyAgent)
	g.AddNode("refund", refundAgent)
	g.AddNode("support", supportAgent)
	g.SetEntry("classify")
	g.SetEnd("refund", "support")

	// Conditional edge: route to refund when intent is "refund"
	g.AddEdge("classify", "refund", func(ctx context.Context, state *GraphState) bool {
		return state.GetStr("last_response") == "classified"
	})
	// Default edge: route to support otherwise
	g.AddEdge("classify", "support", nil)

	resp, err := g.Run(context.Background(), NewSession("branch"), "I want a refund")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "refund-done" {
		t.Errorf("got %q, want %q", resp.Text, "refund-done")
	}

	// Now test the default branch: the conditional edge won't match
	classifyModel2 := &mockModel{responses: []ModelResponse{{Text: "other"}}}
	classifyAgent2 := New(Config{Model: classifyModel2})
	supportModel2 := &mockModel{responses: []ModelResponse{{Text: "support-handled"}}}
	supportAgent2 := New(Config{Model: supportModel2})

	g2 := NewGraph()
	g2.AddNode("classify", classifyAgent2)
	g2.AddNode("refund", refundAgent)
	g2.AddNode("support", supportAgent2)
	g2.SetEntry("classify")
	g2.SetEnd("refund", "support")
	g2.AddEdge("classify", "refund", func(ctx context.Context, state *GraphState) bool {
		return state.GetStr("last_response") == "classified"
	})
	g2.AddEdge("classify", "support", nil)

	resp2, err := g2.Run(context.Background(), NewSession("branch2"), "help me")
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Text != "support-handled" {
		t.Errorf("got %q, want %q", resp2.Text, "support-handled")
	}
}

func TestGraphCycleMaxSteps(t *testing.T) {
	// A -> B -> A cycle, should stop at maxSteps
	modelA := &mockModel{responses: []ModelResponse{
		{Text: "a1"}, {Text: "a2"}, {Text: "a3"},
	}}
	modelB := &mockModel{responses: []ModelResponse{
		{Text: "b1"}, {Text: "b2"}, {Text: "b3"},
	}}

	agentA := New(Config{Model: modelA})
	agentB := New(Config{Model: modelB})

	g := NewGraph()
	g.AddNode("A", agentA)
	g.AddNode("B", agentB)
	g.SetEntry("A")
	g.AddEdge("A", "B", nil)
	g.AddEdge("B", "A", nil)
	g.WithMaxSteps(4)

	_, err := g.Run(context.Background(), NewSession("cycle"), "go")
	if err == nil {
		t.Fatal("expected error for exceeding maxSteps")
	}
	if got := err.Error(); got != "agnogo: graph exceeded maxSteps (4)" {
		t.Errorf("error = %q", got)
	}
}

func TestGraphState(t *testing.T) {
	// Verify state is passed between nodes and accessible
	state := NewGraphState()
	state.Set("key", "value")
	state.Set("count", 42)

	if state.GetStr("key") != "value" {
		t.Errorf("GetStr = %q", state.GetStr("key"))
	}
	if state.GetInt("count") != 42 {
		t.Errorf("GetInt = %d", state.GetInt("count"))
	}
	if state.GetStr("missing") != "" {
		t.Error("expected empty string for missing key")
	}
	if state.GetInt("missing") != 0 {
		t.Error("expected 0 for missing int key")
	}
	if state.GetBool("missing") != false {
		t.Error("expected false for missing bool key")
	}

	state.Set("flag", true)
	if !state.GetBool("flag") {
		t.Error("expected true for bool flag")
	}
}

func TestGraphEndNode(t *testing.T) {
	// Graph stops at end node even if there are edges
	modelA := &mockModel{responses: []ModelResponse{{Text: "done"}}}
	agentA := New(Config{Model: modelA})

	g := NewGraph()
	g.AddNode("A", agentA)
	g.SetEntry("A")
	g.SetEnd("A")

	resp, err := g.Run(context.Background(), NewSession("end"), "go")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "done" {
		t.Errorf("got %q, want %q", resp.Text, "done")
	}
}

func TestGraphMissingEntry(t *testing.T) {
	g := NewGraph()
	_, err := g.Run(context.Background(), NewSession("noentry"), "go")
	if err == nil {
		t.Fatal("expected error for missing entry node")
	}
	if got := err.Error(); got != "agnogo: graph has no entry node" {
		t.Errorf("error = %q", got)
	}
}

// ── Phase 2: Function Nodes ────────────────────────────────

func TestGraphFuncNode(t *testing.T) {
	// func node sets state, downstream agent reads it
	modelB := &mockModel{responses: []ModelResponse{{Text: "final"}}}
	agentB := New(Config{Model: modelB})

	g := NewGraph()
	g.AddFuncNode("transform", func(ctx context.Context, state *GraphState) error {
		state.Set("last_response", "transformed-input")
		state.Set("custom_key", "custom_value")
		return nil
	})
	g.AddNode("finish", agentB)
	g.SetEntry("transform")
	g.SetEnd("finish")
	g.AddEdge("transform", "finish", nil)

	resp, err := g.Run(context.Background(), NewSession("func-node"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "final" {
		t.Errorf("got %q, want %q", resp.Text, "final")
	}
}

func TestGraphFuncNodeOnly(t *testing.T) {
	// Graph with only function nodes — no LLM calls at all
	g := NewGraph()
	g.AddFuncNode("step1", func(ctx context.Context, state *GraphState) error {
		state.Set("last_response", "step1-done")
		return nil
	})
	g.AddFuncNode("step2", func(ctx context.Context, state *GraphState) error {
		prev := state.GetStr("step1_response")
		state.Set("last_response", prev+"+step2-done")
		return nil
	})
	g.SetEntry("step1")
	g.SetEnd("step2")
	g.AddEdge("step1", "step2", nil)

	resp, err := g.Run(context.Background(), NewSession("func-only"), "input")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "step1-done+step2-done" {
		t.Errorf("got %q, want %q", resp.Text, "step1-done+step2-done")
	}
}

func TestGraphFuncNodeError(t *testing.T) {
	g := NewGraph()
	g.AddFuncNode("failing", func(ctx context.Context, state *GraphState) error {
		return fmt.Errorf("processing failed")
	})
	g.SetEntry("failing")
	g.SetEnd("failing")

	_, err := g.Run(context.Background(), NewSession("func-error"), "input")
	if err == nil {
		t.Fatal("expected error from failing func node")
	}
	if !strings.Contains(err.Error(), "func node") || !strings.Contains(err.Error(), "processing failed") {
		t.Errorf("error = %q, want to contain 'func node' and 'processing failed'", err.Error())
	}
}

func TestGraphMixedNodes(t *testing.T) {
	// agent -> func -> agent flow
	modelA := &mockModel{responses: []ModelResponse{{Text: "from-A"}}}
	modelC := &mockModel{responses: []ModelResponse{{Text: "from-C"}}}
	agentA := New(Config{Model: modelA})
	agentC := New(Config{Model: modelC})

	g := NewGraph()
	g.AddNode("A", agentA)
	g.AddFuncNode("B", func(ctx context.Context, state *GraphState) error {
		prev := state.GetStr("A_response")
		state.Set("last_response", "processed:"+prev)
		return nil
	})
	g.AddNode("C", agentC)
	g.SetEntry("A")
	g.SetEnd("C")
	g.AddEdge("A", "B", nil)
	g.AddEdge("B", "C", nil)

	resp, err := g.Run(context.Background(), NewSession("mixed"), "start")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "from-C" {
		t.Errorf("got %q, want %q", resp.Text, "from-C")
	}
}

func TestGraphFuncNodeConditionalRouting(t *testing.T) {
	// func node sets state that determines edge routing
	modelApprove := &mockModel{responses: []ModelResponse{{Text: "approved"}}}
	modelReject := &mockModel{responses: []ModelResponse{{Text: "rejected"}}}
	approveAgent := New(Config{Model: modelApprove})
	rejectAgent := New(Config{Model: modelReject})

	g := NewGraph()
	g.AddFuncNode("classify", func(ctx context.Context, state *GraphState) error {
		state.Set("last_response", "classified")
		state.Set("score", 85)
		return nil
	})
	g.AddNode("approve", approveAgent)
	g.AddNode("reject", rejectAgent)
	g.SetEntry("classify")
	g.SetEnd("approve", "reject")
	g.AddEdge("classify", "approve", func(ctx context.Context, state *GraphState) bool {
		return state.GetInt("score") >= 80
	})
	g.AddEdge("classify", "reject", nil) // default

	resp, err := g.Run(context.Background(), NewSession("func-routing"), "check")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "approved" {
		t.Errorf("got %q, want %q", resp.Text, "approved")
	}
}

func TestGraphFuncNodeReceivesInput(t *testing.T) {
	// Entry function node must be able to read the original input via last_response
	g := NewGraph()
	var receivedInput string
	g.AddFuncNode("entry", func(ctx context.Context, state *GraphState) error {
		receivedInput = state.GetStr("last_response")
		state.Set("last_response", "processed:"+receivedInput)
		return nil
	})
	g.SetEntry("entry")
	g.SetEnd("entry")

	resp, err := g.Run(context.Background(), NewSession("func-input"), "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if receivedInput != "hello world" {
		t.Errorf("func node received input %q, want %q", receivedInput, "hello world")
	}
	if resp.Text != "processed:hello world" {
		t.Errorf("got %q, want %q", resp.Text, "processed:hello world")
	}
}

func TestGraphFuncNodePassthrough(t *testing.T) {
	// If a function node doesn't set last_response, the input passes through
	modelB := &mockModel{responses: []ModelResponse{{Text: "final"}}}
	agentB := New(Config{Model: modelB})

	g := NewGraph()
	g.AddFuncNode("noop", func(ctx context.Context, state *GraphState) error {
		// Intentionally do NOT set last_response — should pass through
		state.Set("side_effect", "done")
		return nil
	})
	g.AddNode("finish", agentB)
	g.SetEntry("noop")
	g.SetEnd("finish")
	g.AddEdge("noop", "finish", nil)

	resp, err := g.Run(context.Background(), NewSession("passthrough"), "original input")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "final" {
		t.Errorf("got %q, want %q", resp.Text, "final")
	}
}

func TestGraphAddFuncNodeNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil fn")
		}
	}()
	g := NewGraph()
	g.AddFuncNode("bad", nil)
}
