package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/saeedalam/agnogo"
)

// GoogleSearch returns a tool for searching Google via Custom Search API.
// Requires a Google API key and Custom Search Engine ID.
// Get them at: https://programmablesearchengine.google.com/
func GoogleSearch(apiKey, searchEngineID string) []agnogo.ToolDef {
	client := &http.Client{Timeout: 10 * time.Second}

	return []agnogo.ToolDef{{
		Name: "google_search", Desc: "Search Google and return top results",
		Params: agnogo.Params{
			"query": {Type: "string", Desc: "Search query", Required: true},
			"num":   {Type: "number", Desc: "Number of results (default 5, max 10)"},
		},
		Fn: func(ctx context.Context, args map[string]string) (string, error) {
			q := url.QueryEscape(args["query"])
			num := "5"
			if args["num"] != "" {
				num = args["num"]
			}
			u := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%s",
				apiKey, searchEngineID, q, num)

			req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("Search failed: %s", err), nil
			}
			defer resp.Body.Close()
			data, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				return fmt.Sprintf("Google API error %d: %s", resp.StatusCode, string(data)[:min(len(data), 200)]), nil
			}

			var result struct {
				Items []struct {
					Title   string `json:"title"`
					Link    string `json:"link"`
					Snippet string `json:"snippet"`
				} `json:"items"`
			}
			json.Unmarshal(data, &result)

			if len(result.Items) == 0 {
				return "No results found.", nil
			}

			var out string
			for i, item := range result.Items {
				out += fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, item.Title, item.Snippet, item.Link)
			}
			return out, nil
		},
	}}
}
