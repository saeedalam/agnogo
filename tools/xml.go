package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// XML returns tools for converting between XML and JSON, plus XPath-like queries.
// Handles attributes (_attr), repeated elements as arrays, CDATA, namespaces,
// and provides xml_query for path-based lookups.
func XML() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "xml_to_json",
			Desc: "Convert XML to JSON. Attributes become _attrName keys, repeated elements become arrays, text content uses _text when mixed with children.",
			Params: agnogo.Params{
				"xml":              {Type: "string", Desc: "XML string to convert", Required: true},
				"strip_namespaces": {Type: "string", Desc: "Strip namespace prefixes (true/false, default true)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				xmlStr := args["xml"]
				if xmlStr == "" {
					return "", fmt.Errorf("xml is required")
				}
				strip := args["strip_namespaces"] != "false"
				result, err := xmlToMap(xmlStr, strip)
				if err != nil {
					return "", fmt.Errorf("XML parse error: %w", err)
				}
				out, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return "", fmt.Errorf("JSON marshal error: %w", err)
				}
				return string(out), nil
			},
		},
		{
			Name: "json_to_xml",
			Desc: "Convert JSON to XML. Keys starting with _ become attributes, arrays repeat the parent tag.",
			Params: agnogo.Params{
				"json":     {Type: "string", Desc: "JSON string to convert", Required: true},
				"root_tag": {Type: "string", Desc: "Root XML tag name (default: root)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				jsonStr := args["json"]
				rootTag := args["root_tag"]
				if jsonStr == "" {
					return "", fmt.Errorf("json is required")
				}
				if rootTag == "" {
					rootTag = "root"
				}
				var data any
				if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
					return "", fmt.Errorf("invalid JSON: %w", err)
				}
				var sb strings.Builder
				sb.WriteString(xml.Header)
				mapToXML(&sb, rootTag, data, 0)
				return sb.String(), nil
			},
		},
		{
			Name: "xml_query",
			Desc: "Query XML using a simple path expression. Supports /root/child/item[0] syntax with optional attribute filters like /root/item[@id=123].",
			Params: agnogo.Params{
				"xml":              {Type: "string", Desc: "XML string to query", Required: true},
				"path":             {Type: "string", Desc: "Path expression (e.g. /root/items/item[0])", Required: true},
				"strip_namespaces": {Type: "string", Desc: "Strip namespace prefixes (true/false, default true)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				xmlStr := args["xml"]
				pathExpr := args["path"]
				if xmlStr == "" {
					return "", fmt.Errorf("xml is required")
				}
				if pathExpr == "" {
					return "", fmt.Errorf("path is required")
				}
				strip := args["strip_namespaces"] != "false"
				parsed, err := xmlToMap(xmlStr, strip)
				if err != nil {
					return "", fmt.Errorf("XML parse error: %w", err)
				}
				result := xmlQueryPath(parsed, pathExpr)
				if result == nil {
					return "null", nil
				}
				out, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return "", fmt.Errorf("JSON marshal error: %w", err)
				}
				return string(out), nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// XML to map conversion
// ---------------------------------------------------------------------------

func xmlToMap(input string, stripNS bool) (map[string]any, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	result, err := xmlDecodeElement(dec, nil, stripNS)
	if err != nil {
		return nil, err
	}
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{"value": result}, nil
}

func xmlDecodeElement(dec *xml.Decoder, start *xml.StartElement, stripNS bool) (any, error) {
	children := map[string]any{}
	// Track insertion order for repeated element detection
	childOrder := map[string]int{}
	var charData strings.Builder
	hasCDATA := false

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := xmlDecodeElement(dec, &t, stripNS)
			if err != nil {
				return nil, err
			}
			name := xmlElementName(t, stripNS)
			childOrder[name]++
			if existing, ok := children[name]; ok {
				switch v := existing.(type) {
				case []any:
					children[name] = append(v, child)
				default:
					children[name] = []any{v, child}
				}
			} else {
				children[name] = child
			}

		case xml.CharData:
			charData.Write(t)

		case xml.Directive:
			// CDATA is delivered as CharData by encoding/xml, but
			// we note if we see any directive
			_ = t

		case xml.Comment:
			// skip comments

		case xml.EndElement:
			text := strings.TrimSpace(charData.String())

			// No children: return text or nil
			if len(children) == 0 && !hasCDATA {
				if start != nil && len(start.Attr) > 0 {
					m := map[string]any{}
					for _, attr := range start.Attr {
						attrName := xmlAttrName(attr, stripNS)
						m["_"+attrName] = attr.Value
					}
					if text != "" {
						m["_text"] = text
					}
					return m, nil
				}
				if text == "" {
					return nil, nil
				}
				return text, nil
			}

			// Has children
			if text != "" {
				children["_text"] = text
			}
			if start != nil {
				for _, attr := range start.Attr {
					attrName := xmlAttrName(attr, stripNS)
					children["_"+attrName] = attr.Value
				}
			}
			return children, nil
		}
	}

	if len(children) == 0 {
		text := strings.TrimSpace(charData.String())
		if text != "" {
			return text, nil
		}
		return nil, nil
	}
	_ = hasCDATA
	return children, nil
}

func xmlElementName(el xml.StartElement, stripNS bool) string {
	if stripNS || el.Name.Space == "" {
		return el.Name.Local
	}
	return el.Name.Space + ":" + el.Name.Local
}

func xmlAttrName(attr xml.Attr, stripNS bool) string {
	if stripNS || attr.Name.Space == "" {
		return attr.Name.Local
	}
	return attr.Name.Space + ":" + attr.Name.Local
}

// ---------------------------------------------------------------------------
// XML query (XPath-like)
// ---------------------------------------------------------------------------

func xmlQueryPath(data any, path string) any {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return data
	}
	segments := splitXMLPath(path)
	current := data
	for _, seg := range segments {
		current = xmlNavigateSegment(current, seg)
		if current == nil {
			return nil
		}
	}
	return current
}

type xmlPathSegment struct {
	name      string
	index     int  // -1 means no index
	hasIndex  bool
	attrName  string // for [@attr=value] filters
	attrValue string
}

func splitXMLPath(path string) []xmlPathSegment {
	parts := strings.Split(path, "/")
	var segments []xmlPathSegment
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		seg := xmlPathSegment{index: -1}

		// Check for [@attr=value]
		if idx := strings.Index(p, "[@"); idx >= 0 {
			seg.name = p[:idx]
			filter := p[idx+2:]
			filter = strings.TrimSuffix(filter, "]")
			if eqIdx := strings.Index(filter, "="); eqIdx >= 0 {
				seg.attrName = filter[:eqIdx]
				seg.attrValue = strings.Trim(filter[eqIdx+1:], `"'`)
			}
			segments = append(segments, seg)
			continue
		}

		// Check for [index]
		if idx := strings.Index(p, "["); idx >= 0 {
			seg.name = p[:idx]
			idxStr := strings.TrimSuffix(p[idx+1:], "]")
			if n, err := strconv.Atoi(idxStr); err == nil {
				seg.index = n
				seg.hasIndex = true
			}
		} else {
			seg.name = p
		}
		segments = append(segments, seg)
	}
	return segments
}

func xmlNavigateSegment(data any, seg xmlPathSegment) any {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}

	val, exists := m[seg.name]
	if !exists {
		return nil
	}

	// Attribute filter
	if seg.attrName != "" {
		arr, isArr := val.([]any)
		if !isArr {
			// Single element: check it
			if em, ok := val.(map[string]any); ok {
				if av, ok := em["_"+seg.attrName]; ok && fmt.Sprintf("%v", av) == seg.attrValue {
					return val
				}
			}
			return nil
		}
		for _, item := range arr {
			if em, ok := item.(map[string]any); ok {
				if av, ok := em["_"+seg.attrName]; ok && fmt.Sprintf("%v", av) == seg.attrValue {
					return item
				}
			}
		}
		return nil
	}

	// Index access
	if seg.hasIndex {
		switch v := val.(type) {
		case []any:
			if seg.index >= 0 && seg.index < len(v) {
				return v[seg.index]
			}
			return nil
		default:
			if seg.index == 0 {
				return val
			}
			return nil
		}
	}

	return val
}

// ---------------------------------------------------------------------------
// JSON to XML
// ---------------------------------------------------------------------------

func mapToXML(sb *strings.Builder, tag string, data any, indent int) {
	prefix := strings.Repeat("  ", indent)
	switch v := data.(type) {
	case map[string]any:
		// Collect attributes and children
		var attrs []string
		var textContent string
		childMap := map[string]any{}
		for key, val := range v {
			if strings.HasPrefix(key, "_") && key != "_text" {
				attrName := key[1:]
				attrs = append(attrs, fmt.Sprintf(` %s="%s"`, attrName, xmlEscapeAttr(fmt.Sprintf("%v", val))))
			} else if key == "_text" {
				textContent = fmt.Sprintf("%v", val)
			} else {
				childMap[key] = val
			}
		}

		attrStr := strings.Join(attrs, "")
		if len(childMap) == 0 && textContent == "" {
			sb.WriteString(prefix + "<" + tag + attrStr + "/>\n")
			return
		}
		if len(childMap) == 0 {
			sb.WriteString(prefix + "<" + tag + attrStr + ">" + xmlEscapeText(textContent) + "</" + tag + ">\n")
			return
		}

		sb.WriteString(prefix + "<" + tag + attrStr + ">\n")
		if textContent != "" {
			sb.WriteString(prefix + "  " + xmlEscapeText(textContent) + "\n")
		}
		for key, val := range childMap {
			mapToXML(sb, key, val, indent+1)
		}
		sb.WriteString(prefix + "</" + tag + ">\n")

	case []any:
		for _, item := range v {
			mapToXML(sb, tag, item, indent)
		}

	case nil:
		sb.WriteString(prefix + "<" + tag + "/>\n")

	default:
		sb.WriteString(fmt.Sprintf("%s<%s>%s</%s>\n", prefix, tag, xmlEscapeText(fmt.Sprintf("%v", v)), tag))
	}
}

func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func xmlEscapeAttr(s string) string {
	s = xmlEscapeText(s)
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
