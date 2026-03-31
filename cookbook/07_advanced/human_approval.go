//go:build ignore

// Human-in-the-loop -- require approval before executing dangerous tools.
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
)

// -- Typed tool inputs --

type ListFilesInput struct {
	Path string `json:"path" desc:"Directory path" required:"true"`
}

type DeleteFileInput struct {
	Path string `json:"path" desc:"File to delete" required:"true"`
}

func main() {
	// Safe tool -- no approval needed
	listFiles := agnogo.TypedTool(
		"list_files", "List files in a directory",
		func(ctx context.Context, in ListFilesInput) (string, error) {
			return fmt.Sprintf("Files in %s: config.yaml, data.db, logs.txt, README.md", in.Path), nil
		},
	)

	agent := agnogo.Agent("You are a system admin assistant. You can delete files when asked.",
		listFiles,
	)

	// Dangerous tool -- requires approval (ToolWithApproval still uses the old syntax
	// because it needs the approval reason parameter)
	agent.ToolWithApproval("delete_file", "Delete a file permanently", agnogo.Params{
		"path": {Type: "string", Desc: "File to delete", Required: true},
	}, func(ctx context.Context, args map[string]string) (string, error) {
		return fmt.Sprintf("Deleted: %s", args["path"]), nil
	}, "Permanent file deletion -- cannot be undone")

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
		fmt.Printf("\nTool '%s' needs approval: %s\n", resp.Approval.ToolName, resp.Approval.Reason)
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
