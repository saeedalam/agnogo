package agnogo

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ── Trace Persistence & Query ────────────────────────────────────────
//
// Layer 1 of Trace Intelligence. Store traces, find them later.
//
// Usage:
//
//	store := agnogo.NewMemoryTraceStore()
//	sc := agnogo.NewSpanCollector().WithTraceStore(store)
//	agent := agnogo.Agent("...", agnogo.WithSpanCollector(sc))
//
//	resp, _ := agent.Run(ctx, session, "Book Thursday 2pm")
//	sc.Collect(resp) // auto-saved to store
//
//	// Later: find expensive runs
//	expensive, _ := store.QueryTraces(ctx, agnogo.TraceQuery{MinCost: 0.05})

// TraceStore persists and queries agent traces.
// Implement this interface for your backend (Postgres, SQLite, etc.).
// MemoryTraceStore is provided for development and testing.
type TraceStore interface {
	SaveTrace(ctx context.Context, trace *RunTrace) error
	LoadTrace(ctx context.Context, runID string) (*RunTrace, error)
	QueryTraces(ctx context.Context, q TraceQuery) ([]*RunTrace, error)
	DeleteTrace(ctx context.Context, runID string) error
}

// TraceQuery filters traces. Zero values mean "no filter".
type TraceQuery struct {
	SessionID   string        // filter by session
	MinCost     float64       // cost >= this
	MaxCost     float64       // cost <= this (0 = no limit)
	MinDuration time.Duration // duration >= this
	HasErrors   *bool         // only traces with/without errors
	Since       time.Time     // created after this
	Until       time.Time     // created before this
	Limit       int           // max results (0 = all)
}

// ── MemoryTraceStore ─────────────────────────────────────────────────

// MemoryTraceStore is an in-memory implementation of TraceStore.
// Good for development, testing, and short-lived processes.
type MemoryTraceStore struct {
	mu     sync.RWMutex
	traces map[string]*RunTrace
	order  []string // insertion order for stable queries
}

// NewMemoryTraceStore creates an empty in-memory trace store.
func NewMemoryTraceStore() *MemoryTraceStore {
	return &MemoryTraceStore{
		traces: make(map[string]*RunTrace),
	}
}

func (s *MemoryTraceStore) SaveTrace(_ context.Context, trace *RunTrace) error {
	if trace == nil || trace.RunID == "" {
		return fmt.Errorf("agnogo: trace has no RunID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.traces[trace.RunID]; !exists {
		s.order = append(s.order, trace.RunID)
	}
	s.traces[trace.RunID] = trace
	return nil
}

func (s *MemoryTraceStore) LoadTrace(_ context.Context, runID string) (*RunTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trace, ok := s.traces[runID]
	if !ok {
		return nil, fmt.Errorf("agnogo: trace %q not found", runID)
	}
	return trace, nil
}

func (s *MemoryTraceStore) DeleteTrace(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.traces, runID)
	// Remove from order
	for i, id := range s.order {
		if id == runID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

func (s *MemoryTraceStore) QueryTraces(_ context.Context, q TraceQuery) ([]*RunTrace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*RunTrace
	for _, id := range s.order {
		trace := s.traces[id]
		if trace == nil {
			continue
		}
		if !matchesQuery(trace, q) {
			continue
		}
		results = append(results, trace)
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results, nil
}

// Count returns the number of stored traces.
func (s *MemoryTraceStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.traces)
}

func matchesQuery(trace *RunTrace, q TraceQuery) bool {
	if q.SessionID != "" && trace.SessionID != q.SessionID {
		return false
	}
	if q.MinCost > 0 && trace.TotalCost < q.MinCost {
		return false
	}
	if q.MaxCost > 0 && trace.TotalCost > q.MaxCost {
		return false
	}
	if q.MinDuration > 0 && trace.Duration < q.MinDuration {
		return false
	}
	if q.HasErrors != nil {
		if *q.HasErrors && !trace.HasErrors {
			return false
		}
		if !*q.HasErrors && trace.HasErrors {
			return false
		}
	}
	if !q.Since.IsZero() && trace.StartTime.Before(q.Since) {
		return false
	}
	if !q.Until.IsZero() && trace.StartTime.After(q.Until) {
		return false
	}
	return true
}
