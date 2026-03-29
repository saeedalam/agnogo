package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Docker returns tools for managing Docker containers.
// Uses the docker CLI (must be installed and accessible).
func Docker() []agnogo.ToolDef {
	docker := func(ctx context.Context, args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error: %s\nOutput: %s", err, string(out)), nil
		}
		result := string(out)
		if len(result) > 3000 {
			result = result[:3000] + "\n... (truncated)"
		}
		return result, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "docker_ps", Desc: "List running Docker containers",
			Params: agnogo.Params{
				"all": {Type: "boolean", Desc: "Show all containers (including stopped)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cmd := []string{"ps", "--format", "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"}
				if args["all"] == "true" {
					cmd = append(cmd, "-a")
				}
				return docker(ctx, cmd...)
			},
		},
		{
			Name: "docker_run", Desc: "Run a Docker container",
			Params: agnogo.Params{
				"image":   {Type: "string", Desc: "Docker image name", Required: true},
				"command": {Type: "string", Desc: "Command to run in container"},
				"detach":  {Type: "boolean", Desc: "Run in background"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cmd := []string{"run", "--rm"}
				if args["detach"] == "true" {
					cmd = append(cmd, "-d")
				}
				cmd = append(cmd, args["image"])
				if args["command"] != "" {
					cmd = append(cmd, strings.Fields(args["command"])...)
				}
				return docker(ctx, cmd...)
			},
		},
		{
			Name: "docker_stop", Desc: "Stop a running container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return docker(ctx, "stop", args["container"])
			},
		},
		{
			Name: "docker_logs", Desc: "Get logs from a container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
				"tail":      {Type: "number", Desc: "Number of lines (default 50)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				tail := "50"
				if args["tail"] != "" {
					tail = args["tail"]
				}
				return docker(ctx, "logs", "--tail", tail, args["container"])
			},
		},
		{
			Name: "docker_images", Desc: "List Docker images",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return docker(ctx, "images", "--format", "table {{.Repository}}\t{{.Tag}}\t{{.Size}}")
			},
		},
		{
			Name: "docker_exec", Desc: "Execute a command in a running container",
			Params: agnogo.Params{
				"container": {Type: "string", Desc: "Container ID or name", Required: true},
				"command":   {Type: "string", Desc: "Command to execute", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				cmd := []string{"exec", args["container"]}
				cmd = append(cmd, strings.Fields(args["command"])...)
				return docker(ctx, cmd...)
			},
		},
	}
}
