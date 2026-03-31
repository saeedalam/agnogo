package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// Giphy returns tools for searching GIFs via the Giphy API.
// Clone of agno's GiphyTools. Returns: search_gifs.
// If apiKey is empty, falls back to GIPHY_API_KEY env var.
func Giphy(apiKey string) []agnogo.ToolDef {
	if apiKey == "" {
		apiKey = os.Getenv("GIPHY_API_KEY")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	return []agnogo.ToolDef{
		{
			Name: "search_gifs",
			Desc: "Search for GIFs using the Giphy API. Returns GIF URLs and metadata.",
			Params: agnogo.Params{
				"query": {Type: "string", Desc: "Search query for GIFs", Required: true},
				"limit": {Type: "string", Desc: "Maximum number of GIFs to return (default 5)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if apiKey == "" {
					return "", fmt.Errorf("Giphy API key not configured: pass apiKey or set GIPHY_API_KEY")
				}
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				limit := 5
				if l := strings.TrimSpace(args["limit"]); l != "" {
					if n, err := strconv.Atoi(l); err == nil && n > 0 {
						limit = n
					}
				}

				u := fmt.Sprintf("https://api.giphy.com/v1/gifs/search?api_key=%s&q=%s&limit=%d",
					url.QueryEscape(apiKey), url.QueryEscape(query), limit)

				req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
				if err != nil {
					return "", fmt.Errorf("failed to create request: %w", err)
				}

				resp, err := client.Do(req)
				if err != nil {
					return "", fmt.Errorf("Giphy request failed: %w", err)
				}
				defer resp.Body.Close()

				data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				if err != nil {
					return "", fmt.Errorf("read error: %w", err)
				}

				if resp.StatusCode != http.StatusOK {
					return "", fmt.Errorf("Giphy API returned HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 200)]))
				}

				var apiResp struct {
					Data []struct {
						ID     string `json:"id"`
						Title  string `json:"title"`
						Images struct {
							Original struct {
								URL string `json:"url"`
							} `json:"original"`
							FixedHeight struct {
								URL string `json:"url"`
							} `json:"fixed_height"`
						} `json:"images"`
						AltText string `json:"alt_text"`
					} `json:"data"`
				}
				if err := json.Unmarshal(data, &apiResp); err != nil {
					return "", fmt.Errorf("parse error: %w", err)
				}

				var results []map[string]string
				for _, g := range apiResp.Data {
					results = append(results, map[string]string{
						"id":               g.ID,
						"title":            g.Title,
						"url":              g.Images.Original.URL,
						"fixed_height_url": g.Images.FixedHeight.URL,
						"alt_text":         g.AltText,
					})
				}

				if len(results) == 0 {
					return `{"results":[],"message":"No GIFs found"}`, nil
				}

				out, _ := json.Marshal(results)
				return string(out), nil
			},
		},
	}
}
