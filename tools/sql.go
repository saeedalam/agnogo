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
//
// SECURITY NOTE: For read-only mode, use a database user with only SELECT privileges.
// The query validation here is defense-in-depth, not a security boundary.
func SQL(db *sql.DB, readOnly bool) []agnogo.ToolDef {
	defs := []agnogo.ToolDef{{
		Name: "sql_query", Desc: "Execute a SQL SELECT query and return results as JSON",
		Params: agnogo.Params{
			"query": {Type: "string", Desc: "SQL SELECT query", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			q := strings.TrimSpace(args["query"])
			if readOnly {
				if err := validateReadOnly(q); err != nil {
					return err.Error(), nil
				}
			}
			rows, err := db.QueryContext(ctx, q)
			if err != nil {
				return fmt.Sprintf("Query error: %s", err), nil
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return fmt.Sprintf("Columns error: %s", err), nil
			}
			var results []map[string]any
			for rows.Next() {
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return fmt.Sprintf("Scan error: %s", err), nil
				}
				row := map[string]any{}
				for i, col := range cols {
					row[col] = vals[i]
				}
				results = append(results, row)
				if len(results) >= 50 {
					break
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

// validateReadOnly checks that a query doesn't contain mutation keywords.
// This is defense-in-depth — always use a read-only DB user for real security.
func validateReadOnly(q string) error {
	upper := strings.ToUpper(q)

	// Must start with SELECT or WITH
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT queries are allowed (read-only mode)")
	}

	// Reject semicolons (prevents multi-statement injection)
	if strings.Contains(q, ";") {
		return fmt.Errorf("semicolons not allowed in read-only mode")
	}

	// Reject mutation keywords anywhere in the query
	forbidden := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "CREATE", "GRANT", "REVOKE", "INTO"}
	words := strings.Fields(upper)
	for _, w := range words {
		for _, f := range forbidden {
			if w == f {
				return fmt.Errorf("keyword '%s' not allowed in read-only mode", f)
			}
		}
	}

	return nil
}
