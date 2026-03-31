//go:build ignore

// Middleware hooks -- wrap every agent run with logging and timing.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/10_production/middleware.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

func main() {
	// Timing hook
	timer := func(ctx context.Context, a *agnogo.Core, s *agnogo.Session, msg string, next agnogo.NextFunc) (*agnogo.Response, error) {
		start := time.Now()
		resp, err := next(ctx, a, s, msg)
		fmt.Printf("[timer] Run took %s\n", time.Since(start).Round(time.Millisecond))
		return resp, err
	}

	// Logging hook
	logger := func(ctx context.Context, a *agnogo.Core, s *agnogo.Session, msg string, next agnogo.NextFunc) (*agnogo.Response, error) {
		fmt.Printf("[log] Input: %s\n", msg)
		resp, err := next(ctx, a, s, msg)
		if err == nil {
			preview := resp.Text
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			fmt.Printf("[log] Output: %s\n", preview)
		}
		return resp, err
	}

	agent := agnogo.Agent("You are helpful. Be concise.", agnogo.WithHooks(timer, logger))

	answer, err := agent.Ask(context.Background(), "Name 3 programming languages.")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("\nAnswer:", answer)
}
