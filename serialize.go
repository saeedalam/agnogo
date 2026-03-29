package agnogo

import "encoding/json"

// AgentConfig is a serializable representation of an agent's configuration.
// Matches Agno's to_dict() / from_dict() pattern.
type AgentConfig struct {
	Name         string            `json:"name,omitempty"`
	Instructions string            `json:"instructions,omitempty"`
	Tools        []string          `json:"tools,omitempty"`       // tool names
	MaxLoops     int               `json:"max_loops,omitempty"`
	FallbackText string            `json:"fallback_text,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ToDict serializes the agent configuration to a map.
// Does NOT include the model provider (not serializable) or tool functions.
// Matches Agno: agent.to_dict()
func (a *Agent) ToDict() map[string]any {
	toolNames := make([]string, 0, a.tools.Count())
	for _, t := range a.tools.List() {
		toolNames = append(toolNames, t.Name)
	}
	return map[string]any{
		"instructions":  a.instructions,
		"tools":         toolNames,
		"max_loops":     a.maxLoops,
		"fallback_text": a.fallbackText,
	}
}

// ToJSON serializes the agent configuration to JSON bytes.
func (a *Agent) ToJSON() ([]byte, error) {
	return json.Marshal(a.ToDict())
}

// String returns a human-readable description of the agent.
func (a *Agent) String() string {
	tools := a.tools.Names()
	if tools == "" {
		tools = "(none)"
	}
	return "Agent{tools: [" + tools + "], max_loops: " + itoa(a.maxLoops) + "}"
}

func itoa(n int) string {
	if n == 0 { return "0" }
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
