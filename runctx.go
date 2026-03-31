package agnogo

import (
	"context"
	"sync"
)

// RunContext carries dependencies and metadata through an agent run.
// Tools access it via the standard context.Context.
//
//	rctx := agnogo.NewRunContext()
//	rctx.Set("user_id", "u-123")
//	rctx.Set("db", myDBConn)
//	ctx := rctx.WithContext(context.Background())
//
//	// Inside a tool:
//	func myTool(ctx context.Context, args map[string]string) (string, error) {
//	    rc := agnogo.RunCtx(ctx)
//	    userID := rc.GetStr("user_id")
//	    db := rc.Get("db").(*sql.DB)
//	    ...
//	}
type RunContext struct {
	mu   sync.RWMutex
	data map[string]any
}

type runContextKey struct{}

// NewRunContext creates a new empty RunContext.
func NewRunContext() *RunContext {
	return &RunContext{
		data: make(map[string]any),
	}
}

// Set stores a value in the RunContext.
func (rc *RunContext) Set(key string, value any) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.data[key] = value
}

// Get retrieves a value from the RunContext.
func (rc *RunContext) Get(key string) any {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.data[key]
}

// GetStr retrieves a value as a string. Returns "" if not found or not a string.
func (rc *RunContext) GetStr(key string) string {
	v := rc.Get(key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetInt retrieves a value as an int. Returns 0 if not found or not numeric.
func (rc *RunContext) GetInt(key string) int {
	v := rc.Get(key)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

// GetBool retrieves a value as a bool. Returns false if not found or not a bool.
func (rc *RunContext) GetBool(key string) bool {
	v := rc.Get(key)
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// WithContext returns a new context.Context carrying this RunContext.
func (rc *RunContext) WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

// RunCtx extracts a RunContext from a context.Context. Returns nil if not present.
func RunCtx(ctx context.Context) *RunContext {
	if rc, ok := ctx.Value(runContextKey{}).(*RunContext); ok {
		return rc
	}
	return nil
}
