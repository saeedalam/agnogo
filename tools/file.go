package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/saeedalam/agnogo"
)

// File returns tools for reading and writing files.
// baseDir restricts operations to a directory (empty = current dir).
func File(baseDir string) []agnogo.ToolDef {
	if baseDir == "" {
		baseDir = "."
	}
	resolve := func(name string) string {
		return filepath.Join(baseDir, filepath.Clean(name))
	}
	return []agnogo.ToolDef{
		{
			Name: "read_file", Desc: "Read the contents of a file",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "File path (relative to base dir)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				data, err := os.ReadFile(resolve(args["path"]))
				if err != nil {
					return fmt.Sprintf("Error: %s", err), nil
				}
				if len(data) > 4096 {
					return string(data[:4096]) + "\n... (truncated)", nil
				}
				return string(data), nil
			},
		},
		{
			Name: "write_file", Desc: "Write content to a file",
			Params: agnogo.Params{
				"path":    {Type: "string", Desc: "File path", Required: true},
				"content": {Type: "string", Desc: "Content to write", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				path := resolve(args["path"])
				os.MkdirAll(filepath.Dir(path), 0o755)
				if err := os.WriteFile(path, []byte(args["content"]), 0o644); err != nil {
					return fmt.Sprintf("Error: %s", err), nil
				}
				return fmt.Sprintf("Written %d bytes to %s", len(args["content"]), args["path"]), nil
			},
		},
		{
			Name: "list_files", Desc: "List files in a directory",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Directory path"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				dir := resolve(args["path"])
				if dir == "" {
					dir = baseDir
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					return fmt.Sprintf("Error: %s", err), nil
				}
				var result string
				for _, e := range entries {
					prefix := "📄"
					if e.IsDir() {
						prefix = "📁"
					}
					result += fmt.Sprintf("%s %s\n", prefix, e.Name())
				}
				return result, nil
			},
		},
	}
}
