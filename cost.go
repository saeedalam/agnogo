package agnogo

import (
	"errors"
	"fmt"
	"time"
)

// ErrBudgetExceeded is returned when a cost budget limit is hit.
var ErrBudgetExceeded = errors.New("cost budget exceeded")

// CostBudget limits how much an agent can spend.
type CostBudget struct {
	MaxPerRun     float64                    // max USD per Run() call (0 = unlimited)
	MaxPerSession float64                    // max USD per session lifetime (0 = unlimited)
	MaxPerMinute  float64                    // max USD per rolling minute (0 = unlimited)
	OnExceeded    func(spent, limit float64) // optional callback when budget exceeded
}

// runCostTracker tracks costs within a single Run() call.
type runCostTracker struct {
	budget      *CostBudget
	prices      *CostTracker
	runCost     float64
	startTime   time.Time
	minuteCosts []minuteEntry
}

type minuteEntry struct {
	at   time.Time
	cost float64
}

func newRunCostTracker(budget *CostBudget) *runCostTracker {
	return &runCostTracker{
		budget:    budget,
		prices:    NewCostTracker(),
		startTime: time.Now(),
	}
}

// addUsage records cost from a model call.
func (t *runCostTracker) addUsage(model string, usage *Usage) {
	if usage == nil {
		return
	}
	cost := t.prices.Estimate(model, usage)
	if cost == 0 {
		// Fallback to gpt-4.1-mini pricing if model unknown.
		cost = t.prices.Estimate("gpt-4.1-mini", usage)
	}
	t.runCost += cost
	t.minuteCosts = append(t.minuteCosts, minuteEntry{at: time.Now(), cost: cost})
}

// totalCost returns accumulated run cost.
func (t *runCostTracker) totalCost() float64 {
	return t.runCost
}

// rollingMinuteCost returns total cost in the last 60 seconds.
func (t *runCostTracker) rollingMinuteCost() float64 {
	cutoff := time.Now().Add(-time.Minute)
	var total float64
	for _, e := range t.minuteCosts {
		if e.at.After(cutoff) {
			total += e.cost
		}
	}
	return total
}

// checkBudget checks all limits. Returns error if any exceeded.
func (t *runCostTracker) checkBudget(session *Session) error {
	if t.budget == nil {
		return nil
	}

	// Per-run check
	if t.budget.MaxPerRun > 0 && t.runCost > t.budget.MaxPerRun {
		if t.budget.OnExceeded != nil {
			t.budget.OnExceeded(t.runCost, t.budget.MaxPerRun)
		}
		return fmt.Errorf("%w: run cost $%.4f exceeds limit $%.4f", ErrBudgetExceeded, t.runCost, t.budget.MaxPerRun)
	}

	// Per-session check
	if t.budget.MaxPerSession > 0 && session != nil {
		sessionCost := getSessionCost(session) + t.runCost
		if sessionCost > t.budget.MaxPerSession {
			if t.budget.OnExceeded != nil {
				t.budget.OnExceeded(sessionCost, t.budget.MaxPerSession)
			}
			return fmt.Errorf("%w: session cost $%.4f exceeds limit $%.4f", ErrBudgetExceeded, sessionCost, t.budget.MaxPerSession)
		}
	}

	// Per-minute check
	if t.budget.MaxPerMinute > 0 {
		minuteCost := t.rollingMinuteCost()
		if minuteCost > t.budget.MaxPerMinute {
			if t.budget.OnExceeded != nil {
				t.budget.OnExceeded(minuteCost, t.budget.MaxPerMinute)
			}
			return fmt.Errorf("%w: per-minute cost $%.4f exceeds limit $%.4f", ErrBudgetExceeded, minuteCost, t.budget.MaxPerMinute)
		}
	}

	return nil
}

// getSessionCost reads the accumulated cost from session state.
func getSessionCost(session *Session) float64 {
	v := session.Get("_total_cost")
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// setSessionCost stores the accumulated cost in session state.
func setSessionCost(session *Session, cost float64) {
	session.Set("_total_cost", cost)
}

// WithBudget sets a cost budget for the agent.
//
//	agent := agnogo.Agent("...", agnogo.WithBudget(agnogo.CostBudget{
//	    MaxPerRun:     0.10,
//	    MaxPerSession: 1.00,
//	}))
func WithBudget(budget CostBudget) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.costBudget = &budget
	})
}
