package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// HTTPConfig configures the HTTP request tool.
type HTTPConfig struct {
	// DefaultTimeout in seconds for HTTP requests. Default: 30.
	DefaultTimeout int
	// MaxResponseSize in bytes. Default: 65536.
	MaxResponseSize int64
	// UserAgent string. Default: "agnogo/1.0".
	UserAgent string
}

func (c *HTTPConfig) defaults() {
	if c.DefaultTimeout <= 0 {
		c.DefaultTimeout = 30
	}
	if c.MaxResponseSize <= 0 {
		c.MaxResponseSize = 65536
	}
	if c.UserAgent == "" {
		c.UserAgent = "agnogo/1.0"
	}
}

// HTTP returns a tool for making HTTP requests.
// Supports GET, POST, PUT, DELETE, PATCH methods.
// Configurable headers, timeout, and response size limit.
func HTTP(cfgs ...HTTPConfig) []agnogo.ToolDef {
	var cfg HTTPConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	return []agnogo.ToolDef{{
		Name: "http_request",
		Desc: "Make an HTTP request. Returns JSON with status, headers, and body.",
		Params: agnogo.Params{
			"url":      {Type: "string", Desc: "Full URL to request", Required: true},
			"method":   {Type: "string", Desc: "HTTP method: GET, POST, PUT, DELETE, PATCH", Required: true},
			"body":     {Type: "string", Desc: "Request body (for POST/PUT/PATCH)"},
			"headers":  {Type: "string", Desc: "Request headers as JSON object (e.g. {\"Authorization\": \"Bearer token\"})"},
			"timeout":  {Type: "string", Desc: fmt.Sprintf("Request timeout in seconds (default %d)", cfg.DefaultTimeout)},
			"max_size": {Type: "string", Desc: fmt.Sprintf("Max response body size in bytes (default %d)", cfg.MaxResponseSize)},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", fmt.Errorf("context cancelled: %w", err)
			}

			urlStr := strings.TrimSpace(args["url"])
			if urlStr == "" {
				return "", fmt.Errorf("missing required parameter: url")
			}

			method := strings.ToUpper(strings.TrimSpace(args["method"]))
			validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}
			if !validMethods[method] {
				return "", fmt.Errorf("unsupported HTTP method: %q (supported: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)", method)
			}

			// Parse timeout
			timeout := time.Duration(cfg.DefaultTimeout) * time.Second
			if t := strings.TrimSpace(args["timeout"]); t != "" {
				secs, err := strconv.Atoi(t)
				if err != nil || secs <= 0 {
					return "", fmt.Errorf("invalid timeout: %q (must be positive integer seconds)", t)
				}
				if secs > 300 {
					return "", fmt.Errorf("timeout %d exceeds maximum of 300 seconds", secs)
				}
				timeout = time.Duration(secs) * time.Second
			}

			// Parse max size
			maxSize := cfg.MaxResponseSize
			if ms := strings.TrimSpace(args["max_size"]); ms != "" {
				n, err := strconv.ParseInt(ms, 10, 64)
				if err != nil || n <= 0 {
					return "", fmt.Errorf("invalid max_size: %q (must be positive integer)", ms)
				}
				maxSize = n
			}

			// Build request
			var bodyReader io.Reader
			if args["body"] != "" {
				bodyReader = strings.NewReader(args["body"])
			}

			reqCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			req, err := http.NewRequestWithContext(reqCtx, method, urlStr, bodyReader)
			if err != nil {
				return "", fmt.Errorf("invalid request: %w", err)
			}

			req.Header.Set("User-Agent", cfg.UserAgent)
			if args["body"] != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			// Parse custom headers
			if h := strings.TrimSpace(args["headers"]); h != "" {
				var hdrs map[string]string
				if err := json.Unmarshal([]byte(h), &hdrs); err != nil {
					return "", fmt.Errorf("invalid headers JSON: %w", err)
				}
				for k, v := range hdrs {
					req.Header.Set(k, v)
				}
			}

			client := &http.Client{Timeout: timeout}
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
			if err != nil {
				return "", fmt.Errorf("error reading response body: %w", err)
			}

			truncated := false
			if int64(len(body)) > maxSize {
				body = body[:maxSize]
				truncated = true
			}

			// Collect response headers
			respHeaders := map[string]string{}
			for k := range resp.Header {
				respHeaders[k] = resp.Header.Get(k)
			}

			result := map[string]any{
				"status_code": resp.StatusCode,
				"status":      resp.Status,
				"headers":     respHeaders,
				"body":        string(body),
			}
			if truncated {
				result["truncated"] = true
			}

			out, _ := json.Marshal(result)
			return string(out), nil
		},
	}}
}
