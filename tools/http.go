package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// HTTP returns a tool for making HTTP requests.
func HTTP() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}
	return []agnogo.ToolDef{{
		Name: "http_request", Desc: "Make an HTTP request to any URL",
		Params: agnogo.Params{
			"url":    {Type: "string", Desc: "Full URL to request", Required: true},
			"method": {Type: "string", Desc: "HTTP method (GET, POST, etc.)", Required: true},
			"body":   {Type: "string", Desc: "Request body (for POST/PUT)"},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			method := strings.ToUpper(args["method"])
			var bodyReader io.Reader
			if args["body"] != "" {
				bodyReader = strings.NewReader(args["body"])
			}
			req, err := http.NewRequestWithContext(ctx, method, args["url"], bodyReader)
			if err != nil {
				return fmt.Sprintf("Invalid request: %s", err), nil
			}
			req.Header.Set("User-Agent", "agnogo/1.0")
			if args["body"] != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("Request failed: %s", err), nil
			}
			defer resp.Body.Close()
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return fmt.Sprintf("Status: %d\n%s", resp.StatusCode, string(data)), nil
		},
	}}
}
