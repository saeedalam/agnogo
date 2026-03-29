package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saeedalam/agnogo"
)

// Slack returns tools for interacting with Slack.
// Uses Slack Web API directly (no external SDK).
func Slack(token string) []agnogo.ToolDef {
	client := &http.Client{Timeout: 10 * time.Second}

	slackAPI := func(ctx context.Context, method string, body map[string]any) (string, error) {
		bodyJSON, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/"+method, bytes.NewReader(bodyJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return string(data), nil
	}

	return []agnogo.ToolDef{
		{
			Name: "slack_send_message", Desc: "Send a message to a Slack channel",
			Params: agnogo.Params{
				"channel": {Type: "string", Desc: "Channel ID or name (e.g. #general or C1234)", Required: true},
				"text":    {Type: "string", Desc: "Message text", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				return slackAPI(ctx, "chat.postMessage", map[string]any{
					"channel": args["channel"],
					"text":    args["text"],
				})
			},
		},
		{
			Name: "slack_list_channels", Desc: "List Slack channels",
			Params: agnogo.Params{},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				result, err := slackAPI(ctx, "conversations.list", map[string]any{"limit": 20})
				if err != nil {
					return "", err
				}
				var resp struct {
					Channels []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"channels"`
				}
				json.Unmarshal([]byte(result), &resp)
				var out string
				for _, ch := range resp.Channels {
					out += fmt.Sprintf("- #%s (%s)\n", ch.Name, ch.ID)
				}
				return out, nil
			},
		},
		{
			Name: "slack_read_messages", Desc: "Read recent messages from a Slack channel",
			Params: agnogo.Params{
				"channel": {Type: "string", Desc: "Channel ID", Required: true},
				"limit":   {Type: "number", Desc: "Max messages (default 10)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				limit := "10"
				if args["limit"] != "" {
					limit = args["limit"]
				}
				return slackAPI(ctx, "conversations.history", map[string]any{
					"channel": args["channel"],
					"limit":   limit,
				})
			},
		},
	}
}
