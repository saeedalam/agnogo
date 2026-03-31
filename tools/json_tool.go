package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// JSONConfig configures JSON tools.
type JSONConfig struct {
	// MaxInputSize is the maximum JSON string size in bytes. Default: 1MB.
	MaxInputSize int
}

func (c *JSONConfig) defaults() {
	if c.MaxInputSize <= 0 {
		c.MaxInputSize = 1 << 20 // 1 MB
	}
}

// JSON returns tools for parsing, validating, and transforming JSON.
func JSON(cfgs ...JSONConfig) []agnogo.ToolDef {
	var cfg JSONConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	return []agnogo.ToolDef{
		{
			Name: "json_parse",
			Desc: "Parse a JSON string and extract a field by path. Supports dot notation and array indexing (e.g. 'data.items[0].name').",
			Params: agnogo.Params{
				"json_str": {Type: "string", Desc: "JSON string to parse", Required: true},
				"path":     {Type: "string", Desc: "Path using dot notation + array indexing (e.g. 'user.addresses[0].city')"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				jsonStr := args["json_str"]
				if jsonStr == "" {
					return "", fmt.Errorf("missing required parameter: json_str")
				}
				if len(jsonStr) > cfg.MaxInputSize {
					return "", fmt.Errorf("JSON input too large: %d bytes (max %d)", len(jsonStr), cfg.MaxInputSize)
				}

				var data any
				dec := json.NewDecoder(strings.NewReader(jsonStr))
				dec.UseNumber()
				if err := dec.Decode(&data); err != nil {
					return "", fmt.Errorf("invalid JSON: %w", err)
				}
				if args["path"] == "" {
					pretty, _ := json.MarshalIndent(data, "", "  ")
					return string(pretty), nil
				}
				result := navigateJSON(data, args["path"])
				if result == nil {
					return "null", nil
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "json_format",
			Desc: "Pretty-print a JSON string",
			Params: agnogo.Params{
				"json_str": {Type: "string", Desc: "JSON string", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				jsonStr := args["json_str"]
				if jsonStr == "" {
					return "", fmt.Errorf("missing required parameter: json_str")
				}
				if len(jsonStr) > cfg.MaxInputSize {
					return "", fmt.Errorf("JSON input too large: %d bytes (max %d)", len(jsonStr), cfg.MaxInputSize)
				}
				var data any
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return "", fmt.Errorf("invalid JSON: %w", err)
				}
				pretty, _ := json.MarshalIndent(data, "", "  ")
				return string(pretty), nil
			},
		},
		{
			Name: "json_validate",
			Desc: "Validate that a string is well-formed JSON",
			Params: agnogo.Params{
				"json_str": {Type: "string", Desc: "JSON string to validate", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				jsonStr := args["json_str"]
				if jsonStr == "" {
					return "", fmt.Errorf("missing required parameter: json_str")
				}
				if json.Valid([]byte(jsonStr)) {
					return `{"valid":true}`, nil
				}
				// Try to get a more specific error message
				var data any
				err := json.Unmarshal([]byte(jsonStr), &data)
				errMsg := "unknown"
				if err != nil {
					errMsg = err.Error()
				}
				out, _ := json.Marshal(map[string]any{
					"valid": false,
					"error": errMsg,
				})
				return string(out), nil
			},
		},
		{
			Name: "json_merge",
			Desc: "Deep merge two JSON objects. Values from the second object override the first.",
			Params: agnogo.Params{
				"base":    {Type: "string", Desc: "Base JSON object", Required: true},
				"overlay": {Type: "string", Desc: "JSON object to merge on top", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				baseStr := args["base"]
				if baseStr == "" {
					return "", fmt.Errorf("missing required parameter: base")
				}
				overlayStr := args["overlay"]
				if overlayStr == "" {
					return "", fmt.Errorf("missing required parameter: overlay")
				}

				var base, overlay any
				if err := json.Unmarshal([]byte(baseStr), &base); err != nil {
					return "", fmt.Errorf("invalid base JSON: %w", err)
				}
				if err := json.Unmarshal([]byte(overlayStr), &overlay); err != nil {
					return "", fmt.Errorf("invalid overlay JSON: %w", err)
				}

				merged := deepMerge(base, overlay)
				out, _ := json.MarshalIndent(merged, "", "  ")
				return string(out), nil
			},
		},
	}
}

// navigateJSON traverses a JSON structure using a dot-notation path
// with array index support: "data.items[0].name".
func navigateJSON(data any, path string) any {
	parts := splitJSONPath(path)
	current := data
	for _, part := range parts {
		if current == nil {
			return nil
		}
		// Check for array index: part like "items[0]"
		name, idx, hasIdx := parseArrayIndex(part)

		if name != "" {
			switch v := current.(type) {
			case map[string]any:
				current = v[name]
			default:
				return nil
			}
		}

		if hasIdx {
			switch v := current.(type) {
			case []any:
				if idx < 0 || idx >= len(v) {
					return nil
				}
				current = v[idx]
			default:
				return nil
			}
		}
	}
	return current
}

// splitJSONPath splits a path like "data.items[0].name" into segments.
// Each segment is either a key name, or a key+index like "items[0]".
func splitJSONPath(s string) []string {
	var parts []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '.' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// parseArrayIndex parses "items[0]" into ("items", 0, true) or "name" into ("name", 0, false).
// Also handles bare "[0]" as ("", 0, true).
func parseArrayIndex(part string) (string, int, bool) {
	bracketStart := strings.IndexByte(part, '[')
	if bracketStart < 0 {
		return part, 0, false
	}
	bracketEnd := strings.IndexByte(part[bracketStart:], ']')
	if bracketEnd < 0 {
		return part, 0, false
	}
	bracketEnd += bracketStart
	name := part[:bracketStart]
	idxStr := part[bracketStart+1 : bracketEnd]
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return part, 0, false
	}
	return name, idx, true
}

// deepMerge recursively merges two values. Maps are merged key-by-key.
// For non-map types, overlay wins.
func deepMerge(base, overlay any) any {
	baseMap, baseOk := base.(map[string]any)
	overlayMap, overlayOk := overlay.(map[string]any)
	if baseOk && overlayOk {
		result := make(map[string]any, len(baseMap))
		for k, v := range baseMap {
			result[k] = v
		}
		for k, v := range overlayMap {
			if existing, ok := result[k]; ok {
				result[k] = deepMerge(existing, v)
			} else {
				result[k] = v
			}
		}
		return result
	}
	return overlay
}
