// Package agnogo is a Go agent framework for building AI agents with tools,
// knowledge, memory, teams, workflows, guardrails, and more.
//
// Quick start (auto-detect provider from env vars):
//
//	agent := agnogo.Agent("You are a helpful assistant.")
//	answer, _ := agent.Ask(ctx, "What's the weather in Stockholm?")
//
// Or with an explicit provider:
//
//	agent := agnogo.Agent("You are helpful.", agnogo.WithOpenAI())
//	agent := agnogo.Agent("You are helpful.", agnogo.WithAnthropic("claude-sonnet-4-5-20250514"))
//
// Power-user mode (explicit configuration):
//
//	agent := agnogo.New(agnogo.Config{
//	    Model:        openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini"),
//	    Instructions: "You are a helpful assistant.",
//	})
//	resp, _ := agent.Run(ctx, session, "What's the weather in Stockholm?")
//
// Serve as HTTP API:
//
//	agent.Serve(":8080")
package agnogo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ToolFunc is any Go function that the agent can call.
type ToolFunc func(ctx context.Context, args map[string]string) (string, error)

// ToolDef is a tool definition for bulk registration via AddTools().
// Used by built-in tool packages: tools.Calculator(), tools.WebSearch(), etc.
type ToolDef struct {
	Name   string
	Desc   string
	Params Params
	Fn     ToolFunc
}

// Param describes one parameter for a tool.
type Param struct {
	Type     string   `json:"type"`               // "string", "number", "boolean"
	Desc     string   `json:"description"`
	Required bool     `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// Params is shorthand for a parameter map.
type Params map[string]Param

// Tool is a registered capability the agent can invoke.
type Tool struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Parameters      Params   `json:"parameters,omitempty"`
	Run             ToolFunc `json:"-"`
	RequireApproval bool     `json:"-"` // if true, agent pauses for human approval
	ApprovalReason  string   `json:"-"` // shown to human reviewer
}

// ToolRegistry manages dynamic tool registration.
// Safe for concurrent use.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
	order []string
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]*Tool)}
}

// Add registers a tool. Replaces existing tool with same name.
func (r *ToolRegistry) Add(name, desc string, params Params, fn ToolFunc) *Tool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	t := &Tool{Name: name, Description: desc, Parameters: params, Run: fn}
	r.tools[name] = t
	return t
}

// Get returns a tool by name.
func (r *ToolRegistry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Invoke calls a tool by name.
func (r *ToolRegistry) Invoke(ctx context.Context, name string, args map[string]string) (string, error) {
	t := r.Get(name)
	if t == nil {
		return "", fmt.Errorf("tool '%s' not found (available: %s)", name, r.Names())
	}
	return t.Run(ctx, args)
}

// Names returns comma-separated tool names.
func (r *ToolRegistry) Names() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return strings.Join(r.order, ", ")
}

// Count returns the number of tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// List returns all tools in order.
func (r *ToolRegistry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Tool, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.tools[name])
	}
	return result
}

// SystemPrompt generates a system prompt section describing all registered tools.
// Injected automatically into the agent's system prompt so the LLM knows what
// tools are available and is instructed to use them instead of guessing.
// Mirrors Agno's automatic tool instruction injection.
func (r *ToolRegistry) SystemPrompt() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.order) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n## Tools\n")
	b.WriteString("You have access to the following tools. IMPORTANT RULES:\n")
	b.WriteString("- You do NOT have access to real-time information (current date, time, weather, prices, etc.) — ALWAYS use a tool to get it.\n")
	b.WriteString("- NEVER guess, assume, or make up information that a tool can provide. Call the tool first.\n")
	b.WriteString("- If a tool needs a parameter you can reasonably infer from context, use your best guess rather than asking the user.\n")
	b.WriteString("- Call tools proactively — don't ask the user for information the tool can give you.\n\n")
	for _, name := range r.order {
		t := r.tools[name]
		b.WriteString(fmt.Sprintf("- **%s**: %s", t.Name, t.Description))
		if len(t.Parameters) > 0 {
			params := make([]string, 0, len(t.Parameters))
			for pName, p := range t.Parameters {
				req := ""
				if p.Required {
					req = ", required"
				}
				params = append(params, fmt.Sprintf("`%s` (%s%s)", pName, p.Type, req))
			}
			b.WriteString(fmt.Sprintf(" — params: %s", strings.Join(params, ", ")))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FunctionDefs generates OpenAI-compatible function calling definitions.
// Works with any provider that uses the OpenAI tool format (most do).
func (r *ToolRegistry) FunctionDefs() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]map[string]any, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		props := map[string]any{}
		var required []string
		for pName, p := range t.Parameters {
			prop := map[string]any{"type": p.Type, "description": p.Desc}
			if len(p.Enum) > 0 {
				prop["enum"] = p.Enum
			}
			props[pName] = prop
			if p.Required {
				required = append(required, pName)
			}
		}

		fn := map[string]any{"name": t.Name, "description": t.Description}
		if len(props) > 0 {
			params := map[string]any{"type": "object", "properties": props}
			if len(required) > 0 {
				params["required"] = required
			}
			fn["parameters"] = params
		}
		defs = append(defs, map[string]any{"type": "function", "function": fn})
	}
	return defs
}

// ParseArgs parses a JSON string into a string map.
func ParseArgs(argsJSON string) map[string]string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			result[k] = val
		default:
			b, _ := json.Marshal(val)
			result[k] = string(b)
		}
	}
	return result
}
