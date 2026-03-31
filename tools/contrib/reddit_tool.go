package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/saeedalam/agnogo"
)

// Reddit returns tools for fetching data from Reddit.
// Clone of agno's RedditTools.
func Reddit(clientID, clientSecret, userAgent string) []agnogo.ToolDef {
	if clientID == "" {
		clientID = os.Getenv("REDDIT_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("REDDIT_CLIENT_SECRET")
	}
	if userAgent == "" {
		userAgent = os.Getenv("REDDIT_USER_AGENT")
	}
	if userAgent == "" {
		userAgent = "agnogo/1.0"
	}

	client := &http.Client{Timeout: 15 * time.Second}

	var (
		tokenMu     sync.Mutex
		cachedToken string
		tokenExpiry time.Time
	)

	getToken := func(ctx context.Context) (string, error) {
		tokenMu.Lock()
		defer tokenMu.Unlock()

		if cachedToken != "" && time.Now().Before(tokenExpiry) {
			return cachedToken, nil
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://www.reddit.com/api/v1/access_token",
			strings.NewReader("grant_type=client_credentials"))
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.SetBasicAuth(clientID, clientSecret)
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Reddit OAuth request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Reddit OAuth error %d: %s", resp.StatusCode, string(data))
		}

		var tokenResp struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			Error       string `json:"error"`
		}
		if err := json.Unmarshal(data, &tokenResp); err != nil {
			return "", fmt.Errorf("error parsing Reddit OAuth response: %w", err)
		}
		if tokenResp.Error != "" {
			return "", fmt.Errorf("Reddit OAuth error: %s", tokenResp.Error)
		}

		cachedToken = tokenResp.AccessToken
		// Cache for slightly less than the actual expiry to be safe
		if tokenResp.ExpiresIn > 60 {
			tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
		} else {
			tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		}
		return cachedToken, nil
	}

	redditAPI := func(ctx context.Context, path string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		token, err := getToken(ctx)
		if err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, "GET", "https://oauth.reddit.com"+path, nil)
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Reddit API request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Reddit API error %d: %s", resp.StatusCode, string(data))
		}
		return string(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "get_top_posts",
			Desc: "Get top posts from a subreddit",
			Params: agnogo.Params{
				"subreddit":   {Type: "string", Desc: "Subreddit name (without r/)", Required: true},
				"time_filter": {Type: "string", Desc: "Time filter: hour, day, week, month, year, all (default week)"},
				"limit":       {Type: "string", Desc: "Number of posts to return (default 10)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				subreddit := strings.TrimSpace(args["subreddit"])
				if subreddit == "" {
					return "", fmt.Errorf("missing required parameter: subreddit")
				}
				subreddit = strings.TrimPrefix(subreddit, "r/")
				timeFilter := "week"
				if tf := strings.TrimSpace(args["time_filter"]); tf != "" {
					timeFilter = tf
				}
				limit := "10"
				if l := strings.TrimSpace(args["limit"]); l != "" {
					limit = l
				}
				path := fmt.Sprintf("/r/%s/top?t=%s&limit=%s", subreddit, timeFilter, limit)
				return redditAPI(ctx, path)
			},
		},
		{
			Name: "get_subreddit_info",
			Desc: "Get information about a subreddit",
			Params: agnogo.Params{
				"subreddit": {Type: "string", Desc: "Subreddit name (without r/)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				subreddit := strings.TrimSpace(args["subreddit"])
				if subreddit == "" {
					return "", fmt.Errorf("missing required parameter: subreddit")
				}
				subreddit = strings.TrimPrefix(subreddit, "r/")
				return redditAPI(ctx, fmt.Sprintf("/r/%s/about", subreddit))
			},
		},
		{
			Name: "get_user_info",
			Desc: "Get information about a Reddit user",
			Params: agnogo.Params{
				"username": {Type: "string", Desc: "Reddit username", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				username := strings.TrimSpace(args["username"])
				if username == "" {
					return "", fmt.Errorf("missing required parameter: username")
				}
				username = strings.TrimPrefix(username, "u/")
				return redditAPI(ctx, fmt.Sprintf("/user/%s/about", username))
			},
		},
	}
}
