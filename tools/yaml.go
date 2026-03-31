package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// YAML returns tools for converting between YAML and JSON.
// This is a basic implementation covering the common YAML subset.
func YAML() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "yaml_to_json", Desc: "Convert YAML to JSON (supports maps, lists, scalars)",
			Params: agnogo.Params{
				"yaml": {Type: "string", Desc: "YAML string to convert", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				yamlStr := args["yaml"]
				if yamlStr == "" {
					return "", fmt.Errorf("yaml is required")
				}
				result, err := parseYAML(yamlStr)
				if err != nil {
					return "", fmt.Errorf("YAML parse error: %w", err)
				}
				out, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return "", fmt.Errorf("JSON marshal error: %w", err)
				}
				return string(out), nil
			},
		},
		{
			Name: "json_to_yaml", Desc: "Convert JSON to YAML format",
			Params: agnogo.Params{
				"json": {Type: "string", Desc: "JSON string to convert", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				jsonStr := args["json"]
				if jsonStr == "" {
					return "", fmt.Errorf("json is required")
				}
				var data any
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return "", fmt.Errorf("invalid JSON: %w", err)
				}
				return toYAML(data, 0), nil
			},
		},
	}
}

func parseYAML(input string) (any, error) {
	lines := strings.Split(input, "\n")
	var filtered []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		filtered = append(filtered, l)
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	result, _, err := parseYAMLLines(filtered, 0, 0)
	return result, err
}

func parseYAMLLines(lines []string, start, baseIndent int) (any, int, error) {
	if start >= len(lines) {
		return nil, start, nil
	}

	first := lines[start]
	trimmed := strings.TrimSpace(first)

	// Check if it's a list
	if strings.HasPrefix(trimmed, "- ") {
		return parseYAMLList(lines, start, baseIndent)
	}

	// Otherwise it's a map
	return parseYAMLMap(lines, start, baseIndent)
}

func lineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func parseYAMLMap(lines []string, start, baseIndent int) (map[string]any, int, error) {
	m := map[string]any{}
	i := start
	for i < len(lines) {
		indent := lineIndent(lines[i])
		if indent < baseIndent {
			break
		}
		if indent > baseIndent {
			break
		}
		trimmed := strings.TrimSpace(lines[i])
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			i++
			continue
		}
		key := strings.TrimSpace(trimmed[:colonIdx])
		rest := strings.TrimSpace(trimmed[colonIdx+1:])

		if rest != "" {
			m[key] = parseScalar(rest)
			i++
		} else {
			// Value is on next lines with deeper indent
			if i+1 < len(lines) {
				nextIndent := lineIndent(lines[i+1])
				if nextIndent > baseIndent {
					val, newI, err := parseYAMLLines(lines, i+1, nextIndent)
					if err != nil {
						return nil, i, err
					}
					m[key] = val
					i = newI
				} else {
					m[key] = nil
					i++
				}
			} else {
				m[key] = nil
				i++
			}
		}
	}
	return m, i, nil
}

func parseYAMLList(lines []string, start, baseIndent int) ([]any, int, error) {
	var list []any
	i := start
	for i < len(lines) {
		indent := lineIndent(lines[i])
		if indent < baseIndent {
			break
		}
		if indent > baseIndent {
			break
		}
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "- ") {
			break
		}
		content := strings.TrimPrefix(trimmed, "- ")
		// Check if this list item is a map
		if strings.Contains(content, ": ") || strings.HasSuffix(content, ":") {
			// Inline map-like value or nested structure
			if i+1 < len(lines) && lineIndent(lines[i+1]) > baseIndent {
				// Multi-line map item: parse the first key plus children
				fakeLine := strings.Repeat(" ", baseIndent+2) + content
				tempLines := append([]string{fakeLine}, lines[i+1:]...)
				val, consumed, err := parseYAMLMap(tempLines, 0, baseIndent+2)
				if err != nil {
					return nil, i, err
				}
				list = append(list, val)
				i += consumed // consumed lines from tempLines minus the fake line
			} else {
				colonIdx := strings.Index(content, ":")
				if colonIdx >= 0 {
					key := strings.TrimSpace(content[:colonIdx])
					rest := strings.TrimSpace(content[colonIdx+1:])
					if rest != "" {
						list = append(list, map[string]any{key: parseScalar(rest)})
					} else {
						list = append(list, map[string]any{key: nil})
					}
				} else {
					list = append(list, parseScalar(content))
				}
				i++
			}
		} else {
			list = append(list, parseScalar(content))
			i++
		}
	}
	return list, i, nil
}

func parseScalar(s string) any {
	s = strings.TrimSpace(s)
	if s == "null" || s == "~" {
		return nil
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// Quoted string
	if (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}
	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func toYAML(data any, indent int) string {
	prefix := strings.Repeat("  ", indent)
	switch v := data.(type) {
	case map[string]any:
		if len(v) == 0 {
			return prefix + "{}\n"
		}
		var sb strings.Builder
		for key, val := range v {
			switch child := val.(type) {
			case map[string]any, []any:
				sb.WriteString(prefix + key + ":\n")
				sb.WriteString(toYAML(child, indent+1))
			default:
				sb.WriteString(prefix + key + ": " + scalarToYAML(val) + "\n")
			}
		}
		return sb.String()
	case []any:
		if len(v) == 0 {
			return prefix + "[]\n"
		}
		var sb strings.Builder
		for _, item := range v {
			switch child := item.(type) {
			case map[string]any, []any:
				sb.WriteString(prefix + "-\n")
				sb.WriteString(toYAML(child, indent+1))
			default:
				sb.WriteString(prefix + "- " + scalarToYAML(child) + "\n")
			}
		}
		return sb.String()
	default:
		return scalarToYAML(v) + "\n"
	}
}

func scalarToYAML(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	case string:
		if val == "" || val == "true" || val == "false" || val == "null" {
			return `"` + val + `"`
		}
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}
