package agnogo

import (
	"context"
	"fmt"
)

// Hook is a function that wraps an agent run. It receives the agent, session,
// message, and a next function to call. Hooks can modify inputs, inspect outputs,
// or short-circuit the chain.
//
//	// Logging hook
//	logging := func(ctx context.Context, a *Core, s *Session, msg string, next agnogo.NextFunc) (*Response, error) {
//	    log.Printf("Run start: %s", msg)
//	    resp, err := next(ctx, a, s, msg)
//	    log.Printf("Run end: %s", resp.Text[:50])
//	    return resp, err
//	}
//
//	// Auth hook
//	auth := func(ctx context.Context, a *Core, s *Session, msg string, next agnogo.NextFunc) (*Response, error) {
//	    if s.GetMeta("auth_token") == "" {
//	        return &Response{Text: "Unauthorized"}, nil
//	    }
//	    return next(ctx, a, s, msg)
//	}
//
//	agent := agnogo.Agent("...", agnogo.WithHooks(logging, auth))
type NextFunc func(ctx context.Context, agent *Core, session *Session, msg string) (*Response, error)

// Hook is middleware that wraps an agent's Run call.
// Call next to continue the chain, or return early to short-circuit.
type Hook func(ctx context.Context, agent *Core, session *Session, msg string, next NextFunc) (*Response, error)

// WithHooks adds middleware hooks that wrap every Run() call.
// Hooks execute in order: first hook is outermost wrapper.
func WithHooks(hooks ...Hook) Option {
	return optionFunc(func(sc *smartConfig) {
		sc.hooks = append(sc.hooks, hooks...)
	})
}

// runWithHooks builds the hook chain and executes it.
// The innermost function calls the real Run logic (with hooks temporarily removed
// to avoid infinite recursion).
func (a *Core) runWithHooks(ctx context.Context, session *Session, userMessage string) (*Response, error) {
	inner := func(ctx context.Context, _ *Core, s *Session, msg string) (*Response, error) {
		saved := a.hooks
		a.hooks = nil
		defer func() { a.hooks = saved }()
		return a.Run(ctx, s, msg)
	}

	chain := inner
	for i := len(a.hooks) - 1; i >= 0; i-- {
		hook := a.hooks[i]
		next := chain
		chain = func(ctx context.Context, agent *Core, s *Session, msg string) (*Response, error) {
			return hook(ctx, agent, s, msg, next)
		}
	}

	// Execute with panic recovery
	var resp *Response
	var err error
	func() {
		defer func() {
			if p := recover(); p != nil {
				err = fmt.Errorf("agnogo: hook panicked: %v", p)
			}
		}()
		resp, err = chain(ctx, a, session, userMessage)
	}()
	return resp, err
}
