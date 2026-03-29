//go:build ignore

// Team routing — messages route to booking or support agent. Try:
//   "I want to book a haircut"
//   "What services do you offer?"
//   "Can I schedule an appointment for tomorrow?"
//
//	source .env && go run ./cookbook/03_teams/router_team.go
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

	bookingAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a booking assistant for a hair salon. Help customers schedule appointments.",
		Debug:        &debug,
	})
	bookingAgent.Tool("check_availability", "Check available time slots", agnogo.Params{
		"date": {Type: "string", Desc: "Date (YYYY-MM-DD)", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf(`Available slots for %s: 10:00, 11:30, 14:00, 15:30`, args["date"]), nil
	})

	supportAgent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a customer support agent for a hair salon. Answer questions about services, prices, and policies.",
		Debug:        &debug,
	})

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

	// Interactive loop
	fmt.Println("🏠 Salon Team Agent — type your message (exit to quit)")
	fmt.Println("  Routes to: booking (book/appointment) or support (everything else)")
	fmt.Println()

	session := agnogo.NewSession("customer-1")
	scanner := newScanner()
	for {
		fmt.Print("You > ")
		msg := scanner.ReadLine()
		if msg == "" {
			continue
		}
		if msg == "exit" || msg == "quit" {
			break
		}
		resp, err := team.Run(context.Background(), session, msg)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("\nAgent > %s\n\n", resp.Text)
	}
}

type lineScanner struct{ r *os.File }

func newScanner() lineScanner { return lineScanner{os.Stdin} }
func (s lineScanner) ReadLine() string {
	buf := make([]byte, 0, 256)
	b := make([]byte, 1)
	for {
		n, err := s.r.Read(b)
		if n == 0 || err != nil {
			return string(buf)
		}
		if b[0] == '\n' {
			return strings.TrimSpace(string(buf))
		}
		buf = append(buf, b[0])
	}
}
