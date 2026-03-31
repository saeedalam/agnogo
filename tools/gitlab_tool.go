package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// GitLab returns tools for interacting with the GitLab API.
// Clone of agno's GitlabTools.
func GitLab(token string, baseURL ...string) []agnogo.ToolDef {
	if token == "" {
		token = os.Getenv("GITLAB_ACCESS_TOKEN")
	}
	apiBase := ""
	if len(baseURL) > 0 && baseURL[0] != "" {
		apiBase = baseURL[0]
	}
	if apiBase == "" {
		apiBase = os.Getenv("GITLAB_BASE_URL")
	}
	if apiBase == "" {
		apiBase = "https://gitlab.com"
	}
	apiBase = strings.TrimRight(apiBase, "/") + "/api/v4"

	client := &http.Client{Timeout: 15 * time.Second}

	glAPI := func(ctx context.Context, method, path string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		fullURL := apiBase + path
		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("PRIVATE-TOKEN", token)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("GitLab API request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, string(data))
		}
		return string(data), nil
	}

	paginationMeta := func(items int, args map[string]string) string {
		page := "1"
		if p := strings.TrimSpace(args["page"]); p != "" {
			page = p
		}
		perPage := "20"
		if pp := strings.TrimSpace(args["per_page"]); pp != "" {
			perPage = pp
		}
		meta := map[string]any{
			"current_page":   page,
			"per_page":       perPage,
			"returned_items": items,
		}
		b, _ := json.Marshal(meta)
		return string(b)
	}

	wrapWithMeta := func(rawJSON string, args map[string]string) (string, error) {
		var items []json.RawMessage
		if err := json.Unmarshal([]byte(rawJSON), &items); err != nil {
			return rawJSON, nil
		}
		result := map[string]any{
			"data": items,
			"meta": json.RawMessage(paginationMeta(len(items), args)),
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return string(b), nil
	}

	buildQuery := func(params map[string]string) string {
		q := url.Values{}
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		if encoded := q.Encode(); encoded != "" {
			return "?" + encoded
		}
		return ""
	}

	return []agnogo.ToolDef{
		{
			Name: "list_projects",
			Desc: "List GitLab projects with optional search",
			Params: agnogo.Params{
				"search":   {Type: "string", Desc: "Search query for project name"},
				"owned":    {Type: "string", Desc: "Filter for owned projects (default false)"},
				"page":     {Type: "string", Desc: "Page number (default 1)"},
				"per_page": {Type: "string", Desc: "Items per page (default 20)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				owned := "false"
				if o := strings.TrimSpace(args["owned"]); o != "" {
					owned = o
				}
				page := "1"
				if p := strings.TrimSpace(args["page"]); p != "" {
					page = p
				}
				perPage := "20"
				if pp := strings.TrimSpace(args["per_page"]); pp != "" {
					perPage = pp
				}
				qp := map[string]string{
					"owned":    owned,
					"page":     page,
					"per_page": perPage,
				}
				if s := strings.TrimSpace(args["search"]); s != "" {
					qp["search"] = s
				}
				result, err := glAPI(ctx, "GET", "/projects"+buildQuery(qp))
				if err != nil {
					return "", err
				}
				return wrapWithMeta(result, args)
			},
		},
		{
			Name: "get_project",
			Desc: "Get details of a GitLab project",
			Params: agnogo.Params{
				"project_id": {Type: "string", Desc: "Project ID or URL-encoded path", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				projectID := strings.TrimSpace(args["project_id"])
				if projectID == "" {
					return "", fmt.Errorf("missing required parameter: project_id")
				}
				return glAPI(ctx, "GET", "/projects/"+url.PathEscape(projectID))
			},
		},
		{
			Name: "list_merge_requests",
			Desc: "List merge requests for a GitLab project",
			Params: agnogo.Params{
				"project_id": {Type: "string", Desc: "Project ID or URL-encoded path", Required: true},
				"state":      {Type: "string", Desc: "MR state: opened, closed, merged, all (default opened)"},
				"page":       {Type: "string", Desc: "Page number (default 1)"},
				"per_page":   {Type: "string", Desc: "Items per page (default 20)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				projectID := strings.TrimSpace(args["project_id"])
				if projectID == "" {
					return "", fmt.Errorf("missing required parameter: project_id")
				}
				state := "opened"
				if s := strings.TrimSpace(args["state"]); s != "" {
					state = s
				}
				page := "1"
				if p := strings.TrimSpace(args["page"]); p != "" {
					page = p
				}
				perPage := "20"
				if pp := strings.TrimSpace(args["per_page"]); pp != "" {
					perPage = pp
				}
				qp := map[string]string{
					"state":    state,
					"page":     page,
					"per_page": perPage,
				}
				path := fmt.Sprintf("/projects/%s/merge_requests%s", url.PathEscape(projectID), buildQuery(qp))
				result, err := glAPI(ctx, "GET", path)
				if err != nil {
					return "", err
				}
				return wrapWithMeta(result, args)
			},
		},
		{
			Name: "get_merge_request",
			Desc: "Get details of a specific merge request",
			Params: agnogo.Params{
				"project_id":        {Type: "string", Desc: "Project ID or URL-encoded path", Required: true},
				"merge_request_iid": {Type: "string", Desc: "Merge request IID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				projectID := strings.TrimSpace(args["project_id"])
				if projectID == "" {
					return "", fmt.Errorf("missing required parameter: project_id")
				}
				mrIID := strings.TrimSpace(args["merge_request_iid"])
				if mrIID == "" {
					return "", fmt.Errorf("missing required parameter: merge_request_iid")
				}
				path := fmt.Sprintf("/projects/%s/merge_requests/%s", url.PathEscape(projectID), url.PathEscape(mrIID))
				return glAPI(ctx, "GET", path)
			},
		},
		{
			Name: "list_issues",
			Desc: "List issues for a GitLab project",
			Params: agnogo.Params{
				"project_id": {Type: "string", Desc: "Project ID or URL-encoded path", Required: true},
				"state":      {Type: "string", Desc: "Issue state: opened, closed, all (default opened)"},
				"labels":     {Type: "string", Desc: "Comma-separated list of labels"},
				"search":     {Type: "string", Desc: "Search in title and description"},
				"page":       {Type: "string", Desc: "Page number (default 1)"},
				"per_page":   {Type: "string", Desc: "Items per page (default 20)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				projectID := strings.TrimSpace(args["project_id"])
				if projectID == "" {
					return "", fmt.Errorf("missing required parameter: project_id")
				}
				state := "opened"
				if s := strings.TrimSpace(args["state"]); s != "" {
					state = s
				}
				page := "1"
				if p := strings.TrimSpace(args["page"]); p != "" {
					page = p
				}
				perPage := "20"
				if pp := strings.TrimSpace(args["per_page"]); pp != "" {
					perPage = pp
				}
				qp := map[string]string{
					"state":    state,
					"page":     page,
					"per_page": perPage,
				}
				if labels := strings.TrimSpace(args["labels"]); labels != "" {
					qp["labels"] = labels
				}
				if search := strings.TrimSpace(args["search"]); search != "" {
					qp["search"] = search
				}
				path := fmt.Sprintf("/projects/%s/issues%s", url.PathEscape(projectID), buildQuery(qp))
				result, err := glAPI(ctx, "GET", path)
				if err != nil {
					return "", err
				}
				return wrapWithMeta(result, args)
			},
		},
	}
}
