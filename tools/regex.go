package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/saeedalam/agnogo"
)

const (
	maxTextLen    = 100 * 1024 // 100KB
	maxPatternLen = 1024       // 1KB
)

// Regex returns tools for regex matching, replacing, and extracting.
func Regex() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "regex_match", Desc: "Test if a regex pattern matches text and return all matches",
			Params: agnogo.Params{
				"pattern": {Type: "string", Desc: "Regular expression pattern", Required: true},
				"text":    {Type: "string", Desc: "Text to match against", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				pattern := args["pattern"]
				text := args["text"]
				if len(text) > maxTextLen {
					return "", fmt.Errorf("text exceeds %d byte limit", maxTextLen)
				}
				if len(pattern) > maxPatternLen {
					return "", fmt.Errorf("pattern exceeds %d byte limit", maxPatternLen)
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					return "", fmt.Errorf("invalid regex: %w", err)
				}
				matches := re.FindAllString(text, -1)
				matched := len(matches) > 0
				result := map[string]any{"matched": matched, "matches": matches}
				if matches == nil {
					result["matches"] = []string{}
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "regex_replace", Desc: "Replace all regex matches in text with a replacement string",
			Params: agnogo.Params{
				"pattern":     {Type: "string", Desc: "Regular expression pattern", Required: true},
				"text":        {Type: "string", Desc: "Text to perform replacement on", Required: true},
				"replacement": {Type: "string", Desc: "Replacement string (supports $1, $2 for groups)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				pattern := args["pattern"]
				text := args["text"]
				replacement := args["replacement"]
				if len(text) > maxTextLen {
					return "", fmt.Errorf("text exceeds %d byte limit", maxTextLen)
				}
				if len(pattern) > maxPatternLen {
					return "", fmt.Errorf("pattern exceeds %d byte limit", maxPatternLen)
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					return "", fmt.Errorf("invalid regex: %w", err)
				}
				return re.ReplaceAllString(text, replacement), nil
			},
		},
		{
			Name: "regex_extract", Desc: "Extract named group captures from text using a regex pattern",
			Params: agnogo.Params{
				"pattern": {Type: "string", Desc: "Regular expression with named groups (?P<name>...)", Required: true},
				"text":    {Type: "string", Desc: "Text to extract from", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				pattern := args["pattern"]
				text := args["text"]
				if len(text) > maxTextLen {
					return "", fmt.Errorf("text exceeds %d byte limit", maxTextLen)
				}
				if len(pattern) > maxPatternLen {
					return "", fmt.Errorf("pattern exceeds %d byte limit", maxPatternLen)
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					return "", fmt.Errorf("invalid regex: %w", err)
				}
				names := re.SubexpNames()
				allMatches := re.FindAllStringSubmatch(text, -1)
				var results []map[string]string
				for _, match := range allMatches {
					groups := map[string]string{}
					for i, name := range names {
						if i == 0 || name == "" {
							continue
						}
						if i < len(match) {
							groups[name] = match[i]
						}
					}
					if len(groups) > 0 {
						results = append(results, groups)
					}
				}
				if results == nil {
					results = []map[string]string{}
				}
				out, _ := json.Marshal(results)
				return string(out), nil
			},
		},
	}
}
