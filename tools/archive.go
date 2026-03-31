package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/saeedalam/agnogo"
)

const defaultMaxArchiveSize = 50 * 1024 * 1024 // 50MB

// Archive returns tools for creating, extracting, and listing tar.gz archives.
func Archive() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "archive_create", Desc: "Create a tar.gz archive from files (base64 encoded output)",
			Params: agnogo.Params{
				"files": {Type: "string", Desc: "JSON array of {name, content} objects", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				filesStr := args["files"]
				if filesStr == "" {
					return "", fmt.Errorf("files is required")
				}
				var files []struct {
					Name    string `json:"name"`
					Content string `json:"content"`
				}
				if err := json.Unmarshal([]byte(filesStr), &files); err != nil {
					return "", fmt.Errorf("invalid files JSON: %w", err)
				}
				if len(files) == 0 {
					return "", fmt.Errorf("at least one file is required")
				}

				var buf bytes.Buffer
				gw := gzip.NewWriter(&buf)
				tw := tar.NewWriter(gw)

				for _, f := range files {
					if f.Name == "" {
						return "", fmt.Errorf("file name is required")
					}
					// Path traversal protection
					clean := filepath.Clean(f.Name)
					if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
						return "", fmt.Errorf("path traversal detected: %s", f.Name)
					}
					content := []byte(f.Content)
					hdr := &tar.Header{
						Name: clean,
						Mode: 0644,
						Size: int64(len(content)),
					}
					if err := tw.WriteHeader(hdr); err != nil {
						return "", fmt.Errorf("tar write header: %w", err)
					}
					if _, err := tw.Write(content); err != nil {
						return "", fmt.Errorf("tar write: %w", err)
					}
				}
				if err := tw.Close(); err != nil {
					return "", fmt.Errorf("tar close: %w", err)
				}
				if err := gw.Close(); err != nil {
					return "", fmt.Errorf("gzip close: %w", err)
				}

				encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
				result := map[string]any{"archive_base64": encoded, "size_bytes": buf.Len(), "file_count": len(files)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "archive_extract", Desc: "Extract files from a base64-encoded tar.gz archive",
			Params: agnogo.Params{
				"archive":  {Type: "string", Desc: "Base64-encoded tar.gz archive", Required: true},
				"max_size": {Type: "string", Desc: "Max archive size in bytes (default 50MB)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				archiveStr := args["archive"]
				if archiveStr == "" {
					return "", fmt.Errorf("archive is required")
				}
				maxSize := int64(defaultMaxArchiveSize)
				if args["max_size"] != "" {
					fmt.Sscanf(args["max_size"], "%d", &maxSize)
				}

				data, err := base64.StdEncoding.DecodeString(archiveStr)
				if err != nil {
					return "", fmt.Errorf("invalid base64: %w", err)
				}
				if int64(len(data)) > maxSize {
					return "", fmt.Errorf("archive exceeds max size of %d bytes", maxSize)
				}

				gr, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					return "", fmt.Errorf("gzip open: %w", err)
				}
				defer gr.Close()
				tr := tar.NewReader(gr)

				var files []map[string]string
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return "", fmt.Errorf("tar read: %w", err)
					}
					// Path traversal protection
					clean := filepath.Clean(hdr.Name)
					if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
						return "", fmt.Errorf("path traversal detected: %s", hdr.Name)
					}
					if hdr.Typeflag == tar.TypeDir {
						continue
					}
					content, err := io.ReadAll(io.LimitReader(tr, maxSize))
					if err != nil {
						return "", fmt.Errorf("read file %s: %w", hdr.Name, err)
					}
					files = append(files, map[string]string{
						"name":    clean,
						"content": string(content),
					})
				}
				result := map[string]any{"files": files, "file_count": len(files)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "archive_list", Desc: "List files in a base64-encoded tar.gz archive",
			Params: agnogo.Params{
				"archive": {Type: "string", Desc: "Base64-encoded tar.gz archive", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				archiveStr := args["archive"]
				if archiveStr == "" {
					return "", fmt.Errorf("archive is required")
				}
				data, err := base64.StdEncoding.DecodeString(archiveStr)
				if err != nil {
					return "", fmt.Errorf("invalid base64: %w", err)
				}
				gr, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					return "", fmt.Errorf("gzip open: %w", err)
				}
				defer gr.Close()
				tr := tar.NewReader(gr)

				var entries []map[string]any
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return "", fmt.Errorf("tar read: %w", err)
					}
					entries = append(entries, map[string]any{
						"name": hdr.Name,
						"size": hdr.Size,
						"mode": fmt.Sprintf("%o", hdr.Mode),
						"type": string([]byte{hdr.Typeflag}),
					})
				}
				result := map[string]any{"entries": entries, "count": len(entries)}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
