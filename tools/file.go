package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// FileConfig configures file tools.
type FileConfig struct {
	// MaxReadSize is the maximum number of bytes to read. Default: 1MB.
	MaxReadSize int64
}

func (c *FileConfig) defaults() {
	if c.MaxReadSize <= 0 {
		c.MaxReadSize = 1 << 20 // 1 MB
	}
}

// File returns tools for reading, writing, appending, listing, and inspecting files.
// baseDir restricts operations to a directory (empty = current dir).
func File(baseDir string, cfgs ...FileConfig) []agnogo.ToolDef {
	var cfg FileConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	if baseDir == "" {
		baseDir = "."
	}
	absBase, _ := filepath.Abs(baseDir)

	// resolve resolves a relative path, following symlinks, and ensures
	// the real path stays within baseDir.
	resolve := func(name string) (string, error) {
		if strings.TrimSpace(name) == "" {
			return "", fmt.Errorf("path must not be empty")
		}
		joined := filepath.Join(absBase, filepath.Clean("/"+name))
		abs, err := filepath.Abs(joined)
		if err != nil {
			return "", fmt.Errorf("invalid path: %s", name)
		}
		// Check the directory portion exists and resolve symlinks on it
		dir := filepath.Dir(abs)
		if realDir, err := filepath.EvalSymlinks(dir); err == nil {
			dir = realDir
		}
		realPath := filepath.Join(dir, filepath.Base(abs))
		// If the target itself exists, resolve its symlinks too
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			realPath = real
		}
		// Resolve symlinks on the base as well for comparison
		realBase := absBase
		if rb, err := filepath.EvalSymlinks(absBase); err == nil {
			realBase = rb
		}
		if !strings.HasPrefix(realPath, realBase+string(filepath.Separator)) && realPath != realBase {
			return "", fmt.Errorf("path %q escapes base directory", name)
		}
		return abs, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "read_file",
			Desc: "Read the contents of a file",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "File path (relative to base dir)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				path, err := resolve(args["path"])
				if err != nil {
					return "", err
				}
				info, err := os.Stat(path)
				if err != nil {
					return "", fmt.Errorf("cannot stat file: %w", err)
				}
				if info.IsDir() {
					return "", fmt.Errorf("%q is a directory, not a file", args["path"])
				}
				if info.Size() > cfg.MaxReadSize {
					return "", fmt.Errorf("file size %d bytes exceeds max read size %d bytes", info.Size(), cfg.MaxReadSize)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("read error: %w", err)
				}
				return string(data), nil
			},
		},
		{
			Name: "write_file",
			Desc: "Write content to a file (atomic: writes to temp file then renames)",
			Params: agnogo.Params{
				"path":    {Type: "string", Desc: "File path", Required: true},
				"content": {Type: "string", Desc: "Content to write", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				path, err := resolve(args["path"])
				if err != nil {
					return "", err
				}
				content := args["content"]

				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", fmt.Errorf("cannot create directory: %w", err)
				}

				// Atomic write: temp file + rename
				tmp, err := os.CreateTemp(dir, ".agnogo-write-*")
				if err != nil {
					return "", fmt.Errorf("cannot create temp file: %w", err)
				}
				tmpName := tmp.Name()
				defer os.Remove(tmpName) // clean up on failure

				if _, err := tmp.WriteString(content); err != nil {
					tmp.Close()
					return "", fmt.Errorf("write error: %w", err)
				}
				if err := tmp.Close(); err != nil {
					return "", fmt.Errorf("close error: %w", err)
				}
				if err := os.Chmod(tmpName, 0o644); err != nil {
					return "", fmt.Errorf("chmod error: %w", err)
				}
				if err := os.Rename(tmpName, path); err != nil {
					return "", fmt.Errorf("rename error: %w", err)
				}
				return fmt.Sprintf("Written %d bytes to %s", len(content), args["path"]), nil
			},
		},
		{
			Name: "file_append",
			Desc: "Append content to an existing file",
			Params: agnogo.Params{
				"path":    {Type: "string", Desc: "File path", Required: true},
				"content": {Type: "string", Desc: "Content to append", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				path, err := resolve(args["path"])
				if err != nil {
					return "", err
				}
				f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					return "", fmt.Errorf("cannot open file for append: %w", err)
				}
				defer f.Close()
				n, err := f.WriteString(args["content"])
				if err != nil {
					return "", fmt.Errorf("append error: %w", err)
				}
				return fmt.Sprintf("Appended %d bytes to %s", n, args["path"]), nil
			},
		},
		{
			Name: "file_info",
			Desc: "Get file info: size, permissions, modification time",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "File path", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				path, err := resolve(args["path"])
				if err != nil {
					return "", err
				}
				info, err := os.Stat(path)
				if err != nil {
					return "", fmt.Errorf("cannot stat: %w", err)
				}
				result := map[string]any{
					"name":        info.Name(),
					"size":        info.Size(),
					"permissions": info.Mode().String(),
					"is_dir":      info.IsDir(),
					"mod_time":    info.ModTime().Format(time.RFC3339),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "list_files",
			Desc: "List files in a directory",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Directory path"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				p := args["path"]
				if p == "" {
					p = "."
				}
				dir, err := resolve(p)
				if err != nil {
					return "", err
				}
				entries, err := os.ReadDir(dir)
				if err != nil {
					return "", fmt.Errorf("cannot read directory: %w", err)
				}
				var result string
				for _, e := range entries {
					if err := ctx.Err(); err != nil {
						return "", fmt.Errorf("context cancelled: %w", err)
					}
					prefix := "F"
					if e.IsDir() {
						prefix = "D"
					}
					result += fmt.Sprintf("%s %s\n", prefix, e.Name())
				}
				return result, nil
			},
		},
	}
}
