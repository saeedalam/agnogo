package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/saeedalam/agnogo"
)

// SQL returns tools for querying any SQL database.
// Pass a *sql.DB connection. Read-only by default.
func SQL(db *sql.DB, readOnly bool) []agnogo.ToolDef {
	defs := []agnogo.ToolDef{{
		Name: "sql_query", Desc: "Execute a SQL SELECT query and return results as JSON",
		Params: agnogo.Params{
			"query": {Type: "string", Desc: "SQL SELECT query", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			q := strings.TrimSpace(args["query"])
			if readOnly {
				upper := strings.ToUpper(q)
				if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
					return "Only SELECT queries are allowed (read-only mode).", nil
				}
			}
			rows, err := db.QueryContext(ctx, q)
			if err != nil {
				return fmt.Sprintf("Query error: %s", err), nil
			}
			defer rows.Close()

			cols, _ := rows.Columns()
			var results []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				rows.Scan(ptrs...)
				row := map[string]any{}
				for i, col := range cols {
					row[col] = vals[i]
				}
				results = append(results, row)
				if len(results) >= 50 {
					break // limit results
				}
			}
			data, _ := json.Marshal(results)
			return string(data), nil
		},
	}}

	if !readOnly {
		defs = append(defs, agnogo.ToolDef{
			Name: "sql_execute", Desc: "Execute a SQL INSERT/UPDATE/DELETE statement",
			Params: agnogo.Params{
				"query": {Type: "string", Desc: "SQL statement", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				result, err := db.ExecContext(ctx, args["query"])
				if err != nil {
					return fmt.Sprintf("Execute error: %s", err), nil
				}
				affected, _ := result.RowsAffected()
				return fmt.Sprintf("OK: %d rows affected", affected), nil
			},
		})
	}

	return defs
}
