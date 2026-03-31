package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// DuckDuckGoSearch returns enhanced DuckDuckGo search tools.
// Clone of agno's DuckDuckGoTools. Returns web_search and search_news tools.
// This is an enhanced version that complements the basic DuckDuckGo() in web.go.
func DuckDuckGoSearch() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	type ddgResponse struct {
		Abstract       string `json:"Abstract"`
		AbstractText   string `json:"AbstractText"`
		AbstractSource string `json:"AbstractSource"`
		AbstractURL    string `json:"AbstractURL"`
		Answer         string `json:"Answer"`
		AnswerType     string `json:"AnswerType"`
		Heading        string `json:"Heading"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Icon     struct {
				URL string `json:"URL"`
			} `json:"Icon"`
			Result string `json:"Result"`
		} `json:"RelatedTopics"`
		Results []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Result   string `json:"Result"`
		} `json:"Results"`
	}

	fetchDDG := func(ctx context.Context, query string, extra string) (*ddgResponse, error) {
		q := url.QueryEscape(query)
		u := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1%s", q, extra)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "agnogo/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("DuckDuckGo API returned HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<17))
		if err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		var result ddgResponse
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}
		return &result, nil
	}

	return []agnogo.ToolDef{
		{
			Name: "ddg_web_search",
			Desc: "Search the web using DuckDuckGo and return structured results with titles and URLs",
			Params: agnogo.Params{
				"query":       {Type: "string", Desc: "Search query", Required: true},
				"max_results": {Type: "string", Desc: "Maximum number of results to return (default 5)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				maxResults := 5
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					if n, err := strconv.Atoi(mr); err == nil && n > 0 {
						maxResults = n
					}
				}

				result, err := fetchDDG(ctx, query, "")
				if err != nil {
					return "", err
				}

				var results []map[string]string

				// Add abstract if available
				if result.AbstractText != "" {
					results = append(results, map[string]string{
						"title":   result.Heading,
						"snippet": result.AbstractText,
						"url":     result.AbstractURL,
						"source":  result.AbstractSource,
					})
				}

				// Add answer if available
				if result.Answer != "" {
					results = append(results, map[string]string{
						"title":   "Answer",
						"snippet": result.Answer,
						"url":     "",
						"source":  "DuckDuckGo",
					})
				}

				// Add direct results
				for _, r := range result.Results {
					if len(results) >= maxResults {
						break
					}
					results = append(results, map[string]string{
						"title":   r.Text,
						"snippet": r.Text,
						"url":     r.FirstURL,
						"source":  "DuckDuckGo",
					})
				}

				// Add related topics
				for _, t := range result.RelatedTopics {
					if len(results) >= maxResults {
						break
					}
					if t.Text == "" {
						continue
					}
					results = append(results, map[string]string{
						"title":   t.Text,
						"snippet": t.Text,
						"url":     t.FirstURL,
						"source":  "DuckDuckGo",
					})
				}

				if len(results) == 0 {
					return `{"results":[],"message":"No results found"}`, nil
				}

				out, _ := json.Marshal(map[string]interface{}{
					"results": results,
					"query":   query,
				})
				return string(out), nil
			},
		},
		{
			Name: "ddg_search_news",
			Desc: "Search for news articles using DuckDuckGo",
			Params: agnogo.Params{
				"query":       {Type: "string", Desc: "News search query", Required: true},
				"max_results": {Type: "string", Desc: "Maximum number of results to return (default 5)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				maxResults := 5
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					if n, err := strconv.Atoi(mr); err == nil && n > 0 {
						maxResults = n
					}
				}

				result, err := fetchDDG(ctx, query+" news", "&t=news")
				if err != nil {
					return "", err
				}

				var results []map[string]string

				if result.AbstractText != "" {
					results = append(results, map[string]string{
						"title":   result.Heading,
						"snippet": result.AbstractText,
						"url":     result.AbstractURL,
					})
				}

				for _, r := range result.Results {
					if len(results) >= maxResults {
						break
					}
					results = append(results, map[string]string{
						"title":   r.Text,
						"snippet": r.Text,
						"url":     r.FirstURL,
					})
				}

				for _, t := range result.RelatedTopics {
					if len(results) >= maxResults {
						break
					}
					if t.Text == "" {
						continue
					}
					results = append(results, map[string]string{
						"title":   t.Text,
						"snippet": t.Text,
						"url":     t.FirstURL,
					})
				}

				if len(results) == 0 {
					return `{"results":[],"message":"No news results found"}`, nil
				}

				out, _ := json.Marshal(map[string]interface{}{
					"results": results,
					"query":   query,
				})
				return string(out), nil
			},
		},
	}
}
