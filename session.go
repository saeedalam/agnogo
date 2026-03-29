package agnogo

import (
	"sync"
	"time"
)

// Session holds conversation state. Persisted between turns via Storage.
type Session struct {
	mu        sync.Mutex
	ID        string            `json:"id"`
	UserID    string            `json:"user_id,omitempty"`    // caller identifier (phone, email, etc.)
	Channel   string            `json:"channel,omitempty"`    // "webchat", "voice", "whatsapp", etc.
	History   []Message         `json:"history"`
	Memory    map[string]string `json:"memory,omitempty"`     // learned facts about the user
	State     map[string]any    `json:"state,omitempty"`      // flow state (step, attempts, etc.)
	Metadata  map[string]string `json:"metadata,omitempty"`   // custom key-values (business_id, tenant_id, etc.)
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Message is one turn in the conversation.
type Message struct {
	Role      string     `json:"role"`                  // "system", "user", "assistant", "tool"
	Content   string     `json:"content"`
	Name      string     `json:"name,omitempty"`        // tool_call_id for role=tool
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`  // for role=assistant
}

// ToolCall records a tool invocation by the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// NewSession creates a new conversation.
func NewSession(id string) *Session {
	return &Session{
		ID:       id,
		History:  []Message{},
		Memory:   map[string]string{},
		State:    map[string]any{},
		Metadata: map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddMessage appends a message to history.
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, Message{Role: role, Content: content})
	s.UpdatedAt = time.Now()
}

// AddToolResult appends a tool result message.
func (s *Session) AddToolResult(toolCallID, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, Message{Role: "tool", Content: result, Name: toolCallID})
	s.UpdatedAt = time.Now()
}

// SetMemory stores a learned fact.
func (s *Session) SetMemory(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Memory == nil {
		s.Memory = map[string]string{}
	}
	s.Memory[key] = value
}

// GetMemory retrieves a learned fact.
func (s *Session) GetMemory(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Memory[key]
}

// Set stores a state value.
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State == nil {
		s.State = map[string]any{}
	}
	s.State[key] = value
}

// Get retrieves a state value.
func (s *Session) Get(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State[key]
}

// GetStr retrieves a state value as string.
func (s *Session) GetStr(key string) string {
	v := s.Get(key)
	if v == nil {
		return ""
	}
	if str, ok := v.(string); ok {
		return str
	}
	return ""
}

// Increment increments a numeric state counter, returns new value.
func (s *Session) Increment(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	if v, ok := s.State[key]; ok {
		switch n := v.(type) {
		case float64:
			count = int(n)
		case int:
			count = n
		}
	}
	count++
	s.State[key] = count
	return count
}

// SetMeta stores a metadata value.
func (s *Session) SetMeta(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Metadata == nil {
		s.Metadata = map[string]string{}
	}
	s.Metadata[key] = value
}

// GetMeta retrieves a metadata value.
func (s *Session) GetMeta(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Metadata[key]
}
