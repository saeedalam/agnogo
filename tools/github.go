package tools

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

// GitHubConfig configures GitHub tools.
type GitHubConfig struct {
	// BaseURL for GitHub API. Default: "https://api.github.com".
	// Set to your GitHub Enterprise URL (e.g. "https://github.example.com/api/v3").
	BaseURL string
	// Timeout in seconds. Default: 15.
	Timeout int
	// DefaultPerPage is the default number of items per page. Default: 10.
	DefaultPerPage int
}

func (c *GitHubConfig) defaults() {
	if c.BaseURL == "" {
		c.BaseURL = "https://api.github.com"
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.Timeout <= 0 {
		c.Timeout = 15
	}
	if c.DefaultPerPage <= 0 {
		c.DefaultPerPage = 10
	}
}

// GitHub returns tools for interacting with GitHub.
// Uses GitHub REST API directly (no external SDK).
func GitHub(token string, cfgs ...GitHubConfig) []agnogo.ToolDef {
	var cfg GitHubConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	cfg.defaults()

	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}

	ghAPI := func(ctx context.Context, method, path string, body ...map[string]any) (string, http.Header, int, error) {
		var bodyReader io.Reader
		if len(body) > 0 {
			b, _ := json.Marshal(body[0])
			bodyReader = strings.NewReader(string(b))
		}

		fullURL := cfg.BaseURL + path
		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return "", nil, 0, fmt.Errorf("invalid request: %w", err)
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if bodyReader != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", nil, 0, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			// Parse GitHub error JSON
			var ghErr struct {
				Message string `json:"message"`
				DocURL  string `json:"documentation_url"`
			}
			if json.Unmarshal(data, &ghErr) == nil && ghErr.Message != "" {
				errMsg := fmt.Sprintf("GitHub API error %d: %s", resp.StatusCode, ghErr.Message)
				if ghErr.DocURL != "" {
					errMsg += " (see: " + ghErr.DocURL + ")"
				}
				return "", resp.Header, resp.StatusCode, fmt.Errorf("%s", errMsg)
			}
			return "", resp.Header, resp.StatusCode, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(data))
		}

		return string(data), resp.Header, resp.StatusCode, nil
	}

	// rateLimitWarning checks the rate limit header and prepends a warning if low.
	rateLimitWarning := func(headers http.Header) string {
		if remaining := headers.Get("X-RateLimit-Remaining"); remaining != "" {
			if n, err := strconv.Atoi(remaining); err == nil && n < 10 {
				return fmt.Sprintf("[WARNING: GitHub API rate limit low: %d remaining]\n", n)
			}
		}
		return ""
	}

	paginationParams := func() agnogo.Params {
		return agnogo.Params{
			"page":     {Type: "string", Desc: "Page number (default 1)"},
			"per_page": {Type: "string", Desc: fmt.Sprintf("Items per page (default %d, max 100)", cfg.DefaultPerPage)},
		}
	}

	buildPagination := func(args map[string]string) string {
		page := "1"
		if p := strings.TrimSpace(args["page"]); p != "" {
			page = p
		}
		perPage := strconv.Itoa(cfg.DefaultPerPage)
		if pp := strings.TrimSpace(args["per_page"]); pp != "" {
			if n, err := strconv.Atoi(pp); err == nil && n > 0 && n <= 100 {
				perPage = pp
			} else {
				perPage = strconv.Itoa(cfg.DefaultPerPage)
			}
		}
		return fmt.Sprintf("page=%s&per_page=%s", page, perPage)
	}

	return []agnogo.ToolDef{
		{
			Name: "github_search_repos",
			Desc: "Search GitHub repositories",
			Params: mergeParams(agnogo.Params{
				"query": {Type: "string", Desc: "Search query (e.g. 'language:go stars:>1000')", Required: true},
			}, paginationParams()),
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				query := strings.TrimSpace(args["query"])
				if query == "" {
					return "", fmt.Errorf("missing required parameter: query")
				}
				q := url.QueryEscape(query)
				pagination := buildPagination(args)
				result, headers, _, err := ghAPI(ctx, "GET", "/search/repositories?q="+q+"&"+pagination)
				if err != nil {
					return "", err
				}
				var resp struct {
					TotalCount int `json:"total_count"`
					Items      []struct {
						FullName    string `json:"full_name"`
						Description string `json:"description"`
						Stars       int    `json:"stargazers_count"`
						URL         string `json:"html_url"`
					} `json:"items"`
				}
				if err := json.Unmarshal([]byte(result), &resp); err != nil {
					return "", fmt.Errorf("error parsing response: %w", err)
				}
				var out strings.Builder
				out.WriteString(rateLimitWarning(headers))
				out.WriteString(fmt.Sprintf("Total: %d results\n", resp.TotalCount))
				for _, r := range resp.Items {
					out.WriteString(fmt.Sprintf("- %s (%d stars) %s\n  %s\n", r.FullName, r.Stars, r.Description, r.URL))
				}
				return out.String(), nil
			},
		},
		{
			Name: "github_get_repo",
			Desc: "Get details about a GitHub repository",
			Params: agnogo.Params{
				"repo": {Type: "string", Desc: "Repository (owner/name)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				result, headers, _, err := ghAPI(ctx, "GET", "/repos/"+repo)
				if err != nil {
					return "", err
				}
				return rateLimitWarning(headers) + result, nil
			},
		},
		{
			Name: "github_list_issues",
			Desc: "List issues in a GitHub repository",
			Params: mergeParams(agnogo.Params{
				"repo":  {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"state": {Type: "string", Desc: "Issue state: open, closed, all (default: open)"},
			}, paginationParams()),
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				state := "open"
				if s := strings.TrimSpace(args["state"]); s != "" {
					state = s
				}
				pagination := buildPagination(args)
				result, headers, _, err := ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/issues?state=%s&%s", repo, state, pagination))
				if err != nil {
					return "", err
				}
				var issues []struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
					User   struct {
						Login string `json:"login"`
					} `json:"user"`
				}
				if err := json.Unmarshal([]byte(result), &issues); err != nil {
					return "", fmt.Errorf("error parsing response: %w", err)
				}
				var out strings.Builder
				out.WriteString(rateLimitWarning(headers))
				for _, i := range issues {
					out.WriteString(fmt.Sprintf("#%d [%s] %s (by %s)\n", i.Number, i.State, i.Title, i.User.Login))
				}
				if out.Len() == 0 {
					return "No issues found.", nil
				}
				return out.String(), nil
			},
		},
		{
			Name: "github_create_issue",
			Desc: "Create a new issue in a GitHub repository",
			Params: agnogo.Params{
				"repo":  {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"title": {Type: "string", Desc: "Issue title", Required: true},
				"body":  {Type: "string", Desc: "Issue description"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				title := strings.TrimSpace(args["title"])
				if title == "" {
					return "", fmt.Errorf("missing required parameter: title")
				}
				result, _, _, err := ghAPI(ctx, "POST", "/repos/"+repo+"/issues", map[string]any{
					"title": title,
					"body":  args["body"],
				})
				if err != nil {
					return "", err
				}
				return result, nil
			},
		},
		{
			Name: "github_get_file",
			Desc: "Get file contents from a GitHub repository",
			Params: agnogo.Params{
				"repo": {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"path": {Type: "string", Desc: "File path", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				path := strings.TrimSpace(args["path"])
				if path == "" {
					return "", fmt.Errorf("missing required parameter: path")
				}
				result, headers, _, err := ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/contents/%s", repo, path))
				if err != nil {
					return "", err
				}
				return rateLimitWarning(headers) + result, nil
			},
		},
		{
			Name: "github_list_pulls",
			Desc: "List pull requests in a GitHub repository",
			Params: mergeParams(agnogo.Params{
				"repo":  {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"state": {Type: "string", Desc: "PR state: open, closed, all (default: open)"},
			}, paginationParams()),
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				state := "open"
				if s := strings.TrimSpace(args["state"]); s != "" {
					state = s
				}
				pagination := buildPagination(args)
				result, headers, _, err := ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/pulls?state=%s&%s", repo, state, pagination))
				if err != nil {
					return "", err
				}
				var prs []struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
					User   struct {
						Login string `json:"login"`
					} `json:"user"`
					Draft bool `json:"draft"`
				}
				if err := json.Unmarshal([]byte(result), &prs); err != nil {
					return "", fmt.Errorf("error parsing response: %w", err)
				}
				var out strings.Builder
				out.WriteString(rateLimitWarning(headers))
				for _, pr := range prs {
					draft := ""
					if pr.Draft {
						draft = " [DRAFT]"
					}
					out.WriteString(fmt.Sprintf("#%d [%s]%s %s (by %s)\n", pr.Number, pr.State, draft, pr.Title, pr.User.Login))
				}
				if out.Len() == 0 {
					return "No pull requests found.", nil
				}
				return out.String(), nil
			},
		},
		{
			Name: "github_get_pull",
			Desc: "Get details of a specific pull request",
			Params: agnogo.Params{
				"repo":   {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"number": {Type: "string", Desc: "Pull request number", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				if err := ctx.Err(); err != nil {
					return "", fmt.Errorf("context cancelled: %w", err)
				}
				repo := strings.TrimSpace(args["repo"])
				if repo == "" {
					return "", fmt.Errorf("missing required parameter: repo")
				}
				number := strings.TrimSpace(args["number"])
				if number == "" {
					return "", fmt.Errorf("missing required parameter: number")
				}
				result, headers, _, err := ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/pulls/%s", repo, number))
				if err != nil {
					return "", err
				}
				return rateLimitWarning(headers) + result, nil
			},
		},
	}
}

// mergeParams merges multiple Params maps into one.
func mergeParams(maps ...agnogo.Params) agnogo.Params {
	result := agnogo.Params{}
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
