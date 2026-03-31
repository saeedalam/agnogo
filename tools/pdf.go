package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// PDFTool returns a tool for basic PDF metadata extraction.
func PDFTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "pdf_info", Desc: "Extract basic PDF metadata (page count, title, author)",
			Params: agnogo.Params{
				"path": {Type: "string", Desc: "Path to PDF file", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				path := args["path"]
				if path == "" {
					return "", fmt.Errorf("path is required")
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return "", fmt.Errorf("failed to read file: %w", err)
				}
				if len(data) < 5 || string(data[:5]) != "%PDF-" {
					return "", fmt.Errorf("not a valid PDF file")
				}

				info := map[string]any{
					"file_size": len(data),
				}

				// Extract PDF version
				if idx := strings.Index(string(data[:20]), "%PDF-"); idx >= 0 {
					end := strings.IndexByte(string(data[idx:idx+10]), '\n')
					if end < 0 {
						end = 10
					}
					info["pdf_version"] = strings.TrimSpace(string(data[idx+5 : idx+end]))
				}

				content := string(data)

				// Count pages by looking for /Type /Page (not /Pages)
				pageRe := regexp.MustCompile(`/Type\s*/Page[^s]`)
				pages := pageRe.FindAllStringIndex(content, -1)
				info["page_count"] = len(pages)

				// Try to find page count from /Pages object /Count
				countRe := regexp.MustCompile(`/Type\s*/Pages[^>]*?/Count\s+(\d+)`)
				if m := countRe.FindStringSubmatch(content); len(m) > 1 {
					if c, err := strconv.Atoi(m[1]); err == nil {
						info["page_count"] = c
					}
				}

				// Extract Info dictionary fields
				titleRe := regexp.MustCompile(`/Title\s*\(([^)]*)\)`)
				if m := titleRe.FindStringSubmatch(content); len(m) > 1 {
					info["title"] = m[1]
				}
				authorRe := regexp.MustCompile(`/Author\s*\(([^)]*)\)`)
				if m := authorRe.FindStringSubmatch(content); len(m) > 1 {
					info["author"] = m[1]
				}
				creatorRe := regexp.MustCompile(`/Creator\s*\(([^)]*)\)`)
				if m := creatorRe.FindStringSubmatch(content); len(m) > 1 {
					info["creator"] = m[1]
				}
				producerRe := regexp.MustCompile(`/Producer\s*\(([^)]*)\)`)
				if m := producerRe.FindStringSubmatch(content); len(m) > 1 {
					info["producer"] = m[1]
				}

				out, _ := json.Marshal(info)
				return string(out), nil
			},
		},
	}
}
