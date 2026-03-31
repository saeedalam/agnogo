package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/saeedalam/agnogo"
)

// Linear returns tools for interacting with the Linear project management API.
// Clone of agno's LinearTools.
func Linear(apiKey string) []agnogo.ToolDef {
	if apiKey == "" {
		apiKey = os.Getenv("LINEAR_API_KEY")
	}

	const endpoint = "https://api.linear.app/graphql"
	client := &http.Client{Timeout: 15 * time.Second}

	gqlAPI := func(ctx context.Context, query string, variables map[string]any) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("context cancelled: %w", err)
		}
		body := map[string]any{"query": query}
		if variables != nil {
			body["variables"] = variables
		}
		data, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("invalid request: %w", err)
		}
		req.Header.Set("Authorization", apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("Linear API request failed: %w", err)
		}
		defer resp.Body.Close()
		respData, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))

		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("Linear API error %d: %s", resp.StatusCode, string(respData))
		}

		var gqlResp struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if json.Unmarshal(respData, &gqlResp) == nil && len(gqlResp.Errors) > 0 {
			msgs := make([]string, len(gqlResp.Errors))
			for i, e := range gqlResp.Errors {
				msgs[i] = e.Message
			}
			return "", fmt.Errorf("Linear GraphQL error: %s", strings.Join(msgs, "; "))
		}
		return string(respData), nil
	}

	return []agnogo.ToolDef{
		{
			Name:   "get_user_details",
			Desc:   "Get the authenticated Linear user's details",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return gqlAPI(ctx, `{ viewer { id name email } }`, nil)
			},
		},
		{
			Name:   "get_teams",
			Desc:   "List all teams in the Linear workspace",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return gqlAPI(ctx, `{ teams { nodes { id name } } }`, nil)
			},
		},
		{
			Name: "get_issue",
			Desc: "Get a Linear issue by ID",
			Params: agnogo.Params{
				"issue_id": {Type: "string", Desc: "Linear issue ID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				issueID := strings.TrimSpace(args["issue_id"])
				if issueID == "" {
					return "", fmt.Errorf("missing required parameter: issue_id")
				}
				return gqlAPI(ctx, `query($id: String!) { issue(id: $id) { id title description } }`, map[string]any{
					"id": issueID,
				})
			},
		},
		{
			Name: "create_issue",
			Desc: "Create a new issue in Linear",
			Params: agnogo.Params{
				"title":       {Type: "string", Desc: "Issue title", Required: true},
				"description": {Type: "string", Desc: "Issue description"},
				"team_id":     {Type: "string", Desc: "Team ID to create the issue in", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				title := strings.TrimSpace(args["title"])
				if title == "" {
					return "", fmt.Errorf("missing required parameter: title")
				}
				teamID := strings.TrimSpace(args["team_id"])
				if teamID == "" {
					return "", fmt.Errorf("missing required parameter: team_id")
				}
				mutation := `mutation($title: String!, $description: String, $teamId: String!) {
					createIssue(input: {title: $title, description: $description, teamId: $teamId}) {
						success
						issue { id title url }
					}
				}`
				return gqlAPI(ctx, mutation, map[string]any{
					"title":       title,
					"description": args["description"],
					"teamId":      teamID,
				})
			},
		},
		{
			Name: "update_issue",
			Desc: "Update a Linear issue's title",
			Params: agnogo.Params{
				"issue_id": {Type: "string", Desc: "Issue ID to update", Required: true},
				"title":    {Type: "string", Desc: "New title for the issue", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				issueID := strings.TrimSpace(args["issue_id"])
				if issueID == "" {
					return "", fmt.Errorf("missing required parameter: issue_id")
				}
				title := strings.TrimSpace(args["title"])
				if title == "" {
					return "", fmt.Errorf("missing required parameter: title")
				}
				mutation := `mutation($id: String!, $title: String!) {
					updateIssue(id: $id, input: {title: $title}) {
						success
						issue { id title state { id name } }
					}
				}`
				return gqlAPI(ctx, mutation, map[string]any{
					"id":    issueID,
					"title": title,
				})
			},
		},
		{
			Name: "get_assigned_issues",
			Desc: "Get issues assigned to a specific Linear user",
			Params: agnogo.Params{
				"user_id": {Type: "string", Desc: "Linear user ID", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				userID := strings.TrimSpace(args["user_id"])
				if userID == "" {
					return "", fmt.Errorf("missing required parameter: user_id")
				}
				return gqlAPI(ctx, `query($id: String!) { user(id: $id) { assignedIssues { nodes { id title } } } }`, map[string]any{
					"id": userID,
				})
			},
		},
		{
			Name:   "get_high_priority_issues",
			Desc:   "Get high priority issues (priority 1 or 2) from Linear",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return gqlAPI(ctx, `{ issues(filter: {priority: {lte: 2}}) { nodes { id title priority } } }`, nil)
			},
		},
	}
}
