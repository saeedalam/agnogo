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

// DockerConfig configures Docker tools.
type DockerConfig struct {
	// DefaultTimeout in seconds for docker run/build. Default: 60.
	DefaultTimeout int
	// MaxOutputSize in bytes before truncation. Default: 8000.
	MaxOutputSize int
	// DefaultAutoRemove controls whether containers are removed after exit. Default: true.
	DefaultAutoRemove bool
}

func (c *DockerConfig) defaults() {
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 60
	}
	if c.MaxOutputSize <= 0 {
		c.MaxOutputSize = 8000
	}
	// DefaultAutoRemove is true by default (zero value is false, so we use a flag)
}

// Docker returns tools for managing Docker containers.
// Uses the docker CLI (must be installed and accessible).
func Docker(cfgs ...DockerConfig) []agnogo.ToolDef {
	var cfg DockerConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	} else {
		cfg.DefaultAutoRemove = true
	}
	cfg.defaults()

	dockerExec := func(ctx context.Context, timeout time.Duration, args ...string) (string, string, int, error) {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, "docker", args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return "", "", -1, fmt.Errorf("docker command failed: %w", err)
			}
		}

		return truncateOutput(stdout.String(), cfg.MaxOutputSize),
			truncateOutput(stderr.String(), cfg.MaxOutputSize),
			exitCode, nil
	}

	dockerResult := func(stdout, stderr string, exitCode int) string {
		result := map[string]any{
			"exit_code": exitCode,
			"stdout":    stdout,
		}
		if stderr != "" {
			result["stderr"] = stderr
		}
		out, _ := json.Marshal(result)
		return string(out)
	}

	defaultTimeout := time.Duration(cfg.DefaultTimeout) * time.Second

	parseTimeout := func(args map[string]string) (time.Duration, error) {
		if t := strings.TrimSpace(args["timeout"]); t != "" {
			secs, err := strconv.Atoi(t)
			if err != nil || secs <= 0 {
				return 0, fmt.Errorf("invalid timeout: %q", t)
			}
			if secs > 3600 {
				return 0, fmt.Errorf("timeout %d exceeds maximum of 3600 seconds", secs)
			}
			return time.Duration(secs) * time.Second, nil
		}
		return defaultTimeout, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "docker_ps",
			Desc: "List running Docker containers",
			Params: agnogo.Params{
				"all": {Type: "boolean", Desc: "Show all containers (including stopped)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				cmd := []string{"ps", "--format", "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"}
				if args["all"] == "true" {
					cmd = append(cmd, "-a")
				}
				stdout, stderr, code, err := dockerExec(ctx, 30*time.Second, cmd...)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_run",
			Desc: "Run a Docker container",
			Params: agnogo.Params{
				"image":    {Type: "string", Desc: "Docker image name", Required: true},
				"command":  {Type: "string", Desc: "Command to run in container"},
				"detach":   {Type: "boolean", Desc: "Run in background"},
				"auto_rm":  {Type: "boolean", Desc: fmt.Sprintf("Remove container after exit (default %v)", cfg.DefaultAutoRemove)},
				"memory":   {Type: "string", Desc: "Memory limit (e.g. '256m', '1g')"},
				"cpus":     {Type: "string", Desc: "Number of CPUs (e.g. '0.5', '2')"},
				"timeout":  {Type: "string", Desc: fmt.Sprintf("Timeout in seconds (default %d)", cfg.DefaultTimeout)},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				image := strings.TrimSpace(args["image"])
				if image == "" {
					return "", fmt.Errorf("missing required parameter: image")
				}

				timeout, err := parseTimeout(args)
				if err != nil {
					return "", err
				}

				cmd := []string{"run"}

				// Auto-remove: default from config, overridable by param
				autoRm := cfg.DefaultAutoRemove
				if v := strings.TrimSpace(args["auto_rm"]); v != "" {
					autoRm = v == "true"
				}
				if autoRm {
					cmd = append(cmd, "--rm")
				}

				if args["detach"] == "true" {
					cmd = append(cmd, "-d")
				}
				if mem := strings.TrimSpace(args["memory"]); mem != "" {
					cmd = append(cmd, "--memory", mem)
				}
				if cpus := strings.TrimSpace(args["cpus"]); cpus != "" {
					cmd = append(cmd, "--cpus", cpus)
				}

				cmd = append(cmd, image)
				if c := strings.TrimSpace(args["command"]); c != "" {
					cmd = append(cmd, strings.Fields(c)...)
				}

				stdout, stderr, code, err := dockerExec(ctx, timeout, cmd...)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_stop",
			Desc: "Stop a running container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				container := strings.TrimSpace(args["container"])
				if container == "" {
					return "", fmt.Errorf("missing required parameter: container")
				}
				stdout, stderr, code, err := dockerExec(ctx, 30*time.Second, "stop", container)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_logs",
			Desc: "Get logs from a container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
				"tail":      {Type: "string", Desc: "Number of lines (default 50)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				container := strings.TrimSpace(args["container"])
				if container == "" {
					return "", fmt.Errorf("missing required parameter: container")
				}
				tail := "50"
				if t := strings.TrimSpace(args["tail"]); t != "" {
					tail = t
				}
				stdout, stderr, code, err := dockerExec(ctx, 30*time.Second, "logs", "--tail", tail, container)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_images",
			Desc: "List Docker images",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				stdout, stderr, code, err := dockerExec(ctx, 30*time.Second, "images", "--format", "table {{.Repository}}\t{{.Tag}}\t{{.Size}}")
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_exec",
			Desc: "Execute a command in a running container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
				"command":   {Type: "string", Desc: "Command to execute", Required: true},
				"timeout":   {Type: "string", Desc: fmt.Sprintf("Timeout in seconds (default %d)", cfg.DefaultTimeout)},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				container := strings.TrimSpace(args["container"])
				if container == "" {
					return "", fmt.Errorf("missing required parameter: container")
				}
				command := strings.TrimSpace(args["command"])
				if command == "" {
					return "", fmt.Errorf("missing required parameter: command")
				}

				timeout, err := parseTimeout(args)
				if err != nil {
					return "", err
				}

				cmd := []string{"exec", container}
				cmd = append(cmd, strings.Fields(command)...)
				stdout, stderr, code, err := dockerExec(ctx, timeout, cmd...)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_inspect",
			Desc: "Get detailed information about a container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				container := strings.TrimSpace(args["container"])
				if container == "" {
					return "", fmt.Errorf("missing required parameter: container")
				}
				stdout, stderr, code, err := dockerExec(ctx, 30*time.Second, "inspect", container)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
		{
			Name: "docker_build",
			Desc: "Build a Docker image from a Dockerfile",
			Params: agnogo.Params{
				"path":    {Type: "string", Desc: "Path to build context (directory containing Dockerfile)", Required: true},
				"tag":     {Type: "string", Desc: "Image tag (e.g. 'myapp:latest')", Required: true},
				"timeout": {Type: "string", Desc: fmt.Sprintf("Timeout in seconds (default %d)", cfg.DefaultTimeout)},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				path := strings.TrimSpace(args["path"])
				if path == "" {
					return "", fmt.Errorf("missing required parameter: path")
				}
				tag := strings.TrimSpace(args["tag"])
				if tag == "" {
					return "", fmt.Errorf("missing required parameter: tag")
				}

				timeout, err := parseTimeout(args)
				if err != nil {
					return "", err
				}

				stdout, stderr, code, err := dockerExec(ctx, timeout, "build", "-t", tag, path)
				if err != nil {
					return "", err
				}
				return dockerResult(stdout, stderr, code), nil
			},
		},
	}
}

func truncateOutput(s string, maxSize int) string {
	if len(s) <= maxSize {
		return s
	}
	half := maxSize / 2
	return s[:half] + "\n... (truncated) ...\n" + s[len(s)-half:]
}
