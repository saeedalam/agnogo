package agnogo

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// CLI launches an interactive terminal chat with the agent.
// Matches Agno's agent.cli_app().
//
//	agent := agnogo.New(agnogo.Config{...})
//	agent.CLI() // interactive terminal
func (a *Agent) CLI() {
	ctx := context.Background()
	session := NewSession("cli")

	fmt.Println("🤖 Agent CLI (type 'exit' to quit, 'clear' to reset)")
	fmt.Println(strings.Repeat("─", 50))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}
		if input == "clear" {
			session = NewSession("cli")
			fmt.Println("Session cleared.")
			continue
		}
		if input == "memory" {
			if len(session.Memory) == 0 {
				fmt.Println("No memories stored.")
			} else {
				for k, v := range session.Memory {
					fmt.Printf("  %s: %s\n", k, v)
				}
			}
			continue
		}
		if input == "history" {
			for _, m := range session.History {
				role := m.Role
				content := m.Content
				if len(content) > 80 {
					content = content[:80] + "..."
				}
				fmt.Printf("  [%s] %s\n", role, content)
			}
			continue
		}
		if input == "tools" {
			for _, t := range a.tools.List() {
				fmt.Printf("  🔧 %s — %s\n", t.Name, t.Description)
			}
			continue
		}

		resp, err := a.Run(ctx, session, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		if resp.NeedsApproval {
			fmt.Printf("\n⏸️  Approval needed: %s\n", resp.Approval.Reason)
			fmt.Printf("   Tool: %s\n", resp.Approval.ToolName)
			fmt.Printf("   Args: %v\n", resp.Approval.Arguments)
			fmt.Print("   Approve? (y/n): ")
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				approved := answer == "y" || answer == "yes"
				resp, err = a.Resume(ctx, session, approved)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
			}
		}

		fmt.Printf("\n%s\n", resp.Text)
		if len(resp.ToolsCalled) > 0 {
			fmt.Printf("  (tools: %s)\n", strings.Join(resp.ToolsCalled, ", "))
		}
	}
}
