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
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

const (
	defaultMaxArchiveFiles = 10000
	defaultMaxArchiveSize  = 500 * 1024 * 1024 // 500MB
	defaultMaxInputSize    = 50 * 1024 * 1024   // 50MB for base64 input
)

// Archive returns tools for creating, extracting, listing, and inspecting
// tar.gz archives, plus raw gzip compress/decompress. Includes safety limits
// for file count and total extracted size to prevent zip bombs.
func Archive() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		archiveCreate(),
		archiveExtract(),
		archiveList(),
		archiveInfo(),
		archiveCompress(),
		archiveDecompress(),
	}
}

func archiveCreate() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_create",
		Desc: "Create a tar.gz archive from files. Returns base64-encoded archive with file count and size.",
		Params: agnogo.Params{
			"files":          {Type: "string", Desc: "JSON array of {name, content} objects", Required: true},
			"follow_symlink": {Type: "string", Desc: "Follow symlinks instead of skipping (true/false, default false)"},
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
			if len(files) > defaultMaxArchiveFiles {
				return "", fmt.Errorf("file count %d exceeds limit of %d", len(files), defaultMaxArchiveFiles)
			}

			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)

			var totalSize int64
			for _, f := range files {
				if f.Name == "" {
					return "", fmt.Errorf("file name is required for each entry")
				}
				clean := filepath.Clean(f.Name)
				if err := validateArchivePath(clean); err != nil {
					return "", fmt.Errorf("invalid path %q: %w", f.Name, err)
				}
				content := []byte(f.Content)
				totalSize += int64(len(content))
				if totalSize > defaultMaxArchiveSize {
					return "", fmt.Errorf("total content size exceeds limit of %d bytes", defaultMaxArchiveSize)
				}

				hdr := &tar.Header{
					Name: clean,
					Mode: 0644,
					Size: int64(len(content)),
				}
				if err := tw.WriteHeader(hdr); err != nil {
					return "", fmt.Errorf("failed to write tar header for %q: %w", clean, err)
				}
				if _, err := tw.Write(content); err != nil {
					return "", fmt.Errorf("failed to write tar data for %q: %w", clean, err)
				}
			}

			if err := tw.Close(); err != nil {
				return "", fmt.Errorf("failed to finalize tar: %w", err)
			}
			if err := gw.Close(); err != nil {
				return "", fmt.Errorf("failed to finalize gzip: %w", err)
			}

			encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
			result := map[string]any{
				"archive_base64": encoded,
				"size_bytes":     buf.Len(),
				"file_count":     len(files),
				"total_content":  totalSize,
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		},
	}
}

func archiveExtract() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_extract",
		Desc: "Extract files from a base64-encoded tar.gz archive. Returns file contents with safety limits on count and total size.",
		Params: agnogo.Params{
			"archive":        {Type: "string", Desc: "Base64-encoded tar.gz archive", Required: true},
			"max_files":      {Type: "string", Desc: fmt.Sprintf("Max files to extract (default %d)", defaultMaxArchiveFiles)},
			"max_total_size": {Type: "string", Desc: fmt.Sprintf("Max total extracted bytes (default %d)", defaultMaxArchiveSize)},
			"follow_symlink": {Type: "string", Desc: "Follow symlinks (true/false, default false)"},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			archiveStr := args["archive"]
			if archiveStr == "" {
				return "", fmt.Errorf("archive is required")
			}

			maxFiles := defaultMaxArchiveFiles
			if v := args["max_files"]; v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					maxFiles = n
				}
			}
			maxTotal := int64(defaultMaxArchiveSize)
			if v := args["max_total_size"]; v != "" {
				if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
					maxTotal = n
				}
			}
			followSymlinks := args["follow_symlink"] == "true"

			data, err := base64.StdEncoding.DecodeString(archiveStr)
			if err != nil {
				return "", fmt.Errorf("invalid base64: %w", err)
			}
			if int64(len(data)) > int64(defaultMaxInputSize) {
				return "", fmt.Errorf("archive input exceeds max size of %d bytes", defaultMaxInputSize)
			}

			gr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return "", fmt.Errorf("failed to open gzip: %w", err)
			}
			defer gr.Close()
			tr := tar.NewReader(gr)

			var files []map[string]any
			var totalExtracted int64
			fileCount := 0

			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return "", fmt.Errorf("tar read error: %w", err)
				}

				clean := filepath.Clean(hdr.Name)
				if err := validateArchivePath(clean); err != nil {
					return "", fmt.Errorf("unsafe path in archive %q: %w", hdr.Name, err)
				}

				// Handle symlinks
				if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
					if !followSymlinks {
						continue
					}
				}
				if hdr.Typeflag == tar.TypeDir {
					continue
				}

				fileCount++
				if fileCount > maxFiles {
					return "", fmt.Errorf("archive contains more than %d files (limit reached)", maxFiles)
				}

				remaining := maxTotal - totalExtracted
				if remaining <= 0 {
					return "", fmt.Errorf("extracted data exceeds limit of %d bytes", maxTotal)
				}
				content, err := io.ReadAll(io.LimitReader(tr, remaining+1))
				if err != nil {
					return "", fmt.Errorf("failed to read %q from archive: %w", clean, err)
				}
				if int64(len(content)) > remaining {
					return "", fmt.Errorf("extracted data exceeds limit of %d bytes (at file %q)", maxTotal, clean)
				}
				totalExtracted += int64(len(content))

				files = append(files, map[string]any{
					"name":    clean,
					"content": string(content),
					"size":    len(content),
				})
			}

			result := map[string]any{
				"files":           files,
				"file_count":      len(files),
				"total_extracted": totalExtracted,
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		},
	}
}

func archiveList() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_list",
		Desc: "List files in a base64-encoded tar.gz archive without extracting contents.",
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
				return "", fmt.Errorf("failed to open gzip: %w", err)
			}
			defer gr.Close()
			tr := tar.NewReader(gr)

			var entries []map[string]any
			var totalSize int64
			count := 0

			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return "", fmt.Errorf("tar read error: %w", err)
				}

				count++
				if count > defaultMaxArchiveFiles {
					return "", fmt.Errorf("archive listing exceeded %d entries", defaultMaxArchiveFiles)
				}

				totalSize += hdr.Size
				entryType := "file"
				switch hdr.Typeflag {
				case tar.TypeDir:
					entryType = "directory"
				case tar.TypeSymlink:
					entryType = "symlink"
				case tar.TypeLink:
					entryType = "hardlink"
				}

				entry := map[string]any{
					"name": hdr.Name,
					"size": hdr.Size,
					"mode": fmt.Sprintf("%o", hdr.Mode),
					"type": entryType,
				}
				if hdr.Typeflag == tar.TypeSymlink {
					entry["link_target"] = hdr.Linkname
				}
				if !hdr.ModTime.IsZero() {
					entry["modified"] = hdr.ModTime.Format("2006-01-02T15:04:05Z")
				}
				entries = append(entries, entry)
			}

			result := map[string]any{
				"entries":    entries,
				"count":     len(entries),
				"total_size": totalSize,
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		},
	}
}

func archiveInfo() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_info",
		Desc: "Get summary information about a tar.gz archive: file count, total size, largest file, smallest file.",
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
				return "", fmt.Errorf("failed to open gzip: %w", err)
			}
			defer gr.Close()
			tr := tar.NewReader(gr)

			var totalSize int64
			fileCount := 0
			dirCount := 0
			var largest, smallest string
			var largestSize, smallestSize int64
			smallestSize = -1

			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return "", fmt.Errorf("tar read error: %w", err)
				}

				if hdr.Typeflag == tar.TypeDir {
					dirCount++
					continue
				}

				fileCount++
				totalSize += hdr.Size

				if hdr.Size > largestSize {
					largestSize = hdr.Size
					largest = hdr.Name
				}
				if smallestSize < 0 || hdr.Size < smallestSize {
					smallestSize = hdr.Size
					smallest = hdr.Name
				}
			}

			info := map[string]any{
				"compressed_size": len(data),
				"file_count":     fileCount,
				"dir_count":      dirCount,
				"total_size":     totalSize,
			}
			if largest != "" {
				info["largest_file"] = map[string]any{"name": largest, "size": largestSize}
			}
			if smallest != "" {
				info["smallest_file"] = map[string]any{"name": smallest, "size": smallestSize}
			}

			out, _ := json.MarshalIndent(info, "", "  ")
			return string(out), nil
		},
	}
}

func archiveCompress() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_compress",
		Desc: "Compress raw data with gzip. Input and output are base64-encoded.",
		Params: agnogo.Params{
			"data": {Type: "string", Desc: "Base64-encoded data to compress", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			dataStr := args["data"]
			if dataStr == "" {
				return "", fmt.Errorf("data is required")
			}
			data, err := base64.StdEncoding.DecodeString(dataStr)
			if err != nil {
				return "", fmt.Errorf("invalid base64: %w", err)
			}
			if int64(len(data)) > defaultMaxArchiveSize {
				return "", fmt.Errorf("data exceeds max size of %d bytes", defaultMaxArchiveSize)
			}

			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			if _, err := gw.Write(data); err != nil {
				return "", fmt.Errorf("gzip write error: %w", err)
			}
			if err := gw.Close(); err != nil {
				return "", fmt.Errorf("gzip close error: %w", err)
			}

			result := map[string]any{
				"compressed_base64": base64.StdEncoding.EncodeToString(buf.Bytes()),
				"original_size":    len(data),
				"compressed_size":  buf.Len(),
				"ratio":            fmt.Sprintf("%.1f%%", float64(buf.Len())/float64(len(data))*100),
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		},
	}
}

func archiveDecompress() agnogo.ToolDef {
	return agnogo.ToolDef{
		Name: "archive_decompress",
		Desc: "Decompress gzip data. Input and output are base64-encoded.",
		Params: agnogo.Params{
			"data":           {Type: "string", Desc: "Base64-encoded gzip data to decompress", Required: true},
			"max_total_size": {Type: "string", Desc: fmt.Sprintf("Max decompressed size in bytes (default %d)", defaultMaxArchiveSize)},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			dataStr := args["data"]
			if dataStr == "" {
				return "", fmt.Errorf("data is required")
			}
			maxSize := int64(defaultMaxArchiveSize)
			if v := args["max_total_size"]; v != "" {
				if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
					maxSize = n
				}
			}

			data, err := base64.StdEncoding.DecodeString(dataStr)
			if err != nil {
				return "", fmt.Errorf("invalid base64: %w", err)
			}

			gr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return "", fmt.Errorf("failed to open gzip data: %w", err)
			}
			defer gr.Close()

			decompressed, err := io.ReadAll(io.LimitReader(gr, maxSize+1))
			if err != nil {
				return "", fmt.Errorf("gzip decompress error: %w", err)
			}
			if int64(len(decompressed)) > maxSize {
				return "", fmt.Errorf("decompressed data exceeds limit of %d bytes", maxSize)
			}

			result := map[string]any{
				"decompressed_base64": base64.StdEncoding.EncodeToString(decompressed),
				"compressed_size":     len(data),
				"decompressed_size":   len(decompressed),
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateArchivePath(clean string) error {
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return fmt.Errorf("path traversal detected")
	}
	// Check each component
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("path traversal detected")
		}
	}
	return nil
}
