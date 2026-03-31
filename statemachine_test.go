package agnogo

import (
	"testing"
)

func TestStateMachineTransitions(t *testing.T) {
	sm := NewStateMachine()

	if sm.Current() != StateIdle {
		t.Fatalf("expected idle, got %q", sm.Current())
	}

	steps := []struct {
		to     AgentState
		reason string
	}{
		{StateProcessing, "user message received"},
		{StateCallingModel, "sending to LLM"},
		{StateCallingTool, "model requested tool"},
		{StateProcessing, "tool completed"},
		{StateCallingModel, "continuing with tool result"},
		{StateProcessing, "got final response"},
		{StateComplete, "response delivered"},
	}

	for _, s := range steps {
		if err := sm.Transition(s.to, s.reason); err != nil {
			t.Fatalf("valid transition to %q failed: %v", s.to, err)
		}
	}

	if sm.Current() != StateComplete {
		t.Fatalf("expected complete, got %q", sm.Current())
	}
}

func TestStateMachineInvalidTransition(t *testing.T) {
	sm := NewStateMachine()

	// idle -> calling_tool is not valid
	err := sm.Transition(StateCallingTool, "bad transition")
	if err == nil {
		t.Fatal("expected error for invalid transition idle -> calling_tool")
	}

	// idle -> calling_model is not valid
	err = sm.Transition(StateCallingModel, "bad transition")
	if err == nil {
		t.Fatal("expected error for invalid transition idle -> calling_model")
	}

	// State should still be idle
	if sm.Current() != StateIdle {
		t.Fatalf("expected idle after failed transitions, got %q", sm.Current())
	}
}

func TestStateMachineHistory(t *testing.T) {
	sm := NewStateMachine()

	_ = sm.Transition(StateProcessing, "start")
	_ = sm.Transition(StateCallingModel, "calling model")
	_ = sm.Transition(StateProcessing, "got response")
	_ = sm.Transition(StateComplete, "done")

	history := sm.History()
	if len(history) != 4 {
		t.Fatalf("expected 4 transitions, got %d", len(history))
	}

	if history[0].From != StateIdle || history[0].To != StateProcessing {
		t.Errorf("first transition: expected idle->processing, got %s->%s", history[0].From, history[0].To)
	}
	if history[0].Reason != "start" {
		t.Errorf("first reason: expected 'start', got %q", history[0].Reason)
	}
	if history[3].To != StateComplete {
		t.Errorf("last transition: expected complete, got %s", history[3].To)
	}

	// Verify timestamps are non-zero
	for i, h := range history {
		if h.Timestamp.IsZero() {
			t.Errorf("transition %d has zero timestamp", i)
		}
	}
}

func TestStateMachineWaitingApproval(t *testing.T) {
	sm := NewStateMachine()

	_ = sm.Transition(StateProcessing, "start")
	_ = sm.Transition(StateCallingModel, "model call")
	_ = sm.Transition(StateCallingTool, "tool requested")
	_ = sm.Transition(StateWaitingApproval, "needs human approval")

	// Approved -> calling_tool
	if err := sm.Transition(StateCallingTool, "approved"); err != nil {
		t.Fatalf("approved transition failed: %v", err)
	}

	// Back to processing and complete
	_ = sm.Transition(StateProcessing, "tool done")
	_ = sm.Transition(StateComplete, "done")
}

func TestStateMachineErrorRecovery(t *testing.T) {
	sm := NewStateMachine()

	_ = sm.Transition(StateProcessing, "start")
	_ = sm.Transition(StateError, "something broke")

	// Error -> idle for retry
	if err := sm.Transition(StateIdle, "retry"); err != nil {
		t.Fatalf("error->idle failed: %v", err)
	}

	// Error -> complete for give up
	sm2 := NewStateMachine()
	_ = sm2.Transition(StateProcessing, "start")
	_ = sm2.Transition(StateError, "something broke")
	if err := sm2.Transition(StateComplete, "give up"); err != nil {
		t.Fatalf("error->complete failed: %v", err)
	}
}

func TestCheckpointSaveLoad(t *testing.T) {
	session := NewSession("test-session-cp")

	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	cp := SaveCheckpoint(session, StateCallingModel, messages, 0.0042, 3)

	if cp.SessionID != "test-session-cp" {
		t.Errorf("session ID: expected test-session-cp, got %q", cp.SessionID)
	}
	if cp.State != StateCallingModel {
		t.Errorf("state: expected calling_model, got %q", cp.State)
	}
	if cp.Step != 3 {
		t.Errorf("step: expected 3, got %d", cp.Step)
	}
	if cp.Cost != 0.0042 {
		t.Errorf("cost: expected 0.0042, got %f", cp.Cost)
	}

	// Load it back
	loaded := LoadCheckpoint(session)
	if loaded == nil {
		t.Fatal("LoadCheckpoint returned nil")
	}
	if loaded.SessionID != cp.SessionID {
		t.Errorf("loaded session ID mismatch: %q vs %q", loaded.SessionID, cp.SessionID)
	}
	if loaded.State != cp.State {
		t.Errorf("loaded state mismatch: %q vs %q", loaded.State, cp.State)
	}
	if len(loaded.Messages) != len(messages) {
		t.Errorf("loaded messages: expected %d, got %d", len(messages), len(loaded.Messages))
	}
	if loaded.Step != cp.Step {
		t.Errorf("loaded step mismatch: %d vs %d", loaded.Step, cp.Step)
	}

	// No checkpoint in fresh session
	fresh := NewSession("fresh")
	if LoadCheckpoint(fresh) != nil {
		t.Error("expected nil checkpoint from fresh session")
	}
}
