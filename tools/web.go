package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// WebBrowser returns a tool for fetching and reading web pages.
func WebBrowser() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}
	return []agnogo.ToolDef{{
		Name: "read_url", Desc: "Fetch a URL and return its text content",
		Params: agnogo.Params{
			"url": {Type: "string", Desc: "URL to fetch (must start with https://)", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			u := args["url"]
			if !strings.HasPrefix(u, "http") {
				u = "https://" + u
			}
			req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
			req.Header.Set("User-Agent", "agnogo/1.0")
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("Failed to fetch: %s", err), nil
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			// Strip HTML tags for readability
			text := stripHTML(string(body))
			if len(text) > 3000 {
				text = text[:3000] + "\n... (truncated)"
			}
			return text, nil
		},
	}}
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// DuckDuckGo returns a web search tool using DuckDuckGo instant answers.
func DuckDuckGo() []agnogo.ToolDef {
	client := &http.Client{Timeout: 10 * time.Second}
	return []agnogo.ToolDef{{
		Name: "web_search", Desc: "Search the web using DuckDuckGo",
		Params: agnogo.Params{
			"query": {Type: "string", Desc: "Search query", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			q := url.QueryEscape(args["query"])
			u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", q)
			req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("Search failed: %s", err), nil
			}
			defer resp.Body.Close()
			data, _ := io.ReadAll(resp.Body)
			var result struct {
				Abstract     string `json:"Abstract"`
				AbstractText string `json:"AbstractText"`
				Answer       string `json:"Answer"`
				RelatedTopics []struct {
					Text string `json:"Text"`
				} `json:"RelatedTopics"`
			}
			json.Unmarshal(data, &result)

			var sb strings.Builder
			if result.Answer != "" {
				sb.WriteString("Answer: " + result.Answer + "\n")
			}
			if result.AbstractText != "" {
				sb.WriteString(result.AbstractText + "\n")
			}
			for i, t := range result.RelatedTopics {
				if i >= 5 { break }
				if t.Text != "" {
					sb.WriteString("- " + t.Text + "\n")
				}
			}
			if sb.Len() == 0 {
				return "No results found.", nil
			}
			return sb.String(), nil
		},
	}}
}

// Wikipedia returns a tool for searching Wikipedia.
func Wikipedia() []agnogo.ToolDef {
	client := &http.Client{Timeout: 10 * time.Second}
	return []agnogo.ToolDef{{
		Name: "wikipedia", Desc: "Search Wikipedia for information",
		Params: agnogo.Params{
			"query": {Type: "string", Desc: "Topic to search", Required: true},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			q := url.QueryEscape(args["query"])
			u := fmt.Sprintf("https://en.wikipedia.org/api/rest_v1/page/summary/%s", q)
			req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("Wikipedia search failed: %s", err), nil
			}
			defer resp.Body.Close()
			data, _ := io.ReadAll(resp.Body)
			var result struct {
				Title   string `json:"title"`
				Extract string `json:"extract"`
			}
			json.Unmarshal(data, &result)
			if result.Extract == "" {
				return "No Wikipedia article found.", nil
			}
			return fmt.Sprintf("# %s\n%s", result.Title, result.Extract), nil
		},
	}}
}
