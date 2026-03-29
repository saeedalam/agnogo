package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// GitHub returns tools for interacting with GitHub.
// Uses GitHub REST API directly (no external SDK).
func GitHub(token string) []agnogo.ToolDef {
	client := &http.Client{Timeout: 15 * time.Second}

	ghAPI := func(ctx context.Context, method, path string, body ...map[string]any) (string, error) {
		var bodyReader io.Reader
		if len(body) > 0 {
			b, _ := json.Marshal(body[0])
			bodyReader = strings.NewReader(string(b))
		}
		req, _ := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, bodyReader)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		if bodyReader != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		if resp.StatusCode >= 400 {
			return fmt.Sprintf("GitHub API error %d: %s", resp.StatusCode, string(data)), nil
		}
		return string(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "github_search_repos", Desc: "Search GitHub repositories",
			Params: agnogo.Params{
				"query": {Type: "string", Desc: "Search query (e.g. 'language:go stars:>1000')", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				q := url.QueryEscape(args["query"])
				result, err := ghAPI(ctx, "GET", "/search/repositories?q="+q+"&per_page=5")
				if err != nil {
					return "", err
				}
				var resp struct {
					Items []struct {
						FullName    string `json:"full_name"`
						Description string `json:"description"`
						Stars       int    `json:"stargazers_count"`
						URL         string `json:"html_url"`
					} `json:"items"`
				}
				json.Unmarshal([]byte(result), &resp)
				var out string
				for _, r := range resp.Items {
					out += fmt.Sprintf("- %s (%d★) %s\n  %s\n", r.FullName, r.Stars, r.Description, r.URL)
				}
				return out, nil
			},
		},
		{
			Name: "github_get_repo", Desc: "Get details about a GitHub repository",
			Params: agnogo.Params{
				"repo": {Type: "string", Desc: "Repository (owner/name)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return ghAPI(ctx, "GET", "/repos/"+args["repo"])
			},
		},
		{
			Name: "github_list_issues", Desc: "List issues in a GitHub repository",
			Params: agnogo.Params{
				"repo":  {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"state": {Type: "string", Desc: "Issue state (open/closed/all)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				state := "open"
				if args["state"] != "" {
					state = args["state"]
				}
				result, err := ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/issues?state=%s&per_page=10", args["repo"], state))
				if err != nil {
					return "", err
				}
				var issues []struct {
					Number int    `json:"number"`
					Title  string `json:"title"`
					State  string `json:"state"`
					User   struct{ Login string } `json:"user"`
				}
				json.Unmarshal([]byte(result), &issues)
				var out string
				for _, i := range issues {
					out += fmt.Sprintf("#%d [%s] %s (by %s)\n", i.Number, i.State, i.Title, i.User.Login)
				}
				return out, nil
			},
		},
		{
			Name: "github_create_issue", Desc: "Create a new issue in a GitHub repository",
			Params: agnogo.Params{
				"repo":  {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"title": {Type: "string", Desc: "Issue title", Required: true},
				"body":  {Type: "string", Desc: "Issue description"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return ghAPI(ctx, "POST", "/repos/"+args["repo"]+"/issues", map[string]any{
					"title": args["title"],
					"body":  args["body"],
				})
			},
		},
		{
			Name: "github_get_file", Desc: "Get file contents from a GitHub repository",
			Params: agnogo.Params{
				"repo": {Type: "string", Desc: "Repository (owner/name)", Required: true},
				"path": {Type: "string", Desc: "File path", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return ghAPI(ctx, "GET", fmt.Sprintf("/repos/%s/contents/%s", args["repo"], args["path"]))
			},
		},
	}
}
