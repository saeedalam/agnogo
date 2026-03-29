//go:build ignore

// Human-in-the-loop — require approval before executing dangerous tools.
//
//	OPENAI_API_KEY=sk-... go run ./cookbook/07_advanced/human_approval.go
package main

import (
	"bufio"
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

	agent := agnogo.New(agnogo.Config{
		Model:        model,
		Instructions: "You are a system admin assistant. You can delete files when asked. Always confirm what you're about to delete.",
		Debug:        &debug,
	})

	// Safe tool — no approval needed
	agent.Tool("list_files", "List files in a directory", agnogo.Params{
		"path": {Type: "string", Desc: "Directory path", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf("Files in %s: config.yaml, data.db, logs.txt, README.md", args["path"]), nil
	})

	// Dangerous tool — requires approval
	agent.ToolWithApproval("delete_file", "Delete a file permanently", agnogo.Params{
		"path": {Type: "string", Desc: "File to delete", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf("Deleted: %s", args["path"]), nil
	}, "Permanent file deletion — cannot be undone")

	session := agnogo.NewSession("admin-1")
	ctx := context.Background()

	fmt.Println("--- Asking to delete a file ---")
	resp, err := agent.Run(ctx, session, "Delete the logs.txt file in /var/app/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(resp.Text)

	if resp.NeedsApproval {
		fmt.Printf("\n⚠️  Tool '%s' needs approval: %s\n", resp.Approval.ToolName, resp.Approval.Reason)
		fmt.Printf("   Arguments: %v\n", resp.Approval.Arguments)
		fmt.Print("\nApprove? (y/n): ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		approved := strings.TrimSpace(strings.ToLower(input)) == "y"

		fmt.Printf("\n--- %s ---\n", map[bool]string{true: "Approved", false: "Denied"}[approved])
		resp, err = agent.Resume(ctx, session, approved)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp.Text)
	}
}
