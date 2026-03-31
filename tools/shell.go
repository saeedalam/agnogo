package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// ShellConfig configures the shell tool.
type ShellConfig struct {
	// DefaultTimeout in seconds. Default: 30.
	DefaultTimeout int
	// MaxOutputSize in bytes before head+tail truncation. Default: 4000.
	MaxOutputSize int
	// HeadSize is how many characters to keep from the start when truncating. Default: 1000.
	HeadSize int
	// TailSize is how many characters to keep from the end when truncating. Default: 500.
	TailSize int
}

func (c *ShellConfig) defaults() {
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 30
	}
	if c.MaxOutputSize <= 0 {
		c.MaxOutputSize = 4000
	}
	if c.HeadSize <= 0 {
		c.HeadSize = 1000
	}
	if c.TailSize <= 0 {
		c.TailSize = 500
	}
}

// Shell returns a tool for running shell commands.
//
// WARNING: Only use in trusted environments.
// When allowedCommands is set, commands are executed directly (not via sh -c)
// to prevent shell injection. Without an allowlist, commands run through sh -c.
func Shell(allowedCommands ...string) []agnogo.ToolDef {
	return ShellWithConfig(ShellConfig{}, allowedCommands...)
}

// ShellWithConfig returns a shell tool with explicit configuration.
func ShellWithConfig(cfg ShellConfig, allowedCommands ...string) []agnogo.ToolDef {
	cfg.defaults()

	allowed := map[string]bool{}
	for _, c := range allowedCommands {
		allowed[c] = true
	}

	return []agnogo.ToolDef{{
		Name: "shell",
		Desc: "Run a shell command and return structured output with stdout, stderr, and exit code",
		Params: agnogo.Params{
			"command":     {Type: "string", Desc: "Command to execute", Required: true},
			"timeout":     {Type: "string", Desc: fmt.Sprintf("Timeout in seconds (default %d)", cfg.DefaultTimeout)},
			"working_dir": {Type: "string", Desc: "Working directory (optional)"},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", fmt.Errorf("context cancelled: %w", err)
			}

			cmd := strings.TrimSpace(args["command"])
			if cmd == "" {
				return "", fmt.Errorf("missing required parameter: command")
			}

			// Parse timeout
			timeout := time.Duration(cfg.DefaultTimeout) * time.Second
			if t := strings.TrimSpace(args["timeout"]); t != "" {
				secs, err := strconv.Atoi(t)
				if err != nil || secs <= 0 {
					return "", fmt.Errorf("invalid timeout: %q (must be positive integer seconds)", t)
				}
				if secs > 600 {
					return "", fmt.Errorf("timeout %d exceeds maximum of 600 seconds", secs)
				}
				timeout = time.Duration(secs) * time.Second
			}

			cmdCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var execCmd *exec.Cmd
			if len(allowed) > 0 {
				// Safe mode: parse and execute directly (no shell interpretation)
				parts := strings.Fields(cmd)
				if len(parts) == 0 {
					return "", fmt.Errorf("empty command after parsing")
				}
				if !allowed[parts[0]] {
					return "", fmt.Errorf("command %q not in allowlist", parts[0])
				}
				// Reject shell metacharacters to prevent injection
				for _, ch := range cmd {
					if ch == ';' || ch == '|' || ch == '&' || ch == '`' || ch == '$' || ch == '(' || ch == ')' || ch == '>' || ch == '<' {
						return "", fmt.Errorf("shell metacharacter %q not allowed in safe mode", string(ch))
					}
				}
				execCmd = exec.CommandContext(cmdCtx, parts[0], parts[1:]...)
			} else {
				// Unrestricted mode -- caller accepts the risk
				execCmd = exec.CommandContext(cmdCtx, "sh", "-c", cmd)
			}

			if wd := strings.TrimSpace(args["working_dir"]); wd != "" {
				execCmd.Dir = wd
			}

			var stdout, stderr bytes.Buffer
			execCmd.Stdout = &stdout
			execCmd.Stderr = &stderr

			err := execCmd.Run()

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					return "", fmt.Errorf("execution error: %w", err)
				}
			}

			stdoutStr := truncateHeadTail(stdout.String(), cfg.MaxOutputSize, cfg.HeadSize, cfg.TailSize)
			stderrStr := truncateHeadTail(stderr.String(), cfg.MaxOutputSize, cfg.HeadSize, cfg.TailSize)

			result := map[string]any{
				"exit_code": exitCode,
				"stdout":    stdoutStr,
				"stderr":    stderrStr,
			}
			out, _ := json.Marshal(result)
			return string(out), nil
		},
	}}
}

// truncateHeadTail returns the string as-is if within maxSize, or returns
// the first headSize chars + "..." + last tailSize chars.
func truncateHeadTail(s string, maxSize, headSize, tailSize int) string {
	if len(s) <= maxSize {
		return s
	}
	if headSize+tailSize >= len(s) {
		return s
	}
	return s[:headSize] + "\n... (truncated) ...\n" + s[len(s)-tailSize:]
}
