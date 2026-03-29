package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/saeedalam/agnogo"
)

// CSV returns tools for reading and querying CSV files.
func CSV() []agnogo.ToolDef {
	return []agnogo.ToolDef{{
		Name: "read_csv", Desc: "Read a CSV file and return as JSON array (first row = headers)",
		Params: agnogo.Params{
			"path":  {Type: "string", Desc: "Path to CSV file", Required: true},
			"limit": {Type: "number", Desc: "Max rows to return (default 50)"},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			f, err := os.Open(args["path"])
			if err != nil {
				return fmt.Sprintf("Cannot open file: %s", err), nil
			}
			defer f.Close()

			reader := csv.NewReader(f)
			records, err := reader.ReadAll()
			if err != nil {
				return fmt.Sprintf("CSV parse error: %s", err), nil
			}
			if len(records) < 2 {
				return "CSV is empty or has only headers.", nil
			}

			headers := records[0]
			limit := 50
			if l := args["limit"]; l != "" {
				fmt.Sscanf(l, "%d", &limit)
			}

			var rows []map[string]string
			for i, rec := range records[1:] {
				if i >= limit {
					break
				}
				row := map[string]string{}
				for j, h := range headers {
					if j < len(rec) {
						row[strings.TrimSpace(h)] = rec[j]
					}
				}
				rows = append(rows, row)
			}

			data, _ := json.Marshal(rows)
			return string(data), nil
		},
	}}
}
