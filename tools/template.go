package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/saeedalam/agnogo"
)

const maxTemplateSize = 10 * 1024 // 10KB

// TemplateTool returns a tool for rendering Go text/template strings.
func TemplateTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "render_template", Desc: "Render a Go text/template with JSON data",
			Params: agnogo.Params{
				"template": {Type: "string", Desc: "Go text/template string", Required: true},
				"data":     {Type: "string", Desc: "JSON string with template data", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				tmplStr := args["template"]
				dataStr := args["data"]
				if tmplStr == "" {
					return "", fmt.Errorf("template is required")
				}
				if len(tmplStr) > maxTemplateSize {
					return "", fmt.Errorf("template exceeds %d byte limit", maxTemplateSize)
				}
				if dataStr == "" {
					return "", fmt.Errorf("data is required")
				}

				var data any
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					return "", fmt.Errorf("invalid JSON data: %w", err)
				}

				tmpl, err := template.New("t").Parse(tmplStr)
				if err != nil {
					return "", fmt.Errorf("invalid template: %w", err)
				}

				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err != nil {
					return "", fmt.Errorf("template execution failed: %w", err)
				}
				return buf.String(), nil
			},
		},
	}
}
