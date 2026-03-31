//go:build ignore

// Run context -- inject dependencies into tools via context.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/10_production/run_context.go
package main

import (
	"context"
	"fmt"

	"github.com/saeedalam/agnogo"
)

type UserInfoInput struct {
	Field string `json:"field" desc:"Which field: name, email, or plan" required:"true"`
}

func main() {
	userLookup := agnogo.TypedTool("user_info", "Get current user info",
		func(ctx context.Context, in UserInfoInput) (string, error) {
			rc := agnogo.RunCtx(ctx)
			if rc == nil {
				return "No user context available", nil
			}
			switch in.Field {
			case "name":
				return rc.GetStr("user_name"), nil
			case "email":
				return rc.GetStr("user_email"), nil
			case "plan":
				return rc.GetStr("user_plan"), nil
			default:
				return "Unknown field: " + in.Field, nil
			}
		})

	agent := agnogo.Agent("You are a support assistant. Use the user_info tool to look up user details when needed.",
		agnogo.Tools(userLookup),
	)

	// Inject user dependencies
	rctx := agnogo.NewRunContext()
	rctx.Set("user_name", "Erik Svensson")
	rctx.Set("user_email", "erik@example.com")
	rctx.Set("user_plan", "Premium")
	ctx := rctx.WithContext(context.Background())

	session := agnogo.NewSession("support-1")
	resp, err := agent.Run(ctx, session, "What plan am I on and what's my email?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(resp.Text)
}
