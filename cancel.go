package agnogo

import (
	"context"
	"sync"
)

// RunRegistry tracks active runs for cancellation.
// Matches Agno's cancel_run() static method.
//
//	// Start a run
//	ctx, runID := agnogo.RegisterRun(ctx)
//	go agent.Run(ctx, session, message)
//
//	// Cancel it
//	agnogo.CancelRun(runID)
var (
	activeRuns   = map[string]context.CancelFunc{}
	activeRunsMu sync.Mutex
)

// RegisterRun creates a cancellable context and registers it for later cancellation.
// Returns the context and a run ID.
func RegisterRun(ctx context.Context, runID string) (context.Context, string) {
	ctx, cancel := context.WithCancel(ctx)
	activeRunsMu.Lock()
	activeRuns[runID] = cancel
	activeRunsMu.Unlock()
	return ctx, runID
}

// CancelRun cancels an active run by ID. Matches Agno: Agent.cancel_run()
func CancelRun(runID string) {
	activeRunsMu.Lock()
	if cancel, ok := activeRuns[runID]; ok {
		cancel()
		delete(activeRuns, runID)
	}
	activeRunsMu.Unlock()
}

// UnregisterRun removes a completed run from the registry.
// Must be called by the caller when the run completes to prevent memory leaks.
func UnregisterRun(runID string) {
	activeRunsMu.Lock()
	delete(activeRuns, runID)
	activeRunsMu.Unlock()
}

// ActiveRunCount returns the number of currently active runs.
func ActiveRunCount() int {
	activeRunsMu.Lock()
	defer activeRunsMu.Unlock()
	return len(activeRuns)
}
