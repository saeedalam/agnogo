package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// SQLConfig configures SQL tools.
type SQLConfig struct {
	// MaxRows limits the number of rows returned. Default: 100.
	MaxRows int
}

func (c *SQLConfig) defaults() {
	if c.MaxRows <= 0 {
		c.MaxRows = 100
	}
}

// SQL returns tools for querying any SQL database.
// Pass a *sql.DB connection. Read-only by default.
//
// SECURITY NOTE: For read-only mode, use a database user with only SELECT privileges.
// The query validation here is defense-in-depth, not a security boundary.
func SQL(db *sql.DB, readOnly bool, cfgs ...SQLConfig) []agnogo.ToolDef {
	var cfg SQLConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	defs := []agnogo.ToolDef{
		{
			Name: "sql_query",
			Desc: "Execute a SQL SELECT query and return results as JSON",
			Params: agnogo.Params{
				"query":  {Type: "string", Desc: "SQL SELECT query", Required: true},
				"limit":  {Type: "string", Desc: fmt.Sprintf("Max rows to return (default %d)", cfg.MaxRows)},
				"offset": {Type: "string", Desc: "Number of rows to skip (default 0)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}

				q := strings.TrimSpace(args["query"])
				if q == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}

				if readOnly {
					if err := validateReadOnly(q); err != nil {
						return "", err
					}
				}

				maxRows := cfg.MaxRows
				if l := strings.TrimSpace(args["limit"]); l != "" {
					if n, err := strconv.Atoi(l); err == nil && n > 0 {
						maxRows = n
						if maxRows > cfg.MaxRows {
							maxRows = cfg.MaxRows
						}
					}
				}

				offset := 0
				if o := strings.TrimSpace(args["offset"]); o != "" {
					if n, err := strconv.Atoi(o); err == nil && n >= 0 {
						offset = n
					}
				}

				// Apply LIMIT and OFFSET if not already in query
				upperQ := strings.ToUpper(q)
				if !strings.Contains(upperQ, "LIMIT") {
					q += fmt.Sprintf(" LIMIT %d", maxRows)
				}
				if offset > 0 && !strings.Contains(upperQ, "OFFSET") {
					q += fmt.Sprintf(" OFFSET %d", offset)
				}

				rows, err := db.QueryContext(ctx, q)
				if err != nil {
					return "", fmt.Errorf("query error: %w", err)
				}
				defer rows.Close()

				cols, err := rows.Columns()
				if err != nil {
					return "", fmt.Errorf("columns error: %w", err)
				}

				var results []map[string]any
				count := 0
				for rows.Next() {
					if err := ctx.Err(); err != nil {
						return "", fmt.Errorf("context cancelled: %w", err)
					}
					vals := make([]any, len(cols))
					ptrs := make([]any, len(cols))
					for i := range vals {
						ptrs[i] = &vals[i]
					}
					if err := rows.Scan(ptrs...); err != nil {
						return "", fmt.Errorf("scan error: %w", err)
					}
					row := map[string]any{}
					for i, col := range cols {
						// Convert []byte to string for JSON compatibility
						if b, ok := vals[i].([]byte); ok {
							row[col] = string(b)
						} else {
							row[col] = vals[i]
						}
					}
					results = append(results, row)
					count++
					if count >= maxRows {
						break
					}
				}
				if err := rows.Err(); err != nil {
					return "", fmt.Errorf("rows iteration error: %w", err)
				}

				out := map[string]any{
					"rows":       results,
					"row_count":  count,
					"columns":    cols,
				}
				data, _ := json.Marshal(out)
				return string(data), nil
			},
		},
		{
			Name: "sql_tables",
			Desc: "List tables and their columns from the database",
			Params: agnogo.Params{
				"schema": {Type: "string", Desc: "Schema name (default: public or main depending on database)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}

				// Try information_schema first (PostgreSQL, MySQL, etc.)
				q := `SELECT table_name, column_name, data_type
				      FROM information_schema.columns
				      WHERE table_schema = $1
				      ORDER BY table_name, ordinal_position`

				schema := "public"
				if s := strings.TrimSpace(args["schema"]); s != "" {
					schema = s
				}

				rows, err := db.QueryContext(ctx, q, schema)
				if err != nil {
					// Fallback: try SQLite style
					rows, err = db.QueryContext(ctx, "SELECT name, '', '' FROM sqlite_master WHERE type='table' ORDER BY name")
					if err != nil {
						return "", fmt.Errorf("could not list tables: %w", err)
					}
				}
				defer rows.Close()

				type colInfo struct {
					Column   string `json:"column"`
					DataType string `json:"data_type"`
				}
				tables := map[string][]colInfo{}
				for rows.Next() {
					var table, col, dtype string
					if err := rows.Scan(&table, &col, &dtype); err != nil {
						return "", fmt.Errorf("scan error: %w", err)
					}
					if col != "" {
						tables[table] = append(tables[table], colInfo{Column: col, DataType: dtype})
					} else {
						if _, ok := tables[table]; !ok {
							tables[table] = []colInfo{}
						}
					}
				}
				if err := rows.Err(); err != nil {
					return "", fmt.Errorf("rows error: %w", err)
				}

				data, _ := json.Marshal(tables)
				return string(data), nil
			},
		},
	}

	if !readOnly {
		defs = append(defs, agnogo.ToolDef{
			Name: "sql_execute",
			Desc: "Execute a SQL INSERT/UPDATE/DELETE statement",
			Params: agnogo.Params{
				"query": {Type: "string", Desc: "SQL statement", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				q := strings.TrimSpace(args["query"])
				if q == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				result, err := db.ExecContext(ctx, q)
				if err != nil {
					return "", fmt.Errorf("execute error: %w", err)
				}
				affected, _ := result.RowsAffected()
				return fmt.Sprintf("OK: %d rows affected", affected), nil
			},
		})
	}

	return defs
}

// validateReadOnly checks that a query doesn't contain mutation keywords.
// This is defense-in-depth -- always use a read-only DB user for real security.
func validateReadOnly(q string) error {
	upper := strings.ToUpper(strings.TrimSpace(q))

	// Must start with SELECT or WITH
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT queries are allowed in read-only mode")
	}

	// Reject semicolons (prevents multi-statement injection)
	if strings.Contains(q, ";") {
		return fmt.Errorf("semicolons not allowed in read-only mode")
	}

	// Reject mutation keywords
	forbidden := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE", "GRANT", "REVOKE", "INTO"}
	words := strings.Fields(upper)
	for _, w := range words {
		for _, f := range forbidden {
			if w == f {
				return fmt.Errorf("keyword %q not allowed in read-only mode", f)
			}
		}
	}

	return nil
}
