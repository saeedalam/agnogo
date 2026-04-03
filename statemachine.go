package agnogo

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// AgentState represents a discrete state in the agent lifecycle.
type AgentState string

const (
	StateIdle            AgentState = "idle"
	StateProcessing      AgentState = "processing"
	StateCallingModel    AgentState = "calling_model"
	StateCallingTool     AgentState = "calling_tool"
	StateWaitingApproval AgentState = "waiting_approval"
	StateError           AgentState = "error"
	StateBudgetExceeded  AgentState = "budget_exceeded"
	StateComplete        AgentState = "complete"
)

// validTransitions defines the allowed state transitions.
var validTransitions = map[AgentState][]AgentState{
	StateIdle:            {StateProcessing},
	StateProcessing:      {StateCallingModel, StateError, StateBudgetExceeded, StateComplete},
	StateCallingModel:    {StateProcessing, StateCallingTool, StateError, StateBudgetExceeded},
	StateCallingTool:     {StateProcessing, StateWaitingApproval, StateError},
	StateWaitingApproval: {StateCallingTool, StateError, StateComplete},
	StateError:           {StateIdle, StateComplete},
	StateBudgetExceeded:  {StateComplete},
}

// StateTransition records a single state change.
type StateTransition struct {
	From      AgentState `json:"from"`
	To        AgentState `json:"to"`
	Timestamp time.Time  `json:"timestamp"`
	Reason    string     `json:"reason"`
}

// StateMachine tracks and enforces agent state transitions.
type StateMachine struct {
	mu           sync.Mutex
	current      AgentState
	history      []StateTransition
	onTransition func(from, to AgentState, reason string)
}

// NewStateMachine creates a state machine starting in StateIdle.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		current: StateIdle,
		history: []StateTransition{},
	}
}

// Current returns the current state.
func (sm *StateMachine) Current() AgentState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.current
}

// Transition moves to a new state if the transition is valid.
// Returns an error if the transition is not allowed.
func (sm *StateMachine) Transition(to AgentState, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	allowed, ok := validTransitions[sm.current]
	if !ok {
		return fmt.Errorf("agnogo: no transitions defined from state %q", sm.current)
	}

	valid := false
	for _, s := range allowed {
		if s == to {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("agnogo: invalid transition %q -> %q", sm.current, to)
	}

	from := sm.current
	sm.history = append(sm.history, StateTransition{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Reason:    reason,
	})
	sm.current = to

	if sm.onTransition != nil {
		sm.onTransition(from, to, reason)
	}

	return nil
}

// History returns a copy of all recorded transitions.
func (sm *StateMachine) History() []StateTransition {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	cp := make([]StateTransition, len(sm.history))
	copy(cp, sm.history)
	return cp
}

// ── Checkpoint + Resume ─────────────────────────────────────

// Checkpoint captures agent state for crash recovery.
type Checkpoint struct {
	SessionID string     `json:"session_id"`
	State     AgentState `json:"state"`
	Messages  []Message  `json:"messages"`
	Cost      float64    `json:"cost"`
	Step      int        `json:"step"`
	Timestamp time.Time  `json:"timestamp"`
}

// SaveCheckpoint captures current agent state for crash recovery.
// Stores in session.State["_checkpoint"] as a JSON string.
func SaveCheckpoint(session *Session, state AgentState, messages []Message, cost float64, step int) *Checkpoint {
	cp := &Checkpoint{
		SessionID: session.ID,
		State:     state,
		Messages:  messages,
		Cost:      cost,
		Step:      step,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(cp)
	if err != nil {
		slog.Error("agnogo: failed to marshal checkpoint", "error", err, "session", session.ID)
		return cp
	}
	session.Set("_checkpoint", string(data))
	return cp
}

// LoadCheckpoint retrieves a stored checkpoint from the session.
// Returns nil if no checkpoint exists or it cannot be decoded.
func LoadCheckpoint(session *Session) *Checkpoint {
	raw := session.GetStr("_checkpoint")
	if raw == "" {
		return nil
	}
	var cp Checkpoint
	if err := json.Unmarshal([]byte(raw), &cp); err != nil {
		return nil
	}
	return &cp
}
