package contrib

import (
	"bytes"
	"context"
	"encoding/base64"
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

// Jira returns tools for interacting with Jira.
// Clone of agno's JiraTools.
func Jira(serverURL, username, token string) []agnogo.ToolDef {
	if serverURL == "" {
		serverURL = os.Getenv("JIRA_SERVER_URL")
	}
	if username == "" {
		username = os.Getenv("JIRA_USERNAME")
	}
	if token == "" {
		token = os.Getenv("JIRA_TOKEN")
	}
	serverURL = strings.TrimRight(serverURL, "/")
	baseURL := serverURL + "/rest/api/2"
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+token))
	client := &http.Client{Timeout: 15 * time.Second}

	jiraAPI := func(ctx context.Context, method, path string, body ...map[string]any) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		var bodyReader io.Reader
		if len(body) > 0 {
			data, _ := json.Marshal(body[0])
			bodyReader = bytes.NewReader(data)
		}
		fullURL := baseURL + path
		req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Jira API request failed: %w", err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Jira API error %d: %s", resp.StatusCode, string(data))
		}
		return string(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "get_issue",
			Desc: "Get details of a Jira issue by key",
			Params: agnogo.Params{
				"issue_key": {Type: "string", Desc: "Issue key (e.g. PROJ-123)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				issueKey := strings.TrimSpace(args["issue_key"])
				if issueKey == "" {
					return "", fmt.Errorf("missing required parameter: issue_key")
				}
				result, err := jiraAPI(ctx, "GET", "/issue/"+url.PathEscape(issueKey))
				if err != nil {
					return "", err
				}
				var raw map[string]any
				if err := json.Unmarshal([]byte(result), &raw); err != nil {
					return result, nil
				}
				fields, _ := raw["fields"].(map[string]any)
				if fields == nil {
					return result, nil
				}
				projectName := ""
				if proj, ok := fields["project"].(map[string]any); ok {
					projectName, _ = proj["key"].(string)
				}
				issueType := ""
				if it, ok := fields["issuetype"].(map[string]any); ok {
					issueType, _ = it["name"].(string)
				}
				reporter := ""
				if rep, ok := fields["reporter"].(map[string]any); ok {
					reporter, _ = rep["displayName"].(string)
				}
				summary, _ := fields["summary"].(string)
				description, _ := fields["description"].(string)

				out := map[string]string{
					"key":         fmt.Sprintf("%v", raw["key"]),
					"project":     projectName,
					"issuetype":   issueType,
					"reporter":    reporter,
					"summary":     summary,
					"description": description,
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				return string(b), nil
			},
		},
		{
			Name: "create_issue",
			Desc: "Create a new Jira issue",
			Params: agnogo.Params{
				"project_key": {Type: "string", Desc: "Project key (e.g. PROJ)", Required: true},
				"summary":     {Type: "string", Desc: "Issue summary", Required: true},
				"description": {Type: "string", Desc: "Issue description"},
				"issuetype":   {Type: "string", Desc: "Issue type (default Task)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				projectKey := strings.TrimSpace(args["project_key"])
				if projectKey == "" {
					return "", fmt.Errorf("missing required parameter: project_key")
				}
				summary := strings.TrimSpace(args["summary"])
				if summary == "" {
					return "", fmt.Errorf("missing required parameter: summary")
				}
				issueType := "Task"
				if it := strings.TrimSpace(args["issuetype"]); it != "" {
					issueType = it
				}
				return jiraAPI(ctx, "POST", "/issue", map[string]any{
					"fields": map[string]any{
						"project":     map[string]any{"key": projectKey},
						"summary":     summary,
						"description": args["description"],
						"issuetype":   map[string]any{"name": issueType},
					},
				})
			},
		},
		{
			Name: "search_issues",
			Desc: "Search Jira issues using JQL",
			Params: agnogo.Params{
				"jql":         {Type: "string", Desc: "JQL query string", Required: true},
				"max_results": {Type: "string", Desc: "Maximum results to return (default 50)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				jql := strings.TrimSpace(args["jql"])
				if jql == "" {
					return "", fmt.Errorf("missing required parameter: jql")
				}
				maxResults := "50"
				if mr := strings.TrimSpace(args["max_results"]); mr != "" {
					maxResults = mr
				}
				path := fmt.Sprintf("/search?jql=%s&maxResults=%s", url.QueryEscape(jql), url.QueryEscape(maxResults))
				result, err := jiraAPI(ctx, "GET", path)
				if err != nil {
					return "", err
				}
				var raw struct {
					Issues []struct {
						Key    string `json:"key"`
						Fields struct {
							Summary string `json:"summary"`
							Status  struct {
								Name string `json:"name"`
							} `json:"status"`
							Assignee *struct {
								DisplayName string `json:"displayName"`
							} `json:"assignee"`
						} `json:"fields"`
					} `json:"issues"`
				}
				if err := json.Unmarshal([]byte(result), &raw); err != nil {
					return result, nil
				}
				type issueEntry struct {
					Key      string `json:"key"`
					Summary  string `json:"summary"`
					Status   string `json:"status"`
					Assignee string `json:"assignee"`
				}
				entries := make([]issueEntry, 0, len(raw.Issues))
				for _, iss := range raw.Issues {
					assignee := "Unassigned"
					if iss.Fields.Assignee != nil {
						assignee = iss.Fields.Assignee.DisplayName
					}
					entries = append(entries, issueEntry{
						Key:      iss.Key,
						Summary:  iss.Fields.Summary,
						Status:   iss.Fields.Status.Name,
						Assignee: assignee,
					})
				}
				b, _ := json.MarshalIndent(entries, "", "  ")
				return string(b), nil
			},
		},
		{
			Name: "add_comment",
			Desc: "Add a comment to a Jira issue",
			Params: agnogo.Params{
				"issue_key": {Type: "string", Desc: "Issue key (e.g. PROJ-123)", Required: true},
				"comment":   {Type: "string", Desc: "Comment text", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				issueKey := strings.TrimSpace(args["issue_key"])
				if issueKey == "" {
					return "", fmt.Errorf("missing required parameter: issue_key")
				}
				comment := args["comment"]
				if comment == "" {
					return "", fmt.Errorf("missing required parameter: comment")
				}
				return jiraAPI(ctx, "POST", "/issue/"+url.PathEscape(issueKey)+"/comment", map[string]any{
					"body": comment,
				})
			},
		},
	}
}
