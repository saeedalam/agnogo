package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/saeedalam/agnogo"
)

// YAML returns tools for converting between YAML and JSON.
// Supports: nested maps/lists, quoted strings, block scalars (| and >),
// anchors (&) and aliases (*), flow sequences/mappings, comments, and
// standard scalar types (null, bool, int, float).
func YAML() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "yaml_to_json", Desc: "Convert YAML to JSON. Supports nested maps, lists, quoted strings, block scalars, anchors/aliases, flow syntax, and all scalar types.",
			Params: agnogo.Params{
				"yaml": {Type: "string", Desc: "YAML string to convert", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				yamlStr := args["yaml"]
				if yamlStr == "" {
					return "", fmt.Errorf("yaml is required")
				}
				p := &yamlParser{
					anchors: map[string]any{},
				}
				result, err := p.parse(yamlStr)
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
			Name: "json_to_yaml", Desc: "Convert JSON to YAML format with proper indentation and quoting.",
			Params: agnogo.Params{
				"json": {Type: "string", Desc: "JSON string to convert", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				jsonStr := args["json"]
				if jsonStr == "" {
					return "", fmt.Errorf("json is required")
				}
				dec := json.NewDecoder(strings.NewReader(jsonStr))
				dec.UseNumber()
				var data any
				if err := dec.Decode(&data); err != nil {
					return "", fmt.Errorf("invalid JSON: %w", err)
				}
				return toYAML(data, 0), nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// YAML parser
// ---------------------------------------------------------------------------

type yamlLine struct {
	indent int
	raw    string // original line
	text   string // trimmed content (no leading spaces)
	num    int    // 1-based line number
}

type yamlParser struct {
	lines   []yamlLine
	pos     int
	anchors map[string]any
}

func (p *yamlParser) parse(input string) (any, error) {
	raw := strings.Split(input, "\n")
	for i, r := range raw {
		stripped := stripComment(r)
		trimmed := strings.TrimRight(stripped, " \t\r")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		indent := countIndent(trimmed)
		p.lines = append(p.lines, yamlLine{
			indent: indent,
			raw:    trimmed,
			text:   strings.TrimSpace(trimmed),
			num:    i + 1,
		})
	}
	if len(p.lines) == 0 {
		return nil, nil
	}
	p.pos = 0

	// Handle document start marker
	if p.pos < len(p.lines) && p.lines[p.pos].text == "---" {
		p.pos++
	}
	if p.pos >= len(p.lines) {
		return nil, nil
	}

	result, err := p.parseValue(p.lines[p.pos].indent)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// stripComment removes inline comments, respecting quoted strings.
func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if ch == '#' && !inSingle && !inDouble {
			// Must be preceded by whitespace (or be at start)
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		}
	}
	return line
}

func countIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func (p *yamlParser) peek() *yamlLine {
	if p.pos >= len(p.lines) {
		return nil
	}
	return &p.lines[p.pos]
}

func (p *yamlParser) parseValue(baseIndent int) (any, error) {
	line := p.peek()
	if line == nil {
		return nil, nil
	}

	text := line.text

	// Flow sequence
	if strings.HasPrefix(text, "[") {
		return p.parseFlowSequence(text)
	}
	// Flow mapping
	if strings.HasPrefix(text, "{") {
		return p.parseFlowMapping(text)
	}
	// List item
	if strings.HasPrefix(text, "- ") || text == "-" {
		return p.parseList(baseIndent)
	}
	// Map (key: value)
	if isMapLine(text) {
		return p.parseMap(baseIndent)
	}
	// Scalar
	p.pos++
	return p.resolveScalar(text)
}

func isMapLine(text string) bool {
	// A line is a map entry if it contains an unquoted colon followed by space or EOL.
	inSingle := false
	inDouble := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if ch == ':' && !inSingle && !inDouble {
			if i+1 == len(text) || text[i+1] == ' ' {
				return true
			}
		}
	}
	return false
}

func (p *yamlParser) parseMap(baseIndent int) (map[string]any, error) {
	m := map[string]any{}
	for {
		line := p.peek()
		if line == nil || line.indent < baseIndent {
			break
		}
		if line.indent > baseIndent {
			break
		}
		text := line.text

		// Skip non-map lines at this indent (shouldn't happen in well-formed YAML)
		if !isMapLine(text) {
			break
		}

		key, rest := splitMapEntry(text)
		key = unquoteScalar(key)
		anchor, alias, val := extractAnchorAlias(rest)

		if alias != "" {
			// *alias reference
			p.pos++
			resolved, ok := p.anchors[alias]
			if !ok {
				return nil, fmt.Errorf("line %d: undefined alias *%s", line.num, alias)
			}
			m[key] = resolved
			if anchor != "" {
				p.anchors[anchor] = resolved
			}
			continue
		}

		p.pos++

		var value any
		var err error

		if val != "" {
			// Inline value
			if strings.HasPrefix(val, "|") || strings.HasPrefix(val, ">") {
				value, err = p.parseBlockScalar(val, baseIndent)
			} else if strings.HasPrefix(val, "[") {
				value, err = p.parseFlowSequence(val)
			} else if strings.HasPrefix(val, "{") {
				value, err = p.parseFlowMapping(val)
			} else {
				value, err = p.resolveScalar(val)
			}
		} else {
			// Value on next lines
			next := p.peek()
			if next != nil && next.indent > baseIndent {
				value, err = p.parseValue(next.indent)
			} else {
				value = nil
			}
		}
		if err != nil {
			return nil, err
		}

		if anchor != "" {
			p.anchors[anchor] = value
		}
		m[key] = value
	}
	return m, nil
}

func (p *yamlParser) parseList(baseIndent int) ([]any, error) {
	var list []any
	for {
		line := p.peek()
		if line == nil || line.indent < baseIndent {
			break
		}
		if line.indent > baseIndent {
			break
		}
		text := line.text
		if !strings.HasPrefix(text, "- ") && text != "-" {
			break
		}

		content := ""
		if text != "-" {
			content = text[2:]
		}
		p.pos++

		if content == "" {
			// Value on subsequent indented lines
			next := p.peek()
			if next != nil && next.indent > baseIndent {
				val, err := p.parseValue(next.indent)
				if err != nil {
					return nil, err
				}
				list = append(list, val)
			} else {
				list = append(list, nil)
			}
		} else if strings.HasPrefix(content, "[") {
			val, err := p.parseFlowSequence(content)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		} else if strings.HasPrefix(content, "{") {
			val, err := p.parseFlowMapping(content)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		} else if isMapLine(content) {
			// Inline map on list item line, possibly with continuation
			// Re-insert as a virtual line so the map parser sees it
			p.pos-- // back up
			// The list item content needs to be parsed as a block at indent+2
			childIndent := baseIndent + 2
			var virtualLines []yamlLine
			virtualLines = append(virtualLines, yamlLine{
				indent: childIndent,
				text:   content,
				num:    line.num,
			})
			// Collect child lines
			for p.pos+1 < len(p.lines) && p.lines[p.pos+1].indent > baseIndent {
				p.pos++
				virtualLines = append(virtualLines, p.lines[p.pos])
			}
			p.pos++ // skip past the original "- ..." line

			subParser := &yamlParser{
				lines:   virtualLines,
				anchors: p.anchors,
			}
			val, err := subParser.parseMap(childIndent)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		} else {
			val, err := p.resolveScalar(content)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		}
	}
	return list, nil
}

func (p *yamlParser) parseBlockScalar(header string, parentIndent int) (any, error) {
	chomp := "clip"
	fold := strings.HasPrefix(header, ">")
	h := strings.TrimSpace(header[1:])
	if strings.Contains(h, "-") {
		chomp = "strip"
	} else if strings.Contains(h, "+") {
		chomp = "keep"
	}

	var contentLines []string
	scalarIndent := -1
	for {
		line := p.peek()
		if line == nil {
			break
		}
		if line.indent <= parentIndent {
			break
		}
		if scalarIndent < 0 {
			scalarIndent = line.indent
		}
		// Include the raw line content preserving relative indentation
		if line.indent >= scalarIndent {
			prefix := strings.Repeat(" ", line.indent-scalarIndent)
			contentLines = append(contentLines, prefix+line.text)
		}
		p.pos++
	}

	if len(contentLines) == 0 {
		return "", nil
	}

	var result string
	if fold {
		// Folded: replace single newlines with spaces, preserve double newlines
		var parts []string
		current := contentLines[0]
		for i := 1; i < len(contentLines); i++ {
			if contentLines[i] == "" {
				parts = append(parts, current, "")
				current = ""
			} else if current == "" {
				current = contentLines[i]
			} else {
				current += " " + contentLines[i]
			}
		}
		parts = append(parts, current)
		result = strings.Join(parts, "\n")
	} else {
		// Literal: preserve newlines
		result = strings.Join(contentLines, "\n")
	}

	switch chomp {
	case "strip":
		result = strings.TrimRight(result, "\n")
	case "keep":
		result += "\n"
	default: // clip
		result = strings.TrimRight(result, "\n") + "\n"
	}

	return result, nil
}

// splitMapEntry splits "key: value" into key and value parts, respecting quotes.
func splitMapEntry(text string) (string, string) {
	inSingle := false
	inDouble := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if ch == ':' && !inSingle && !inDouble {
			if i+1 == len(text) {
				return strings.TrimSpace(text[:i]), ""
			}
			if text[i+1] == ' ' {
				return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+2:])
			}
		}
	}
	return text, ""
}

func extractAnchorAlias(val string) (anchor, alias, rest string) {
	val = strings.TrimSpace(val)
	if val == "" {
		return "", "", ""
	}
	// Check for alias: *name
	if strings.HasPrefix(val, "*") {
		parts := strings.SplitN(val[1:], " ", 2)
		alias = parts[0]
		if len(parts) > 1 {
			rest = strings.TrimSpace(parts[1])
		}
		return "", alias, rest
	}
	// Check for anchor: &name value
	if strings.HasPrefix(val, "&") {
		parts := strings.SplitN(val[1:], " ", 2)
		anchor = parts[0]
		if len(parts) > 1 {
			rest = strings.TrimSpace(parts[1])
		}
		return anchor, "", rest
	}
	return "", "", val
}

func (p *yamlParser) resolveScalar(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	// Check alias
	if strings.HasPrefix(s, "*") {
		name := s[1:]
		val, ok := p.anchors[name]
		if !ok {
			return nil, fmt.Errorf("undefined alias *%s", name)
		}
		return val, nil
	}
	// Check anchor on scalar
	if strings.HasPrefix(s, "&") {
		parts := strings.SplitN(s[1:], " ", 2)
		anchor := parts[0]
		rest := ""
		if len(parts) > 1 {
			rest = parts[1]
		}
		val := parseScalar(rest)
		p.anchors[anchor] = val
		return val, nil
	}
	return parseScalar(s), nil
}

// parseFlowSequence parses [a, b, c] style inline sequences.
func (p *yamlParser) parseFlowSequence(s string) ([]any, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid flow sequence: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return []any{}, nil
	}
	parts := splitFlowItems(inner)
	var result []any
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "{") {
			sub := &yamlParser{anchors: p.anchors}
			val, err := sub.parseFlowMapping(part)
			if err != nil {
				return nil, err
			}
			result = append(result, val)
		} else if strings.HasPrefix(part, "[") {
			sub := &yamlParser{anchors: p.anchors}
			val, err := sub.parseFlowSequence(part)
			if err != nil {
				return nil, err
			}
			result = append(result, val)
		} else {
			result = append(result, parseScalar(part))
		}
	}
	return result, nil
}

// parseFlowMapping parses {a: b, c: d} style inline mappings.
func (p *yamlParser) parseFlowMapping(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("invalid flow mapping: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return map[string]any{}, nil
	}
	parts := splitFlowItems(inner)
	m := map[string]any{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		colonIdx := strings.Index(part, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("invalid flow mapping entry: %s", part)
		}
		key := unquoteScalar(strings.TrimSpace(part[:colonIdx]))
		val := strings.TrimSpace(part[colonIdx+1:])
		if strings.HasPrefix(val, "{") {
			sub := &yamlParser{anchors: p.anchors}
			v, err := sub.parseFlowMapping(val)
			if err != nil {
				return nil, err
			}
			m[key] = v
		} else if strings.HasPrefix(val, "[") {
			sub := &yamlParser{anchors: p.anchors}
			v, err := sub.parseFlowSequence(val)
			if err != nil {
				return nil, err
			}
			m[key] = v
		} else {
			m[key] = parseScalar(val)
		}
	}
	return m, nil
}

// splitFlowItems splits comma-separated items respecting nesting.
func splitFlowItems(s string) []string {
	var items []string
	depth := 0
	start := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		} else if !inSingle && !inDouble {
			if ch == '[' || ch == '{' {
				depth++
			} else if ch == ']' || ch == '}' {
				depth--
			} else if ch == ',' && depth == 0 {
				items = append(items, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		items = append(items, s[start:])
	}
	return items
}

func unquoteScalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseScalar(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Null
	if s == "null" || s == "~" || s == "Null" || s == "NULL" {
		return nil
	}
	// Boolean
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}
	// Special floats
	switch s {
	case ".inf", ".Inf", ".INF":
		return math.Inf(1)
	case "-.inf", "-.Inf", "-.INF":
		return math.Inf(-1)
	case ".nan", ".NaN", ".NAN":
		return math.NaN()
	}
	// Quoted string
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return unescapeDoubleQuoted(s[1 : len(s)-1])
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			// In single-quoted YAML, '' is an escaped single quote
			return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
		}
	}
	// Integer (decimal, octal 0o, hex 0x)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if i, err := strconv.ParseInt(s[2:], 16, 64); err == nil {
			return i
		}
	}
	if strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O") {
		if i, err := strconv.ParseInt(s[2:], 8, 64); err == nil {
			return i
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		// Only treat as float if it looks like a float (has dot or e/E)
		if strings.ContainsAny(s, ".eE") {
			return f
		}
	}
	return s
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '0':
				b.WriteByte(0)
			case 'x':
				if i+2 < len(s) {
					if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
						b.WriteByte(byte(v))
						i += 2
					} else {
						b.WriteByte('x')
					}
				}
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// JSON to YAML
// ---------------------------------------------------------------------------

func toYAML(data any, indent int) string {
	prefix := strings.Repeat("  ", indent)
	switch v := data.(type) {
	case map[string]any:
		if len(v) == 0 {
			return prefix + "{}\n"
		}
		var sb strings.Builder
		for key, val := range v {
			quotedKey := yamlQuoteKey(key)
			switch child := val.(type) {
			case map[string]any, []any:
				sb.WriteString(prefix + quotedKey + ":\n")
				sb.WriteString(toYAML(child, indent+1))
			default:
				sb.WriteString(prefix + quotedKey + ": " + scalarToYAML(val) + "\n")
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

// yamlQuoteKey quotes a map key if it contains special characters.
func yamlQuoteKey(key string) string {
	if key == "" {
		return `""`
	}
	needsQuote := false
	for _, ch := range key {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' && ch != '.' {
			needsQuote = true
			break
		}
	}
	// Also quote if it looks like a bool/null
	switch strings.ToLower(key) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		needsQuote = true
	}
	if needsQuote {
		return `"` + strings.ReplaceAll(key, `"`, `\"`) + `"`
	}
	return key
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
		if math.IsInf(val, 1) {
			return ".inf"
		}
		if math.IsInf(val, -1) {
			return "-.inf"
		}
		if math.IsNaN(val) {
			return ".nan"
		}
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	case json.Number:
		return val.String()
	case string:
		return yamlQuoteString(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// yamlQuoteString quotes a string value if needed for valid YAML output.
func yamlQuoteString(s string) string {
	if s == "" {
		return `""`
	}
	// Quote if it could be misinterpreted
	switch strings.ToLower(s) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return `"` + s + `"`
	}
	// Quote if it starts with special chars or contains problematic chars
	if strings.ContainsAny(s, ":{}\n\t[]&*!|>'\",#%@`") {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\t", `\t`)
		return `"` + escaped + `"`
	}
	// Quote if it looks like a number
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return `"` + s + `"`
	}
	return s
}
