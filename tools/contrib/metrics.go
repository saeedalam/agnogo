package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/saeedalam/agnogo"
)

// MetricsTool returns a tool for formatting metrics in Prometheus exposition format.
func MetricsTool() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "metrics_format", Desc: "Format a metric in Prometheus exposition format",
			Params: agnogo.Params{
				"name":   {Type: "string", Desc: "Metric name (e.g. http_requests_total)", Required: true},
				"type":   {Type: "string", Desc: "Metric type", Required: true, Enum: []string{"counter", "gauge", "histogram"}},
				"value":  {Type: "string", Desc: "Metric value", Required: true},
				"labels": {Type: "string", Desc: "JSON object of label key-value pairs"},
				"help":   {Type: "string", Desc: "Help text describing the metric"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				name := args["name"]
				metricType := args["type"]
				value := args["value"]
				if name == "" || metricType == "" || value == "" {
					return "", fmt.Errorf("name, type, and value are required")
				}

				// Validate metric name
				for i, c := range name {
					if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == ':' || (i > 0 && c >= '0' && c <= '9')) {
						return "", fmt.Errorf("invalid metric name character %q at position %d", string(c), i)
					}
				}

				switch metricType {
				case "counter", "gauge", "histogram":
				default:
					return "", fmt.Errorf("unsupported type: %s", metricType)
				}

				var sb strings.Builder

				// HELP line
				if args["help"] != "" {
					sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, args["help"]))
				}

				// TYPE line
				sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, metricType))

				// Labels
				labelStr := ""
				if args["labels"] != "" {
					var labels map[string]string
					if err := json.Unmarshal([]byte(args["labels"]), &labels); err != nil {
						return "", fmt.Errorf("invalid labels JSON: %w", err)
					}
					if len(labels) > 0 {
						// Sort keys for deterministic output
						keys := make([]string, 0, len(labels))
						for k := range labels {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						var parts []string
						for _, k := range keys {
							parts = append(parts, fmt.Sprintf(`%s="%s"`, k, labels[k]))
						}
						labelStr = "{" + strings.Join(parts, ",") + "}"
					}
				}

				// Metric line
				sb.WriteString(fmt.Sprintf("%s%s %s\n", name, labelStr, value))

				return sb.String(), nil
			},
		},
	}
}
