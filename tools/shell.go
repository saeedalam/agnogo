package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Shell returns a tool for running shell commands.
// WARNING: Only use in trusted environments.
func Shell(allowedCommands ...string) []agnogo.ToolDef {
	allowed := map[string]bool{}
	for _, c := range allowedCommands {
		allowed[c] = true
	}
	return []agnogo.ToolDef{{
		Name: "shell", Desc: "Run a shell command and return output",
		Params: agnogo.Params{
			"command": {Type: "string", Desc: "Command to execute", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			cmd := args["command"]
			// Safety: check allowed list if provided
			if len(allowed) > 0 {
				parts := strings.Fields(cmd)
				if len(parts) == 0 || !allowed[parts[0]] {
					return fmt.Sprintf("Command '%s' not allowed", parts[0]), nil
				}
			}
			out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("Error: %s\nOutput: %s", err, string(out)), nil
			}
			// Limit output
			result := string(out)
			if len(result) > 2000 {
				result = result[:2000] + "\n... (truncated)"
			}
			return result, nil
		},
	}}
}
