package agnogo

import (
	"context"
	"fmt"
	"log/slog"
)

// Team routes conversations to specialized sub-agents based on intent.
//
//	team := agnogo.NewTeam(agnogo.TeamConfig{
//	    Model: agnogo.OpenAI(key, "gpt-4.1-mini"),
//	})
//	team.Agent("booking", bookingAgent)
//	team.Agent("support", supportAgent)
//	team.Agent("complaint", complaintAgent)
//	resp, _ := team.Run(ctx, session, "I want to book a haircut")
//	// → routes to "booking" agent
type Team struct {
	agents     map[string]*Agent
	order      []string
	routerFunc func(ctx context.Context, msg string, agents []string) (string, error)
	fallback   string // agent name to use if routing fails
}

// TeamConfig configures a Team.
type TeamConfig struct {
	// RouterFunc classifies the user message and returns an agent name.
	// If nil, uses LLM-based intent classification.
	RouterFunc func(ctx context.Context, msg string, agents []string) (string, error)

	// Model is used for LLM-based routing (only needed if RouterFunc is nil).
	Model ModelProvider

	// Fallback agent name if routing fails. Defaults to first registered agent.
	Fallback string
}

// NewTeam creates a team with sub-agent routing.
func NewTeam(cfg TeamConfig) *Team {
	t := &Team{
		agents:   make(map[string]*Agent),
		fallback: cfg.Fallback,
	}

	if cfg.RouterFunc != nil {
		t.routerFunc = cfg.RouterFunc
	} else if cfg.Model != nil {
		// LLM-based router
		model := cfg.Model
		t.routerFunc = func(ctx context.Context, msg string, agents []string) (string, error) {
			prompt := fmt.Sprintf(
				"Classify this message into ONE of these categories: %v\nMessage: %q\nRespond with ONLY the category name, nothing else.",
				agents, msg,
			)
			resp, err := model.ChatCompletion(ctx, []Message{
				{Role: "user", Content: prompt},
			}, nil)
			if err != nil {
				return "", err
			}
			return resp.Text, nil
		}
	}

	return t
}

// Agent registers a sub-agent with a name (used for routing).
func (t *Team) Agent(name string, agent *Agent) *Team {
	t.agents[name] = agent
	t.order = append(t.order, name)
	if t.fallback == "" {
		t.fallback = name // first agent becomes default fallback
	}
	return t
}

// Run routes the message to the right sub-agent and executes it.
func (t *Team) Run(ctx context.Context, session *Session, userMessage string) (*Response, error) {
	if len(t.agents) == 0 {
		return nil, fmt.Errorf("no agents registered in team")
	}

	// Single agent — no routing needed
	if len(t.agents) == 1 {
		for _, a := range t.agents {
			return a.Run(ctx, session, userMessage)
		}
	}

	// Route to the right agent
	agentName := t.fallback
	if t.routerFunc != nil {
		name, err := t.routerFunc(ctx, userMessage, t.order)
		if err != nil {
			slog.Warn("agnogo: routing failed, using fallback", "error", err, "fallback", t.fallback)
		} else {
			// Clean the router response (LLM might add quotes or whitespace)
			name = cleanAgentName(name, t.order)
			if name != "" {
				agentName = name
			}
		}
	}

	agent, ok := t.agents[agentName]
	if !ok {
		agent = t.agents[t.fallback]
	}

	slog.Info("agnogo: team routing", "agent", agentName, "session", session.ID)
	session.Set("_routed_to", agentName)

	return agent.Run(ctx, session, userMessage)
}

// cleanAgentName finds the best matching agent name from a raw LLM response.
func cleanAgentName(raw string, valid []string) string {
	raw = trimQuotes(raw)
	for _, name := range valid {
		if eqFold(raw, name) {
			return name
		}
	}
	// Partial match
	for _, name := range valid {
		if containsFold(raw, name) {
			return name
		}
	}
	return ""
}

func eqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func containsFold(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if eqFold(s[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
