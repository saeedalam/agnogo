//go:build ignore

// Team routing — multiple agents, automatic intent-based routing.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/03_teams/router_team.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
	"github.com/saeedalam/agnogo/providers/openai"
)

func main() {
	model := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1-mini")
	debug := agnogo.DefaultDebug()

	// Booking agent — handles appointments
	bookingAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a booking assistant for a hair salon. Help customers schedule appointments. Be friendly.",
		Debug:        &debug,
	})
	bookingAgent.Tool("check_availability", "Check available time slots", agnogo.Params{
		"date": {Type: "string", Desc: "Date (YYYY-MM-DD)", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf(`Available slots for %s: 10:00, 11:30, 14:00, 15:30`, args["date"]), nil
	})

	// Support agent — handles questions
	supportAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a customer support agent. Answer questions about the salon's services and policies. Be helpful and concise.",
		Debug:        &debug,
	})

	// Team with custom routing
	team := agnogo.NewTeam(agnogo.TeamConfig{
		RouterFunc: func(ctx context.Context, msg string, agents []string) (string, error) {
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "book") || strings.Contains(lower, "appointment") || strings.Contains(lower, "schedule") {
				return "booking", nil
			}
			return "support", nil
		},
	})
	team.Agent("booking", bookingAgent)
	team.Agent("support", supportAgent)

	session := agnogo.NewSession("customer-1")
	ctx := context.Background()

	messages := []string{
		"What services do you offer?",
		"I'd like to book a haircut for tomorrow",
		"What are your prices?",
	}

	for _, msg := range messages {
		fmt.Printf("\n--- Customer: %s ---\n", msg)
		resp, err := team.Run(ctx, session, msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println(resp.Text)
	}
}
