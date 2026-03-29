package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Shell returns a tool for running shell commands.
//
// WARNING: Only use in trusted environments.
// When allowedCommands is set, commands are executed directly (not via sh -c)
// to prevent shell injection. Without an allowlist, commands run through sh -c.
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
			cmd := strings.TrimSpace(args["command"])
			if cmd == "" {
				return "Error: empty command", nil
			}

			var out []byte
			var err error

			if len(allowed) > 0 {
				// Safe mode: parse and execute directly (no shell interpretation)
				parts := strings.Fields(cmd)
				if len(parts) == 0 {
					return "Error: empty command", nil
				}
				if !allowed[parts[0]] {
					return fmt.Sprintf("Command '%s' not in allowlist", parts[0]), nil
				}
				// Reject shell metacharacters to prevent injection
				for _, ch := range cmd {
					if ch == ';' || ch == '|' || ch == '&' || ch == '`' || ch == '$' || ch == '(' || ch == ')' || ch == '>' || ch == '<' {
						return fmt.Sprintf("Shell metacharacter '%c' not allowed in safe mode", ch), nil
					}
				}
				out, err = exec.CommandContext(ctx, parts[0], parts[1:]...).CombinedOutput()
			} else {
				// Unrestricted mode — caller accepts the risk
				out, err = exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
			}

			if err != nil {
				return fmt.Sprintf("Error: %s\nOutput: %s", err, string(out)), nil
			}
			result := string(out)
			if len(result) > 2000 {
				result = result[:2000] + "\n... (truncated)"
			}
			return result, nil
		},
	}}
}
