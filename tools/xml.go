package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/saeedalam/agnogo"
)

// XML returns tools for converting between XML and JSON.
func XML() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "xml_to_json", Desc: "Convert XML to JSON",
			Params: agnogo.Params{
				"xml": {Type: "string", Desc: "XML string to convert", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				xmlStr := args["xml"]
				if xmlStr == "" {
					return "", fmt.Errorf("xml is required")
				}
				result, err := xmlToMap(xmlStr)
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
			Name: "json_to_xml", Desc: "Convert JSON to XML",
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
	}
}

func xmlToMap(input string) (map[string]any, error) {
	dec := xml.NewDecoder(strings.NewReader(input))
	result, err := xmlDecodeElement(dec, nil)
	if err != nil {
		return nil, err
	}
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{"value": result}, nil
}

func xmlDecodeElement(dec *xml.Decoder, start *xml.StartElement) (any, error) {
	children := map[string]any{}
	var charData strings.Builder

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
			child, err := xmlDecodeElement(dec, &t)
			if err != nil {
				return nil, err
			}
			name := t.Name.Local
			if existing, ok := children[name]; ok {
				// Convert to array
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
		case xml.EndElement:
			text := strings.TrimSpace(charData.String())
			if len(children) == 0 {
				if text == "" {
					return nil, nil
				}
				return text, nil
			}
			if text != "" {
				children["#text"] = text
			}
			if start != nil {
				for _, attr := range start.Attr {
					children["-"+attr.Name.Local] = attr.Value
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
	return children, nil
}

func mapToXML(sb *strings.Builder, tag string, data any, indent int) {
	prefix := strings.Repeat("  ", indent)
	switch v := data.(type) {
	case map[string]any:
		sb.WriteString(prefix + "<" + tag + ">\n")
		for key, val := range v {
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
		sb.WriteString(fmt.Sprintf("%s<%s>%v</%s>\n", prefix, tag, v, tag))
	}
}
