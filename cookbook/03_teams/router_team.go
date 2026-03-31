//go:build ignore

// Team routing -- messages route to booking or support agent. Try:
//
//	"I want to book a haircut"
//	"What services do you offer?"
//	"Can I schedule an appointment for tomorrow?"
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/03_teams/router_team.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
)

type CheckAvailInput struct {
	Date string `json:"date" desc:"Date (YYYY-MM-DD)" required:"true"`
}

func main() {
	checkAvail := agnogo.TypedTool(
		"check_availability", "Check available time slots",
		func(ctx context.Context, in CheckAvailInput) (string, error) {
			return fmt.Sprintf("Available slots for %s: 10:00, 11:30, 14:00, 15:30", in.Date), nil
		},
	)

	bookingAgent := agnogo.Agent(
		"You are a booking assistant for a hair salon. Help customers schedule appointments.",
		checkAvail,
	)

	supportAgent := agnogo.Agent(
		"You are a customer support agent for a hair salon. Answer questions about services, prices, and policies.",
	)

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

	fmt.Println("Salon Team Agent -- type your message (exit to quit)")
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
