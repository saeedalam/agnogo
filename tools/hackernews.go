package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/saeedalam/agnogo"
)

// HackerNews returns tools for interacting with the Hacker News API.
// Clone of agno's HackerNewsTools.
func HackerNews() []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	fetchJSON := func(ctx context.Context, url string, dest interface{}) error {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "agnogo/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		return json.Unmarshal(data, dest)
	}

	return []agnogo.ToolDef{
		{
			Name: "get_top_stories",
			Desc: "Get top stories from Hacker News with title, URL, score, and author",
			Params: agnogo.Params{
				"num_stories": {Type: "string", Desc: "Number of stories to return (default 10)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				numStories := 10
				if ns := strings.TrimSpace(args["num_stories"]); ns != "" {
					if n, err := strconv.Atoi(ns); err == nil && n > 0 {
						numStories = n
					}
				}
				if numStories > 30 {
					numStories = 30
				}

				var ids []int
				if err := fetchJSON(ctx, "https://hacker-news.firebaseio.com/v0/topstories.json", &ids); err != nil {
					return "", fmt.Errorf("failed to fetch top stories: %w", err)
				}
				if len(ids) > numStories {
					ids = ids[:numStories]
				}

				type story struct {
					ID    int    `json:"id"`
					Title string `json:"title"`
					URL   string `json:"url"`
					Score int    `json:"score"`
					By    string `json:"by"`
					Time  int64  `json:"time"`
					Type  string `json:"type"`
				}

				stories := make([]story, len(ids))
				errs := make([]error, len(ids))
				var wg sync.WaitGroup
				for i, id := range ids {
					wg.Add(1)
					go func(idx, storyID int) {
						defer wg.Done()
						u := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", storyID)
						errs[idx] = fetchJSON(ctx, u, &stories[idx])
					}(i, id)
				}
				wg.Wait()

				var results []map[string]interface{}
				for i, s := range stories {
					if errs[i] != nil || s.Title == "" {
						continue
					}
					results = append(results, map[string]interface{}{
						"id":    s.ID,
						"title": s.Title,
						"url":   s.URL,
						"score": s.Score,
						"by":    s.By,
						"time":  s.Time,
					})
				}

				out, _ := json.Marshal(results)
				return string(out), nil
			},
		},
		{
			Name: "get_user_details",
			Desc: "Get details about a Hacker News user including karma and submissions",
			Params: agnogo.Params{
				"username": {Type: "string", Desc: "HN username", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				username := strings.TrimSpace(args["username"])
				if username == "" {
					return "", fmt.Errorf("missing required parameter: username")
				}

				var user struct {
					ID        string `json:"id"`
					Karma     int    `json:"karma"`
					About     string `json:"about"`
					Submitted []int  `json:"submitted"`
				}
				u := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/user/%s.json", username)
				if err := fetchJSON(ctx, u, &user); err != nil {
					return "", fmt.Errorf("failed to fetch user: %w", err)
				}
				if user.ID == "" {
					return "User not found.", nil
				}

				result := map[string]interface{}{
					"id":                    user.ID,
					"karma":                 user.Karma,
					"about":                 user.About,
					"total_items_submitted": len(user.Submitted),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}
