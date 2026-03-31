package agnogo

import (
	"context"
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
