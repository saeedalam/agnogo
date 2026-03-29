package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/saeedalam/agnogo"
)

// JSON returns tools for parsing and transforming JSON.
func JSON() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "json_parse", Desc: "Parse a JSON string and extract a field by path (dot notation)",
			Params: agnogo.Params{
				"json_str": {Type: "string", Desc: "JSON string to parse", Required: true},
				"path":     {Type: "string", Desc: "Dot-separated path (e.g. 'user.name')"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				var data any
				if err := json.Unmarshal([]byte(args["json_str"]), &data); err != nil {
					return fmt.Sprintf("Invalid JSON: %s", err), nil
				}
				if args["path"] == "" {
					pretty, _ := json.MarshalIndent(data, "", "  ")
					return string(pretty), nil
				}
				// Navigate dot path
				result := navigateJSON(data, args["path"])
				if result == nil {
					return "null", nil
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "json_format", Desc: "Pretty-print a JSON string",
			Params: agnogo.Params{
				"json_str": {Type: "string", Desc: "JSON string", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				var data any
				if err := json.Unmarshal([]byte(args["json_str"]), &data); err != nil {
					return fmt.Sprintf("Invalid JSON: %s", err), nil
				}
				pretty, _ := json.MarshalIndent(data, "", "  ")
				return string(pretty), nil
			},
		},
	}
}

func navigateJSON(data any, path string) any {
	parts := splitDot(path)
	current := data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			return nil
		}
	}
	return current
}

func splitDot(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '.' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
